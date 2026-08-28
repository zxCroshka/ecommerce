package grpc

import (
	"context"
	"errors"
	"strings"

	userauth "github.com/zxCroshka/ecommerce/services/user-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	tokenType       = "Bearer"
	accessExpiresIn = int64(900)
)

type UserService interface {
	Register(ctx context.Context, email, password, name string) error
	Login(ctx context.Context, email, password string) (*jwt.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*jwt.TokenPair, error)
	Logout(ctx context.Context, identity domain.Identity, refreshToken string) error
	UpdateEmail(ctx context.Context, userID int64, newEmail string) error
	UpdateName(ctx context.Context, userID int64, newName string) error
	UpdatePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error
	GetUser(ctx context.Context, userID int64) (domain.User, error)
	ValidateToken(ctx context.Context, token string) (domain.Identity, error)
}

type ServerAPI struct {
	userservicev1.UnimplementedUserServer
	usrservice UserService
}

func RegisterServerAPI(server *grpc.Server, usrservice UserService) {
	userservicev1.RegisterUserServer(server, &ServerAPI{usrservice: usrservice})
}

func (s *ServerAPI) ValidateToken(
	ctx context.Context,
	req *userservicev1.ValidateTokenRequest,
) (*userservicev1.ValidateTokenResponse, error) {
	if err := ValidateValidateToken(req); err != nil {
		return nil, err
	}

	identity, err := s.usrservice.ValidateToken(ctx, strings.TrimSpace(req.GetToken()))
	if err != nil {
		switch {
		case errors.Is(err, customerrors.ErrInvalidToken):
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		case errors.Is(err, customerrors.ErrTokenBlacklisted):
			return nil, status.Error(codes.Unauthenticated, "token is blacklisted")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &userservicev1.ValidateTokenResponse{
		UserId: identity.UserID,
		Role:   string(identity.Role),
	}, nil
}

func (s *ServerAPI) Register(
	ctx context.Context,
	req *userservicev1.RegisterRequest,
) (*userservicev1.RegisterResponse, error) {
	if err := ValidateRegister(req); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = "anonymous user"
	}
	if err := s.usrservice.Register(ctx, req.GetEmail(), req.GetPassword(), name); err != nil {
		if errors.Is(err, customerrors.ErrDuplicateEmail) {
			return nil, status.Error(codes.AlreadyExists, "user with this email already exists")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &userservicev1.RegisterResponse{}, nil
}

func (s *ServerAPI) Login(
	ctx context.Context,
	req *userservicev1.LoginRequest,
) (*userservicev1.TokenPairResponse, error) {
	if err := ValidateLogin(req); err != nil {
		return nil, err
	}

	tokenPair, err := s.usrservice.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidCredentials) {
			return nil, status.Error(codes.Unauthenticated, "invalid email or password")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return tokenPairResponse(tokenPair), nil
}

func (s *ServerAPI) RefreshTokens(
	ctx context.Context,
	req *userservicev1.RefreshTokensRequest,
) (*userservicev1.TokenPairResponse, error) {
	if err := ValidateRefreshTokens(req); err != nil {
		return nil, err
	}

	tokenPair, err := s.usrservice.RefreshTokens(ctx, req.GetRefreshToken())
	if err != nil {
		if errors.Is(err, customerrors.ErrRefreshTokenNotFound) ||
			errors.Is(err, customerrors.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return tokenPairResponse(tokenPair), nil
}

func (s *ServerAPI) Logout(
	ctx context.Context,
	req *userservicev1.LogoutRequest,
) (*userservicev1.LogoutResponse, error) {
	if err := ValidateLogout(req); err != nil {
		return nil, err
	}
	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.usrservice.Logout(ctx, identity, req.GetRefreshToken()); err != nil {
		if errors.Is(err, customerrors.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid token pair")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &userservicev1.LogoutResponse{}, nil
}

func (s *ServerAPI) GetUser(
	ctx context.Context,
	_ *userservicev1.GetUserRequest,
) (*userservicev1.GetUserResponse, error) {
	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}

	user, err := s.usrservice.GetUser(ctx, identity.UserID)
	if err != nil {
		if errors.Is(err, customerrors.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &userservicev1.GetUserResponse{
		UserId: user.Id,
		Email:  user.Email,
		Name:   user.Name,
		Role:   string(user.Role),
	}, nil
}

func (s *ServerAPI) UpdateEmail(
	ctx context.Context,
	req *userservicev1.UpdateEmailRequest,
) (*userservicev1.UpdateEmailResponse, error) {
	if err := ValidateUpdateEmail(req); err != nil {
		return nil, err
	}
	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.usrservice.UpdateEmail(ctx, identity.UserID, req.GetNewEmail()); err != nil {
		if errors.Is(err, customerrors.ErrDuplicateEmail) {
			return nil, status.Error(codes.AlreadyExists, "email already exists")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &userservicev1.UpdateEmailResponse{}, nil
}

func (s *ServerAPI) UpdateName(
	ctx context.Context,
	req *userservicev1.UpdateNameRequest,
) (*userservicev1.UpdateNameResponse, error) {
	if err := ValidateUpdateName(req); err != nil {
		return nil, err
	}
	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.usrservice.UpdateName(ctx, identity.UserID, req.GetNewName()); err != nil {
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &userservicev1.UpdateNameResponse{}, nil
}

func (s *ServerAPI) UpdatePassword(
	ctx context.Context,
	req *userservicev1.UpdatePasswordRequest,
) (*userservicev1.UpdatePasswordResponse, error) {
	if err := ValidateUpdatePassword(req); err != nil {
		return nil, err
	}
	identity, err := authenticatedIdentity(ctx)
	if err != nil {
		return nil, err
	}

	err = s.usrservice.UpdatePassword(
		ctx,
		identity.UserID,
		req.GetOldPassword(),
		req.GetNewPassword(),
	)
	if err != nil {
		switch {
		case errors.Is(err, customerrors.ErrInvalidCredentials):
			return nil, status.Error(codes.Unauthenticated, "invalid old password")
		case errors.Is(err, customerrors.ErrSamePassword):
			return nil, status.Error(codes.InvalidArgument, "new password must differ from old password")
		case errors.Is(err, customerrors.ErrUserNotFound):
			return nil, status.Error(codes.NotFound, "user not found")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}
	return &userservicev1.UpdatePasswordResponse{}, nil
}

func authenticatedIdentity(ctx context.Context) (domain.Identity, error) {
	identity, ok := userauth.IdentityFromContext(ctx)
	if !ok || identity.UserID <= 0 {
		return domain.Identity{}, status.Error(codes.Unauthenticated, "authenticated identity is missing")
	}
	return identity, nil
}

func tokenPairResponse(pair *jwt.TokenPair) *userservicev1.TokenPairResponse {
	return &userservicev1.TokenPairResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    tokenType,
		ExpiresIn:    accessExpiresIn,
	}
}
