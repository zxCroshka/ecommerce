package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	userauth "github.com/zxCroshka/ecommerce/services/user-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type userServiceStub struct {
	registerFn       func(context.Context, string, string, string) error
	loginFn          func(context.Context, string, string) (*jwt.TokenPair, error)
	refreshFn        func(context.Context, string) (*jwt.TokenPair, error)
	logoutFn         func(context.Context, domain.Identity, string) error
	updateEmailFn    func(context.Context, int64, string) error
	updateNameFn     func(context.Context, int64, string) error
	updatePasswordFn func(context.Context, int64, string, string) error
	getUserFn        func(context.Context, int64) (domain.User, error)
	validateTokenFn  func(context.Context, string) (domain.Identity, error)
}

func (s *userServiceStub) Register(ctx context.Context, email, password, name string) error {
	return s.registerFn(ctx, email, password, name)
}

func (s *userServiceStub) Login(ctx context.Context, email, password string) (*jwt.TokenPair, error) {
	return s.loginFn(ctx, email, password)
}

func (s *userServiceStub) RefreshTokens(ctx context.Context, token string) (*jwt.TokenPair, error) {
	return s.refreshFn(ctx, token)
}

func (s *userServiceStub) Logout(ctx context.Context, identity domain.Identity, token string) error {
	return s.logoutFn(ctx, identity, token)
}

func (s *userServiceStub) UpdateEmail(ctx context.Context, userID int64, email string) error {
	return s.updateEmailFn(ctx, userID, email)
}

func (s *userServiceStub) UpdateName(ctx context.Context, userID int64, name string) error {
	return s.updateNameFn(ctx, userID, name)
}

func (s *userServiceStub) UpdatePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	return s.updatePasswordFn(ctx, userID, oldPassword, newPassword)
}

func (s *userServiceStub) GetUser(ctx context.Context, userID int64) (domain.User, error) {
	return s.getUserFn(ctx, userID)
}

func (s *userServiceStub) ValidateToken(ctx context.Context, token string) (domain.Identity, error) {
	return s.validateTokenFn(ctx, token)
}

func TestValidateToken(t *testing.T) {
	server := &ServerAPI{usrservice: &userServiceStub{
		validateTokenFn: func(_ context.Context, token string) (domain.Identity, error) {
			require.Equal(t, "valid-token", token)
			return domain.Identity{UserID: 123, Role: domain.RoleAdmin}, nil
		},
	}}

	response, err := server.ValidateToken(context.Background(), &userservicev1.ValidateTokenRequest{Token: "valid-token"})
	require.NoError(t, err)
	require.Equal(t, int64(123), response.GetUserId())
	require.Equal(t, "admin", response.GetRole())
}

func TestValidateTokenMapsBlacklist(t *testing.T) {
	server := &ServerAPI{usrservice: &userServiceStub{
		validateTokenFn: func(context.Context, string) (domain.Identity, error) {
			return domain.Identity{}, customerrors.ErrTokenBlacklisted
		},
	}}

	_, err := server.ValidateToken(context.Background(), &userservicev1.ValidateTokenRequest{Token: "token"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestPublicAuthMethods(t *testing.T) {
	pair := &jwt.TokenPair{AccessToken: "access", RefreshToken: "refresh"}
	server := &ServerAPI{usrservice: &userServiceStub{
		registerFn: func(_ context.Context, email, password, name string) error {
			require.Equal(t, "user@example.com", email)
			require.Equal(t, "password123", password)
			require.Equal(t, "anonymous user", name)
			return nil
		},
		loginFn: func(_ context.Context, email, password string) (*jwt.TokenPair, error) {
			require.Equal(t, "user@example.com", email)
			require.Equal(t, "password123", password)
			return pair, nil
		},
		refreshFn: func(_ context.Context, token string) (*jwt.TokenPair, error) {
			require.Equal(t, "old-refresh", token)
			return pair, nil
		},
	}}

	_, err := server.Register(context.Background(), &userservicev1.RegisterRequest{
		Email: "user@example.com", Password: "password123",
	})
	require.NoError(t, err)

	loginResponse, err := server.Login(context.Background(), &userservicev1.LoginRequest{
		Email: "user@example.com", Password: "password123",
	})
	require.NoError(t, err)
	require.Equal(t, "access", loginResponse.GetAccessToken())
	require.Equal(t, accessExpiresIn, loginResponse.GetExpiresIn())

	refreshResponse, err := server.RefreshTokens(context.Background(), &userservicev1.RefreshTokensRequest{
		RefreshToken: "old-refresh",
	})
	require.NoError(t, err)
	require.Equal(t, "refresh", refreshResponse.GetRefreshToken())
}

func TestProtectedMethodsUseContextIdentity(t *testing.T) {
	identity := domain.Identity{
		UserID:    42,
		Role:      domain.RoleCustomer,
		TokenID:   "access-id",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	ctx := userauth.WithIdentity(context.Background(), identity)
	server := &ServerAPI{usrservice: &userServiceStub{
		getUserFn: func(_ context.Context, userID int64) (domain.User, error) {
			require.Equal(t, identity.UserID, userID)
			return domain.User{Id: userID, Email: "user@example.com", Name: "User", Role: identity.Role}, nil
		},
		updateEmailFn: func(_ context.Context, userID int64, email string) error {
			require.Equal(t, identity.UserID, userID)
			require.Equal(t, "new@example.com", email)
			return nil
		},
		updateNameFn: func(_ context.Context, userID int64, name string) error {
			require.Equal(t, identity.UserID, userID)
			require.Equal(t, "New Name", name)
			return nil
		},
		updatePasswordFn: func(_ context.Context, userID int64, oldPassword, newPassword string) error {
			require.Equal(t, identity.UserID, userID)
			require.Equal(t, "oldpass123", oldPassword)
			require.Equal(t, "newpass123", newPassword)
			return nil
		},
		logoutFn: func(_ context.Context, actual domain.Identity, refreshToken string) error {
			require.Equal(t, identity, actual)
			require.Equal(t, "refresh-token", refreshToken)
			return nil
		},
	}}

	profile, err := server.GetUser(ctx, &userservicev1.GetUserRequest{})
	require.NoError(t, err)
	require.Equal(t, identity.UserID, profile.GetUserId())

	_, err = server.UpdateEmail(ctx, &userservicev1.UpdateEmailRequest{NewEmail: "new@example.com"})
	require.NoError(t, err)
	_, err = server.UpdateName(ctx, &userservicev1.UpdateNameRequest{NewName: "New Name"})
	require.NoError(t, err)
	_, err = server.UpdatePassword(ctx, &userservicev1.UpdatePasswordRequest{
		OldPassword: "oldpass123", NewPassword: "newpass123",
	})
	require.NoError(t, err)
	_, err = server.Logout(ctx, &userservicev1.LogoutRequest{RefreshToken: "refresh-token"})
	require.NoError(t, err)
}

func TestProtectedMethodRejectsMissingIdentity(t *testing.T) {
	server := &ServerAPI{usrservice: &userServiceStub{}}
	_, err := server.GetUser(context.Background(), &userservicev1.GetUserRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestValidation(t *testing.T) {
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateValidateToken(nil)))
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateRegister(&userservicev1.RegisterRequest{})))
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateLogin(&userservicev1.LoginRequest{})))
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateRefreshTokens(nil)))
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateLogout(nil)))
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateUpdateEmail(nil)))
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateUpdateName(nil)))
	require.Equal(t, codes.InvalidArgument, status.Code(ValidateUpdatePassword(nil)))
}
