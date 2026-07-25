package grpc

import (
	"context"
	"strings"

	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthInterceptor struct {
	userServiceClient userservicev1.UserClient
}

func NewAuthInterceptor(userServiceClient userservicev1.UserClient) *AuthInterceptor {
	return &AuthInterceptor{userServiceClient: userServiceClient}
}

func (a *AuthInterceptor) unaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		token, err := extractTokenFromMetadata(ctx)
		if err != nil {
			return nil, err
		}
		userID, role, err := a.validateToken(ctx, token)
		if err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, "userId", userID)
		ctx = context.WithValue(ctx, "isAdmin", role == "admin")
		return handler(ctx, req)
	}

}

func extractTokenFromMetadata(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return "", status.Errorf(codes.Unauthenticated, "missing authorization header")
	}
	authHeader := authHeaders[0]
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", status.Errorf(codes.Unauthenticated, "invalid authorization header format")
	}
	return parts[1], nil
}

func (a *AuthInterceptor) validateToken(ctx context.Context, token string) (int64, string, error) {
	resp, err := a.userServiceClient.ValidateToken(ctx, &userservicev1.ValidateTokenRequest{Token: token})
	if err != nil {
		return 0, "", status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}
	return resp.UserId, resp.Role, nil
}
