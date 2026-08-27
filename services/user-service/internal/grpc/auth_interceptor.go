package grpc

import (
	"context"
	"errors"
	"strings"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (domain.Identity, error)
}

type AuthInterceptor struct {
	tokenValidator TokenValidator
}

func NewAuthInterceptor(tknValidator TokenValidator) *AuthInterceptor {
	return &AuthInterceptor{
		tokenValidator: tknValidator,
	}
}

func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		switch info.FullMethod {
		case userservicev1.User_ValidateToken_FullMethodName,
			userservicev1.User_Login_FullMethodName,
			userservicev1.User_Register_FullMethodName,
			userservicev1.User_RefreshTokens_FullMethodName:
			return handler(ctx, req)
		case userservicev1.User_UpdateEmail_FullMethodName,
			userservicev1.User_UpdateName_FullMethodName,
			userservicev1.User_UpdatePassword_FullMethodName,
			userservicev1.User_Logout_FullMethodName,
			userservicev1.User_GetUser_FullMethodName:
			identity, err := a.authenticateUser(ctx)
			if err != nil {
				return nil, err
			}
			return handler(auth.WithIdentity(ctx, identity), req)

		default:
			return nil, status.Error(codes.PermissionDenied, "access policy is not configured for this method")

		}
	}
}

func (a *AuthInterceptor) authenticateUser(
	ctx context.Context,
) (domain.Identity, error) {
	if a.tokenValidator == nil {
		return domain.Identity{}, status.Error(
			codes.Internal,
			"token validator is not configured",
		)
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return domain.Identity{}, status.Error(
			codes.Unauthenticated,
			"authorization metadata is missing",
		)
	}

	values := md.Get("authorization")
	if len(values) != 1 {
		return domain.Identity{}, status.Error(
			codes.Unauthenticated,
			"authorization metadata is missing",
		)
	}

	parts := strings.Fields(values[0])
	if len(parts) != 2 ||
		!strings.EqualFold(parts[0], "Bearer") ||
		parts[1] == "" {
		return domain.Identity{}, status.Error(
			codes.Unauthenticated,
			"invalid authorization metadata",
		)
	}

	identity, err := a.tokenValidator.ValidateToken(ctx, parts[1])
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidToken) ||
			errors.Is(err, customerrors.ErrTokenBlacklisted) {
			return domain.Identity{}, status.Error(
				codes.Unauthenticated,
				"invalid or expired access token",
			)
		}

		return domain.Identity{}, status.Error(
			codes.Internal,
			"failed to validate access token",
		)
	}

	if identity.UserID <= 0 ||
		identity.TokenID == "" ||
		identity.ExpiresAt.IsZero() {
		return domain.Identity{}, status.Error(
			codes.Unauthenticated,
			"invalid access token identity",
		)
	}

	return identity, nil
}
