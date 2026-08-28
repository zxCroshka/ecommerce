package userservice

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMappingErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     codes.Code
		expected error
	}{
		{name: "invalid argument", code: codes.InvalidArgument, expected: customerrors.ErrInvalidArgument},
		{name: "unauthenticated", code: codes.Unauthenticated, expected: customerrors.ErrUnauthenticated},
		{name: "permission denied", code: codes.PermissionDenied, expected: customerrors.ErrPermissionDenied},
		{name: "not found", code: codes.NotFound, expected: customerrors.ErrNotFound},
		{name: "already exists", code: codes.AlreadyExists, expected: customerrors.ErrAlreadyExists},
		{name: "failed precondition", code: codes.FailedPrecondition, expected: customerrors.ErrFailedPrecondition},
		{name: "aborted", code: codes.Aborted, expected: customerrors.ErrFailedPrecondition},
		{name: "resource exhausted", code: codes.ResourceExhausted, expected: customerrors.ErrResourceExhausted},
		{name: "canceled", code: codes.Canceled, expected: customerrors.ErrCanceled},
		{name: "deadline exceeded", code: codes.DeadlineExceeded, expected: customerrors.ErrDeadlineExceeded},
		{name: "unavailable", code: codes.Unavailable, expected: customerrors.ErrServiceUnavailable},
		{name: "internal", code: codes.Internal, expected: customerrors.ErrInternal},
		{name: "unknown", code: codes.Unknown, expected: customerrors.ErrInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := mappingErrors("test.operation", status.Error(tt.code, "upstream message"))

			require.ErrorIs(t, err, tt.expected)
			require.ErrorContains(t, err, "test.operation")
			require.ErrorContains(t, err, "upstream message")
		})
	}
}

func TestMappingErrorsHandlesNilAndPlainErrors(t *testing.T) {
	t.Parallel()

	require.NoError(t, mappingErrors("test.operation", nil))
	require.ErrorIs(t, mappingErrors("test.operation", errors.New("plain error")), customerrors.ErrInternal)
	require.ErrorIs(t, mappingErrors("test.operation", context.Canceled), customerrors.ErrCanceled)
	require.ErrorIs(t, mappingErrors("test.operation", context.DeadlineExceeded), customerrors.ErrDeadlineExceeded)
}

func TestValidateTokenPairResponse(t *testing.T) {
	t.Parallel()

	valid := &userservicev1.TokenPairResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}
	require.NoError(t, ValidateTokenPairResponse(valid))

	tests := []struct {
		name     string
		response *userservicev1.TokenPairResponse
	}{
		{name: "nil response", response: nil},
		{name: "empty access token", response: &userservicev1.TokenPairResponse{RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 900}},
		{name: "empty refresh token", response: &userservicev1.TokenPairResponse{AccessToken: "access", TokenType: "Bearer", ExpiresIn: 900}},
		{name: "invalid token type", response: &userservicev1.TokenPairResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: "Basic", ExpiresIn: 900}},
		{name: "invalid expiration", response: &userservicev1.TokenPairResponse{AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTokenPairResponse(tt.response)

			require.ErrorIs(t, err, customerrors.ErrInternal)
		})
	}
}

func TestValidateUserResponses(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateTokenResponse(&userservicev1.ValidateTokenResponse{
		UserId: 42,
		Role:   "customer",
	}))
	require.NoError(t, ValidateGetUserResponse(&userservicev1.GetUserResponse{
		UserId: 42,
		Email:  "user@example.com",
		Name:   "User",
		Role:   "admin",
	}))

	require.ErrorIs(t, ValidateTokenResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateTokenResponse(&userservicev1.ValidateTokenResponse{}), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateTokenResponse(&userservicev1.ValidateTokenResponse{
		UserId: 42,
		Role:   "user",
	}), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateGetUserResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateGetUserResponse(&userservicev1.GetUserResponse{UserId: 42}), customerrors.ErrInternal)
}

func TestValidateEmptyResponses(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateRegisterResponse(&userservicev1.RegisterResponse{}))
	require.NoError(t, ValidateLogoutResponse(&userservicev1.LogoutResponse{}))
	require.NoError(t, ValidateUpdateEmailResponse(&userservicev1.UpdateEmailResponse{}))
	require.NoError(t, ValidateUpdateNameResponse(&userservicev1.UpdateNameResponse{}))
	require.NoError(t, ValidateUpdatePasswordResponse(&userservicev1.UpdatePasswordResponse{}))

	require.ErrorIs(t, ValidateRegisterResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateLogoutResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateUpdateEmailResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateUpdateNameResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateUpdatePasswordResponse(nil), customerrors.ErrInternal)
}
