package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type tokenValidatorStub struct {
	response *userservicev1.ValidateTokenResponse
	err      error
	token    string
}

func (s *tokenValidatorStub) ValidateToken(
	_ context.Context,
	req *userservicev1.ValidateTokenRequest,
	_ ...grpc.CallOption,
) (*userservicev1.ValidateTokenResponse, error) {
	s.token = req.GetToken()
	return s.response, s.err
}

func TestAuthInterceptor_PublicGetProduct(t *testing.T) {
	interceptor := NewAuthInterceptor("internal-token", nil).UnaryInterceptor()
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

func TestAuthInterceptor_PublicMethodUsesOptionalIdentity(t *testing.T) {
	validator := &tokenValidatorStub{
		response: &userservicev1.ValidateTokenResponse{UserId: 42, Role: adminRole},
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authorizationMetadataKey, "Bearer admin-token",
	))
	interceptor := NewAuthInterceptor("internal-token", validator).UnaryInterceptor()

	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: productservicev1.Products_ListProducts_FullMethodName},
		func(ctx context.Context, _ any) (any, error) {
			require.True(t, IsAdmin(ctx))
			identity, ok := IdentityFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, int64(42), identity.UserID)
			return nil, nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, "admin-token", validator.token)
}

func TestAuthInterceptor_PublicMethodRejectsInvalidProvidedToken(t *testing.T) {
	validator := &tokenValidatorStub{
		err: status.Error(codes.Unauthenticated, "invalid token"),
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authorizationMetadataKey, "Bearer invalid-token",
	))
	interceptor := NewAuthInterceptor("internal-token", validator).UnaryInterceptor()
	called := false

	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: productservicev1.Products_GetProduct_FullMethodName},
		func(context.Context, any) (any, error) {
			called = true
			return nil, nil
		},
	)

	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.False(t, called)
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
			interceptor := NewAuthInterceptor(internalToken, nil).UnaryInterceptor()
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

func TestAuthInterceptor_AdminMethods(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		validator  *tokenValidatorStub
		wantCode   codes.Code
		wantCalled bool
	}{
		{
			name:      "missing bearer token",
			ctx:       context.Background(),
			validator: &tokenValidatorStub{},
			wantCode:  codes.Unauthenticated,
		},
		{
			name: "invalid bearer format",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				authorizationMetadataKey, "access-token",
			)),
			validator: &tokenValidatorStub{},
			wantCode:  codes.Unauthenticated,
		},
		{
			name: "invalid token",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				authorizationMetadataKey, "Bearer invalid-token",
			)),
			validator: &tokenValidatorStub{
				err: status.Error(codes.Unauthenticated, "invalid token"),
			},
			wantCode: codes.Unauthenticated,
		},
		{
			name: "customer is forbidden",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				authorizationMetadataKey, "Bearer customer-token",
			)),
			validator: &tokenValidatorStub{
				response: &userservicev1.ValidateTokenResponse{UserId: 12, Role: "customer"},
			},
			wantCode: codes.PermissionDenied,
		},
		{
			name: "admin is allowed",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				authorizationMetadataKey, "Bearer admin-token",
			)),
			validator: &tokenValidatorStub{
				response: &userservicev1.ValidateTokenResponse{UserId: 42, Role: adminRole},
			},
			wantCode:   codes.OK,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := NewAuthInterceptor("internal-token", tt.validator).UnaryInterceptor()
			called := false

			_, err := interceptor(
				tt.ctx,
				nil,
				&grpc.UnaryServerInfo{
					FullMethod: productservicev1.Products_CreateProduct_FullMethodName,
				},
				func(ctx context.Context, _ any) (any, error) {
					called = true
					identity, ok := IdentityFromContext(ctx)
					require.True(t, ok)
					require.Equal(t, int64(42), identity.UserID)
					require.Equal(t, adminRole, identity.Role)
					require.True(t, IsAdmin(ctx))
					return nil, nil
				},
			)

			require.Equal(t, tt.wantCode, status.Code(err))
			require.Equal(t, tt.wantCalled, called)
		})
	}
}

func TestAuthInterceptor_DeniesMethodWithoutPolicy(t *testing.T) {
	interceptor := NewAuthInterceptor("internal-token", nil).UnaryInterceptor()
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
