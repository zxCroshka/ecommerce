package grpc

import (
	"context"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/zxCroshka/ecommerce/services/cart-service/internal/auth"
	cartservicev1 "github.com/zxCroshka/ecommerce/shared/cartservice/gen/go"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authorizationMetadataKey = "authorization"
	serviceTokenMetadataKey  = "x-service-token"
)

type TokenValidator interface {
	ValidateToken(
		ctx context.Context,
		request *userservicev1.ValidateTokenRequest,
		options ...grpc.CallOption,
	) (*userservicev1.ValidateTokenResponse, error)
}

type AuthInterceptor struct {
	internalToken  string
	tokenValidator TokenValidator
	authTimeout    time.Duration
}

func NewAuthInterceptor(
	internalToken string,
	tokenValidator TokenValidator,
	authTimeout time.Duration,
) *AuthInterceptor {
	return &AuthInterceptor{
		internalToken:  strings.TrimSpace(internalToken),
		tokenValidator: tokenValidator,
		authTimeout:    authTimeout,
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
		case cartservicev1.Cart_GetCart_FullMethodName,
			cartservicev1.Cart_AddProduct_FullMethodName,
			cartservicev1.Cart_RemoveProduct_FullMethodName,
			cartservicev1.Cart_ChangeProductQuantity_FullMethodName:
			identity, err := a.authenticateUser(ctx)
			if err != nil {
				return nil, err
			}
			return handler(auth.WithUserIdentity(ctx, identity), req)

		case cartservicev1.Cart_CheckoutCart_FullMethodName,
			cartservicev1.Cart_ClearCartIfUnchanged_FullMethodName:
			if err := a.authenticateService(ctx); err != nil {
				return nil, err
			}
			return handler(auth.WithServiceIdentity(ctx), req)

		default:
			return nil, status.Error(codes.PermissionDenied, "access policy is not configured for this method")
		}
	}
}

func (a *AuthInterceptor) authenticateUser(ctx context.Context) (auth.UserIdentity, error) {
	token, err := bearerToken(ctx)
	if err != nil {
		return auth.UserIdentity{}, err
	}
	if a.tokenValidator == nil {
		return auth.UserIdentity{}, status.Error(codes.Internal, "authentication service is not configured")
	}

	validationCtx := ctx
	cancel := func() {}
	if a.authTimeout > 0 {
		validationCtx, cancel = context.WithTimeout(ctx, a.authTimeout)
	}
	defer cancel()

	response, err := a.tokenValidator.ValidateToken(
		validationCtx,
		&userservicev1.ValidateTokenRequest{Token: token},
	)
	if err != nil {
		switch status.Code(err) {
		case codes.Unavailable:
			return auth.UserIdentity{}, status.Error(codes.Unavailable, "authentication service is unavailable")
		case codes.DeadlineExceeded:
			return auth.UserIdentity{}, status.Error(codes.DeadlineExceeded, "authentication request timed out")
		case codes.Canceled:
			return auth.UserIdentity{}, status.Error(codes.Canceled, "authentication request was canceled")
		default:
			return auth.UserIdentity{}, status.Error(codes.Unauthenticated, "invalid access token")
		}
	}
	if response.GetUserId() <= 0 || strings.TrimSpace(response.GetRole()) == "" {
		return auth.UserIdentity{}, status.Error(codes.Unauthenticated, "invalid access token")
	}
	return auth.UserIdentity{UserID: response.GetUserId(), Role: response.GetRole()}, nil
}

func (a *AuthInterceptor) authenticateService(ctx context.Context) error {
	metadataValues, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing service credentials")
	}
	values := metadataValues.Get(serviceTokenMetadataKey)
	if len(values) != 1 || a.internalToken == "" ||
		subtle.ConstantTimeCompare([]byte(values[0]), []byte(a.internalToken)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid service credentials")
	}
	return nil
}

func bearerToken(ctx context.Context) (string, error) {
	metadataValues, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authorization metadata is missing")
	}
	values := metadataValues.Get(authorizationMetadataKey)
	if len(values) != 1 {
		return "", status.Error(codes.Unauthenticated, "authorization metadata is missing")
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", status.Error(codes.Unauthenticated, "invalid authorization metadata")
	}
	return parts[1], nil
}
