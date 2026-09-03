package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/auth"
	orderservicev1 "github.com/zxCroshka/ecommerce/shared/orderservice/gen/go"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type tokenValidatorStub struct {
	response *userservicev1.ValidateTokenResponse
	err      error
	calls    int
	token    string
}

func (s *tokenValidatorStub) ValidateToken(
	_ context.Context,
	request *userservicev1.ValidateTokenRequest,
	_ ...grpcpkg.CallOption,
) (*userservicev1.ValidateTokenResponse, error) {
	s.calls++
	s.token = request.GetToken()
	return s.response, s.err
}

func invokeOrderAuth(
	t *testing.T,
	interceptor *AuthInterceptor,
	ctx context.Context,
) (auth.UserIdentity, error) {
	t.Helper()
	var identity auth.UserIdentity
	_, err := interceptor.UnaryInterceptor()(
		ctx,
		&orderservicev1.CreateOrderRequest{},
		&grpcpkg.UnaryServerInfo{FullMethod: orderservicev1.Orders_CreateOrder_FullMethodName},
		func(handlerContext context.Context, _ any) (any, error) {
			var ok bool
			identity, ok = auth.UserIdentityFromContext(handlerContext)
			require.True(t, ok)
			return &orderservicev1.CreateOrderResponse{}, nil
		},
	)
	return identity, err
}

func TestOrderAuthRejectsMissingBearerToken(t *testing.T) {
	validator := &tokenValidatorStub{}
	identity, err := invokeOrderAuth(t, NewAuthInterceptor(validator, time.Second), context.Background())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, identity.UserID)
	require.Zero(t, validator.calls)
}

func TestOrderAuthRejectsInvalidToken(t *testing.T) {
	validator := &tokenValidatorStub{err: status.Error(codes.Unauthenticated, "invalid")}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer invalid-token"))
	identity, err := invokeOrderAuth(t, NewAuthInterceptor(validator, time.Second), ctx)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.Zero(t, identity.UserID)
	require.Equal(t, 1, validator.calls)
}

func TestOrderAuthAddsValidatedIdentityToContext(t *testing.T) {
	validator := &tokenValidatorStub{response: &userservicev1.ValidateTokenResponse{UserId: 42, Role: "customer"}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer valid-token"))
	identity, err := invokeOrderAuth(t, NewAuthInterceptor(validator, time.Second), ctx)
	require.NoError(t, err)
	require.Equal(t, auth.UserIdentity{UserID: 42, Role: "customer"}, identity)
	require.Equal(t, "valid-token", validator.token)
}
