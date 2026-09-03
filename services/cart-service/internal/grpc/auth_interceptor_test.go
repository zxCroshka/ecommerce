package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/auth"
	cartservicev1 "github.com/zxCroshka/ecommerce/shared/cartservice/gen/go"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type cartTokenValidator struct {
	response *userservicev1.ValidateTokenResponse
	err      error
}

func (v *cartTokenValidator) ValidateToken(
	context.Context,
	*userservicev1.ValidateTokenRequest,
	...grpc.CallOption,
) (*userservicev1.ValidateTokenResponse, error) {
	return v.response, v.err
}

func TestCartAuthRejectsMissingToken(t *testing.T) {
	interceptor := NewAuthInterceptor("service-secret", &cartTokenValidator{}, time.Second)
	_, err := interceptor.UnaryInterceptor()(
		context.Background(),
		&cartservicev1.GetCartRequest{},
		&grpc.UnaryServerInfo{FullMethod: cartservicev1.Cart_GetCart_FullMethodName},
		func(context.Context, any) (any, error) { return nil, nil },
	)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCartAuthRejectsInvalidToken(t *testing.T) {
	interceptor := NewAuthInterceptor(
		"service-secret",
		&cartTokenValidator{err: status.Error(codes.Unauthenticated, "bad token")},
		time.Second,
	)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer invalid"))
	_, err := interceptor.UnaryInterceptor()(
		ctx,
		&cartservicev1.GetCartRequest{},
		&grpc.UnaryServerInfo{FullMethod: cartservicev1.Cart_GetCart_FullMethodName},
		func(context.Context, any) (any, error) { return nil, nil },
	)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestCartAuthProvidesVerifiedUser(t *testing.T) {
	interceptor := NewAuthInterceptor(
		"service-secret",
		&cartTokenValidator{response: &userservicev1.ValidateTokenResponse{UserId: 42, Role: "customer"}},
		time.Second,
	)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer valid"))
	_, err := interceptor.UnaryInterceptor()(
		ctx,
		&cartservicev1.GetCartRequest{UserId: 42},
		&grpc.UnaryServerInfo{FullMethod: cartservicev1.Cart_GetCart_FullMethodName},
		func(ctx context.Context, _ any) (any, error) {
			identity, ok := auth.UserIdentityFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, int64(42), identity.UserID)
			return nil, nil
		},
	)
	require.NoError(t, err)
}

func TestCartRejectsSubstitutedUserID(t *testing.T) {
	interceptor := NewAuthInterceptor(
		"service-secret",
		&cartTokenValidator{response: &userservicev1.ValidateTokenResponse{UserId: 42, Role: "customer"}},
		time.Second,
	)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer valid"))
	_, err := interceptor.UnaryInterceptor()(
		ctx,
		&cartservicev1.GetCartRequest{UserId: 99},
		&grpc.UnaryServerInfo{FullMethod: cartservicev1.Cart_GetCart_FullMethodName},
		func(ctx context.Context, request any) (any, error) {
			return nil, validateAuthenticatedUserID(ctx, request.(*cartservicev1.GetCartRequest).GetUserId())
		},
	)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCartInternalMethodRejectsInvalidServiceCredential(t *testing.T) {
	interceptor := NewAuthInterceptor("service-secret", nil, time.Second)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-service-token", "wrong"))
	_, err := interceptor.UnaryInterceptor()(
		ctx,
		&cartservicev1.CheckoutCartRequest{UserId: 42},
		&grpc.UnaryServerInfo{FullMethod: cartservicev1.Cart_CheckoutCart_FullMethodName},
		func(context.Context, any) (any, error) { return nil, nil },
	)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
