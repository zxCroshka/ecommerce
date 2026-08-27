package grpc

import (
	"context"
	"crypto/subtle"
	"strings"

	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authorizationMetadataKey = "authorization"
	serviceTokenMetadataKey  = "x-service-token"
	adminRole                = "admin"
)

type TokenValidator interface {
	ValidateToken(
		ctx context.Context,
		in *userservicev1.ValidateTokenRequest,
		opts ...grpc.CallOption,
	) (*userservicev1.ValidateTokenResponse, error)
}

type Identity struct {
	UserID int64
	Role   string
}

type identityContextKey struct{}

func withIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}

func IsAdmin(ctx context.Context) bool {
	identity, ok := IdentityFromContext(ctx)
	return ok && identity.Role == adminRole
}

type AuthInterceptor struct {
	internalToken  string
	tokenValidator TokenValidator
}

func NewAuthInterceptor(internalToken string, tokenValidator TokenValidator) *AuthInterceptor {
	return &AuthInterceptor{
		internalToken:  internalToken,
		tokenValidator: tokenValidator,
	}
}

func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		switch info.FullMethod {
		case productservicev1.Products_GetProduct_FullMethodName,
			productservicev1.Products_ListProducts_FullMethodName:
			if hasAuthorizationMetadata(ctx) {
				identity, err := a.authenticateUser(ctx)
				if err != nil {
					return nil, err
				}
				ctx = withIdentity(ctx, identity)
			}
			return handler(ctx, req)

		case productservicev1.Products_ReserveStock_FullMethodName,
			productservicev1.Products_ReleaseStock_FullMethodName:
			if err := a.validateServiceToken(ctx); err != nil {
				return nil, err
			}
			return handler(ctx, req)

		case productservicev1.Products_CreateProduct_FullMethodName,
			productservicev1.Products_UpdateProductFields_FullMethodName,
			productservicev1.Products_SoftDelete_FullMethodName:
			identity, err := a.authenticateUser(ctx)
			if err != nil {
				return nil, err
			}
			if identity.Role != adminRole {
				return nil, status.Error(codes.PermissionDenied, "admin role required")
			}
			return handler(withIdentity(ctx, identity), req)

		default:
			return nil, status.Error(codes.PermissionDenied, "access policy is not configured for this method")
		}
	}
}

func hasAuthorizationMetadata(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	return ok && len(md.Get(authorizationMetadataKey)) > 0
}

func (a *AuthInterceptor) authenticateUser(ctx context.Context) (Identity, error) {
	token, err := extractBearerToken(ctx)
	if err != nil {
		return Identity{}, err
	}
	if a.tokenValidator == nil {
		return Identity{}, status.Error(codes.Internal, "authentication service is not configured")
	}

	response, err := a.tokenValidator.ValidateToken(ctx, &userservicev1.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		switch status.Code(err) {
		case codes.Unavailable:
			return Identity{}, status.Error(codes.Unavailable, "authentication service is unavailable")
		case codes.DeadlineExceeded:
			return Identity{}, status.Error(codes.DeadlineExceeded, "authentication request timed out")
		case codes.Canceled:
			return Identity{}, status.Error(codes.Canceled, "authentication request was canceled")
		default:
			return Identity{}, status.Error(codes.Unauthenticated, "invalid access token")
		}
	}
	if response.GetUserId() <= 0 || response.GetRole() == "" {
		return Identity{}, status.Error(codes.Unauthenticated, "invalid access token")
	}

	return Identity{
		UserID: response.GetUserId(),
		Role:   response.GetRole(),
	}, nil
}

func (a *AuthInterceptor) validateServiceToken(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing service credentials")
	}

	values := md.Get(serviceTokenMetadataKey)
	if len(values) != 1 {
		return status.Error(codes.Unauthenticated, "missing service credentials")
	}

	if a.internalToken == "" ||
		subtle.ConstantTimeCompare([]byte(values[0]), []byte(a.internalToken)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid service credentials")
	}

	return nil
}

func extractBearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing authorization metadata")
	}

	values := md.Get(authorizationMetadataKey)
	if len(values) != 1 {
		return "", status.Error(codes.Unauthenticated, "missing authorization metadata")
	}

	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", status.Error(codes.Unauthenticated, "invalid authorization metadata")
	}

	return parts[1], nil
}
