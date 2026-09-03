package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/zxCroshka/ecommerce/services/notification-service/internal/auth"
	notificationservicev1 "github.com/zxCroshka/ecommerce/shared/notificationservice/gen/go"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type TokenValidator interface {
	ValidateToken(context.Context, *userservicev1.ValidateTokenRequest, ...grpc.CallOption) (*userservicev1.ValidateTokenResponse, error)
}

type AuthInterceptor struct {
	validator TokenValidator
	timeout   time.Duration
}

func NewAuthInterceptor(validator TokenValidator, timeout time.Duration) *AuthInterceptor {
	return &AuthInterceptor{validator: validator, timeout: timeout}
}

func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		switch info.FullMethod {
		case notificationservicev1.Notifications_ListNotifications_FullMethodName,
			notificationservicev1.Notifications_MarkAsRead_FullMethodName:
			identity, err := a.authenticate(ctx)
			if err != nil {
				return nil, err
			}
			return handler(auth.WithUserIdentity(ctx, identity), request)
		default:
			return nil, status.Error(codes.PermissionDenied, "access policy is not configured")
		}
	}
}

func (a *AuthInterceptor) authenticate(ctx context.Context) (auth.UserIdentity, error) {
	values, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(values.Get("authorization")) != 1 {
		return auth.UserIdentity{}, status.Error(codes.Unauthenticated, "authorization metadata is missing")
	}
	parts := strings.Fields(values.Get("authorization")[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return auth.UserIdentity{}, status.Error(codes.Unauthenticated, "invalid authorization metadata")
	}
	if a.validator == nil {
		return auth.UserIdentity{}, status.Error(codes.Internal, "authentication service is not configured")
	}
	validationCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	response, err := a.validator.ValidateToken(validationCtx, &userservicev1.ValidateTokenRequest{Token: parts[1]})
	if err != nil {
		switch status.Code(err) {
		case codes.Unavailable:
			return auth.UserIdentity{}, status.Error(codes.Unavailable, "authentication service is unavailable")
		case codes.DeadlineExceeded:
			return auth.UserIdentity{}, status.Error(codes.DeadlineExceeded, "authentication request timed out")
		case codes.Canceled:
			return auth.UserIdentity{}, status.Error(codes.Canceled, "authentication request canceled")
		default:
			return auth.UserIdentity{}, status.Error(codes.Unauthenticated, "invalid access token")
		}
	}
	if response.GetUserId() <= 0 || strings.TrimSpace(response.GetRole()) == "" {
		return auth.UserIdentity{}, status.Error(codes.Unauthenticated, "invalid access token")
	}
	return auth.UserIdentity{UserID: response.GetUserId(), Role: response.GetRole()}, nil
}
