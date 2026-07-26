package grpc

import (
	"context"
	"crypto/subtle"

	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const serviceTokenMetadataKey = "x-service-token"

type AuthInterceptor struct {
	internalToken string
}

func NewAuthInterceptor(internalToken string) *AuthInterceptor {
	return &AuthInterceptor{internalToken: internalToken}
}

func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		switch info.FullMethod {
		case productservicev1.Products_GetProduct_FullMethodName:
			return handler(ctx, req)
		case
			productservicev1.Products_ReserveStock_FullMethodName,
			productservicev1.Products_ReleaseStock_FullMethodName:
			if err := a.validateServiceToken(ctx); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		default:
			return nil, status.Error(codes.PermissionDenied, "access policy is not configured for this method")
		}
	}
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

	if subtle.ConstantTimeCompare([]byte(values[0]), []byte(a.internalToken)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid service credentials")
	}

	return nil
}
