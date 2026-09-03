package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/auth"
	notificationservicev1 "github.com/zxCroshka/ecommerce/shared/notificationservice/gen/go"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type validatorStub struct {
	response *userservicev1.ValidateTokenResponse
	err      error
}

func (s *validatorStub) ValidateToken(context.Context, *userservicev1.ValidateTokenRequest, ...grpcpkg.CallOption) (*userservicev1.ValidateTokenResponse, error) {
	return s.response, s.err
}

func invokeAuth(t *testing.T, interceptor *AuthInterceptor, ctx context.Context) (auth.UserIdentity, error) {
	t.Helper()
	var identity auth.UserIdentity
	_, err := interceptor.UnaryInterceptor()(
		ctx,
		&notificationservicev1.ListNotificationsRequest{},
		&grpcpkg.UnaryServerInfo{FullMethod: notificationservicev1.Notifications_ListNotifications_FullMethodName},
		func(handlerContext context.Context, _ any) (any, error) {
			var ok bool
			identity, ok = auth.UserIdentityFromContext(handlerContext)
			require.True(t, ok)
			return &notificationservicev1.ListNotificationsResponse{}, nil
		},
	)
	return identity, err
}

func TestAuthRejectsMissingAndInvalidBearer(t *testing.T) {
	interceptor := NewAuthInterceptor(&validatorStub{}, time.Second)
	_, err := invokeAuth(t, interceptor, context.Background())
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Basic token"))
	_, err = invokeAuth(t, interceptor, ctx)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAuthRejectsBearerRejectedByUserService(t *testing.T) {
	interceptor := NewAuthInterceptor(
		&validatorStub{err: status.Error(codes.Unauthenticated, "expired")},
		time.Second,
	)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer expired-token"))
	_, err := invokeAuth(t, interceptor, ctx)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAuthUsesValidatedIdentity(t *testing.T) {
	interceptor := NewAuthInterceptor(&validatorStub{response: &userservicev1.ValidateTokenResponse{UserId: 42, Role: "customer"}}, time.Second)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer valid-token"))
	identity, err := invokeAuth(t, interceptor, ctx)
	require.NoError(t, err)
	require.Equal(t, int64(42), identity.UserID)
}
