package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthInterceptor_PublicGetProduct(t *testing.T) {
	interceptor := NewAuthInterceptor("internal-token").UnaryInterceptor()
	called := false

	resp, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: productservicev1.Products_GetProduct_FullMethodName},
		func(context.Context, any) (any, error) {
			called = true
			return "ok", nil
		},
	)

	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "ok", resp)
}

func TestAuthInterceptor_StockMethodsRequireServiceToken(t *testing.T) {
	const internalToken = "internal-token"

	tests := []struct {
		name       string
		method     string
		ctx        context.Context
		wantCode   codes.Code
		wantCalled bool
	}{
		{
			name:     "reserve without token",
			method:   productservicev1.Products_ReserveStock_FullMethodName,
			ctx:      context.Background(),
			wantCode: codes.Unauthenticated,
		},
		{
			name:   "release with invalid token",
			method: productservicev1.Products_ReleaseStock_FullMethodName,
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				serviceTokenMetadataKey, "wrong-token",
			)),
			wantCode: codes.Unauthenticated,
		},
		{
			name:   "reserve with valid token",
			method: productservicev1.Products_ReserveStock_FullMethodName,
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				serviceTokenMetadataKey, internalToken,
			)),
			wantCode:   codes.OK,
			wantCalled: true,
		},
		{
			name:   "release with valid token",
			method: productservicev1.Products_ReleaseStock_FullMethodName,
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				serviceTokenMetadataKey, internalToken,
			)),
			wantCode:   codes.OK,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := NewAuthInterceptor(internalToken).UnaryInterceptor()
			called := false

			_, err := interceptor(
				tt.ctx,
				nil,
				&grpc.UnaryServerInfo{FullMethod: tt.method},
				func(context.Context, any) (any, error) {
					called = true
					return nil, nil
				},
			)

			require.Equal(t, tt.wantCode, status.Code(err))
			require.Equal(t, tt.wantCalled, called)
		})
	}
}

func TestAuthInterceptor_DeniesMethodWithoutPolicy(t *testing.T) {
	interceptor := NewAuthInterceptor("internal-token").UnaryInterceptor()
	called := false

	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/productservice.Products/Unknown"},
		func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		},
	)

	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, called)
}
