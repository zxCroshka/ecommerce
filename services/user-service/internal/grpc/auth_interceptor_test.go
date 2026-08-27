package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	userauth "github.com/zxCroshka/ecommerce/services/user-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type tokenValidatorFunc func(context.Context, string) (domain.Identity, error)

func (f tokenValidatorFunc) ValidateToken(ctx context.Context, token string) (domain.Identity, error) {
	return f(ctx, token)
}

func TestAuthInterceptor_PublicMethodsBypassAuthentication(t *testing.T) {
	methods := []string{
		userservicev1.User_ValidateToken_FullMethodName,
		userservicev1.User_Login_FullMethodName,
		userservicev1.User_Register_FullMethodName,
		userservicev1.User_RefreshTokens_FullMethodName,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			validatorCalled := false
			interceptor := NewAuthInterceptor(tokenValidatorFunc(
				func(context.Context, string) (domain.Identity, error) {
					validatorCalled = true
					return domain.Identity{}, errors.New("must not be called")
				},
			))
			handlerCalled := false

			response, err := interceptor.UnaryInterceptor()(
				context.Background(), "request",
				&grpc.UnaryServerInfo{FullMethod: method},
				func(ctx context.Context, req any) (any, error) {
					handlerCalled = true
					require.Equal(t, "request", req)
					_, exists := userauth.IdentityFromContext(ctx)
					require.False(t, exists)
					return "response", nil
				},
			)

			require.NoError(t, err)
			require.Equal(t, "response", response)
			require.True(t, handlerCalled)
			require.False(t, validatorCalled)
		})
	}
}

func TestAuthInterceptor_ProtectedMethodsPropagateIdentity(t *testing.T) {
	methods := []string{
		userservicev1.User_GetUser_FullMethodName,
		userservicev1.User_UpdateEmail_FullMethodName,
		userservicev1.User_UpdateName_FullMethodName,
		userservicev1.User_UpdatePassword_FullMethodName,
		userservicev1.User_Logout_FullMethodName,
	}
	expected := validInterceptorIdentity()

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			interceptor := NewAuthInterceptor(tokenValidatorFunc(
				func(_ context.Context, token string) (domain.Identity, error) {
					require.Equal(t, "access-token", token)
					return expected, nil
				},
			))
			handlerCalled := false

			_, err := interceptor.UnaryInterceptor()(
				bearerIncomingContext("access-token"), nil,
				&grpc.UnaryServerInfo{FullMethod: method},
				func(ctx context.Context, _ any) (any, error) {
					handlerCalled = true
					identity, exists := userauth.IdentityFromContext(ctx)
					require.True(t, exists)
					require.Equal(t, expected, identity)
					return nil, nil
				},
			)

			require.NoError(t, err)
			require.True(t, handlerCalled)
		})
	}
}

func TestAuthInterceptor_RejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing metadata", ctx: context.Background()},
		{name: "missing authorization", ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{})},
		{
			name: "multiple values",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{
				"authorization": []string{"Bearer one", "Bearer two"},
			}),
		},
		{name: "wrong scheme", ctx: incomingAuthorization("Basic token")},
		{name: "missing token", ctx: incomingAuthorization("Bearer")},
		{name: "too many fields", ctx: incomingAuthorization("Bearer token extra")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validatorCalled := false
			interceptor := NewAuthInterceptor(tokenValidatorFunc(
				func(context.Context, string) (domain.Identity, error) {
					validatorCalled = true
					return validInterceptorIdentity(), nil
				},
			))
			handlerCalled := false

			_, err := interceptor.UnaryInterceptor()(
				tt.ctx, nil,
				&grpc.UnaryServerInfo{FullMethod: userservicev1.User_GetUser_FullMethodName},
				func(context.Context, any) (any, error) {
					handlerCalled = true
					return nil, nil
				},
			)

			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.False(t, validatorCalled)
			require.False(t, handlerCalled)
		})
	}
}

func TestAuthInterceptor_MapsValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "invalid token", err: customerrors.ErrInvalidToken, code: codes.Unauthenticated},
		{name: "blacklisted", err: customerrors.ErrTokenBlacklisted, code: codes.Unauthenticated},
		{name: "wrapped invalid", err: errors.Join(errors.New("validate"), customerrors.ErrInvalidToken), code: codes.Unauthenticated},
		{name: "redis failure", err: errors.New("redis unavailable"), code: codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := NewAuthInterceptor(tokenValidatorFunc(
				func(context.Context, string) (domain.Identity, error) {
					return domain.Identity{}, tt.err
				},
			))

			_, err := invokeProtected(interceptor, bearerIncomingContext("token"), func(context.Context, any) (any, error) {
				t.Fatal("handler must not be called")
				return nil, nil
			})
			require.Equal(t, tt.code, status.Code(err))
		})
	}
}

func TestAuthInterceptor_RejectsInvalidIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity domain.Identity
	}{
		{name: "missing user id", identity: domain.Identity{TokenID: "id", ExpiresAt: time.Now().Add(time.Minute)}},
		{name: "missing token id", identity: domain.Identity{UserID: 1, ExpiresAt: time.Now().Add(time.Minute)}},
		{name: "missing expiration", identity: domain.Identity{UserID: 1, TokenID: "id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := NewAuthInterceptor(tokenValidatorFunc(
				func(context.Context, string) (domain.Identity, error) { return tt.identity, nil },
			))
			_, err := invokeProtected(interceptor, bearerIncomingContext("token"), func(context.Context, any) (any, error) {
				t.Fatal("handler must not be called")
				return nil, nil
			})
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
}

func TestAuthInterceptor_DeniesUnknownMethod(t *testing.T) {
	interceptor := NewAuthInterceptor(nil)
	handlerCalled := false
	_, err := interceptor.UnaryInterceptor()(
		context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/userservice.User/FutureMethod"},
		func(context.Context, any) (any, error) {
			handlerCalled = true
			return nil, nil
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, handlerCalled)
}

func TestAuthInterceptor_RejectsMissingValidator(t *testing.T) {
	interceptor := NewAuthInterceptor(nil)
	_, err := invokeProtected(interceptor, bearerIncomingContext("token"), func(context.Context, any) (any, error) {
		t.Fatal("handler must not be called")
		return nil, nil
	})
	require.Equal(t, codes.Internal, status.Code(err))
}

func invokeProtected(
	interceptor *AuthInterceptor,
	ctx context.Context,
	handler grpc.UnaryHandler,
) (any, error) {
	return interceptor.UnaryInterceptor()(
		ctx, nil,
		&grpc.UnaryServerInfo{FullMethod: userservicev1.User_GetUser_FullMethodName},
		handler,
	)
}

func incomingAuthorization(value string) context.Context {
	return metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", value),
	)
}

func bearerIncomingContext(token string) context.Context {
	return incomingAuthorization("Bearer " + token)
}

func validInterceptorIdentity() domain.Identity {
	return domain.Identity{
		UserID:    42,
		Role:      domain.RoleCustomer,
		TokenID:   "access-id",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
}
