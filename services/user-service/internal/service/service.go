package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service/pwdgen"
)

type UserService struct {
	log        *slog.Logger
	usrManager UserManager
	tknManager TokenManager
	jwtManager JWTManager
}

func NewUserService(log *slog.Logger, usrManager UserManager, tknManager TokenManager, jwtManager JWTManager) *UserService {
	return &UserService{
		log:        log,
		usrManager: usrManager,
		tknManager: tknManager,
		jwtManager: jwtManager,
	}
}

type UserManager interface {
	RegisterUserTX(
		ctx context.Context,
		email string,
		passHash []byte,
		name string,
		role domain.Role,
	) (int64, error)
	User(ctx context.Context, email string) (domain.User, error)
	UserByID(ctx context.Context, userID int64) (domain.User, error)
	Role(ctx context.Context, userID int64) (domain.Role, error)
	UpdateName(ctx context.Context, userID int64, newName string) error
	UpdateEmail(ctx context.Context, userID int64, newEmail string) error
	UpdatePassword(ctx context.Context, userId int64, newPassHash []byte) error
}

type TokenManager interface {
	SaveRefreshToken(ctx context.Context, userID int64, tokenID string, ttl time.Duration) error
	ValidateRefreshToken(ctx context.Context, userID int64, tokenID string) (bool, error)
	DeleteRefreshToken(ctx context.Context, userID int64, tokenID string) error
	AddToBlacklist(ctx context.Context, tokenID string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, tokenID string) (bool, error)
	RotateRefreshToken(ctx context.Context, userID int64, oldTokenID string, newTokenID string, ttl time.Duration) (bool, error)
}

type JWTManager interface {
	GenerateTokenPair(userID int64, email string, role domain.Role) (*jwt.TokenPair, string, error)
	GetRefreshTTL() time.Duration
	ValidateAccessToken(token string) (*jwt.TokenClaims, error)
	ValidateRefreshToken(token string) (*jwt.TokenClaims, error)
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must contain at least 8 characters")
	}
	return nil
}

func (s *UserService) Register(ctx context.Context, email string, password string, name string) error {
	const op = "service.Register"
	log := s.log.With(slog.String("op", op))
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if err := ValidatePassword(password); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	passHash := pwdgen.Generate([]byte(password))
	_, err := s.usrManager.RegisterUserTX(ctx, email, passHash, name, domain.RoleCustomer)
	if err != nil {
		if errors.Is(err, customerrors.ErrDuplicateEmail) {
			log.Warn("user already exists")
			return fmt.Errorf("%s: %w", op, err)
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *UserService) Login(ctx context.Context, email string, password string) (*jwt.TokenPair, error) {
	const op = "service.Login"
	log := s.log.With(slog.String("op", op))

	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.usrManager.User(ctx, email)
	if err != nil {
		log.Error("invalid credentials")
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrInvalidCredentials)
	}

	if !pwdgen.Check([]byte(password), user.PassHash) {
		log.Error("invalid credentials")
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrInvalidCredentials)
	}

	tokenPair, refreshTokenID, err := s.jwtManager.GenerateTokenPair(user.Id, user.Email, user.Role)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrRefreshTokenNotFound)
	}

	if err := s.tknManager.SaveRefreshToken(ctx, user.Id, refreshTokenID, s.jwtManager.GetRefreshTTL()); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return tokenPair, nil
}

func (s *UserService) RefreshTokens(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	const op = "service.RefreshTokens"
	log := s.log.With(slog.String("op", op))

	claims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		log.Warn("invalid refresh token", "error", err)
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}
	newTokenPair, refreshTokenID, err := s.jwtManager.GenerateTokenPair(claims.UserID, claims.Email, claims.Role)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	rotated, err := s.tknManager.RotateRefreshToken(ctx, claims.UserID, claims.ID, refreshTokenID, s.jwtManager.GetRefreshTTL())
	if err != nil {
		return nil, fmt.Errorf("%s: rotate refresh token: %w", op, err)
	}
	if !rotated {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrRefreshTokenNotFound)
	}
	return newTokenPair, nil
}

func (s *UserService) Logout(ctx context.Context, identity domain.Identity, refreshToken string) error {
	const op = "service.Logout"
	if identity.UserID <= 0 || identity.TokenID == "" || identity.ExpiresAt.IsZero() {
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}

	refreshClaims, err := s.jwtManager.ValidateRefreshToken(refreshToken)
	if err != nil {
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}

	if identity.UserID != refreshClaims.UserID {
		return fmt.Errorf("%s: token belongs to different users %w", op, customerrors.ErrInvalidToken)
	}

	if err := s.tknManager.DeleteRefreshToken(ctx, refreshClaims.UserID, refreshClaims.ID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	remainingTTL := time.Until(identity.ExpiresAt)
	if remainingTTL <= 0 {
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}
	if err := s.tknManager.AddToBlacklist(ctx, identity.TokenID, remainingTTL); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *UserService) ValidateToken(ctx context.Context, token string) (domain.Identity, error) {
	const op = "service.ValidateToken"
	log := s.log.With(slog.String("op", op))

	claims, err := s.jwtManager.ValidateAccessToken(token)
	if err != nil {
		log.Error("failed to validate token", "error", err)
		return domain.Identity{}, fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}
	if claims.ExpiresAt == nil {
		return domain.Identity{}, fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}

	exists, err := s.tknManager.IsBlacklisted(ctx, claims.ID)
	if err != nil {
		return domain.Identity{}, fmt.Errorf("%s: %w", op, err)
	}
	if exists {
		log.Warn("token is blacklisted", "token_id", claims.ID)
		return domain.Identity{}, fmt.Errorf("%s: %w", op, customerrors.ErrTokenBlacklisted)
	}

	return domain.Identity{
		UserID:    claims.UserID,
		Role:      claims.Role,
		TokenID:   claims.ID,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

func (s *UserService) GetUser(ctx context.Context, userID int64) (domain.User, error) {
	const op = "service.GetUser"
	log := s.log.With(slog.String("op", op))

	user, err := s.usrManager.UserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrUserNotFound) {
			log.Error("user is not found")
			return domain.User{}, fmt.Errorf("%s: %w", op, customerrors.ErrUserNotFound)
		}
		log.Error("failed to get user")
		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil

}

func (s *UserService) UpdateEmail(ctx context.Context, userID int64, newEmail string) error {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	const op = "service.UpdateEmail"
	log := s.log.With(slog.String("op", op))

	if userID <= 0 {
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}

	existingUser, err := s.usrManager.User(ctx, newEmail)
	if err != nil && !errors.Is(err, customerrors.ErrUserNotFound) {
		log.Error("failed to check email uniqueness", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if existingUser.Id != 0 && existingUser.Id != userID {
		return fmt.Errorf("%s: %w", op, customerrors.ErrDuplicateEmail)
	}

	if existingUser.Id == userID {
		log.Info("email is the same, skipping update")
		return nil
	}

	if err := s.usrManager.UpdateEmail(ctx, userID, newEmail); err != nil {
		log.Error("failed to update email", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("email updated successfully", "user_id", userID)
	return nil
}

func (s *UserService) UpdateName(ctx context.Context, userID int64, newName string) error {
	const op = "service.UpdateName"
	log := s.log.With(slog.String("op", op))

	if userID <= 0 {
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}
	if err := s.usrManager.UpdateName(ctx, userID, strings.TrimSpace(newName)); err != nil {
		log.Error("failed to update name")
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *UserService) UpdatePassword(ctx context.Context, userID int64, oldPassword string, newPassword string) error {
	const op = "service.UpdatePassword"
	log := s.log.With(slog.String("op", op))
	if err := ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if oldPassword == newPassword {
		return fmt.Errorf("%s: %w", op, customerrors.ErrSamePassword)
	}

	if userID <= 0 {
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}
	user, err := s.usrManager.UserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrUserNotFound) {
			log.Error("user is not found")
			return fmt.Errorf("%s: %w", op, customerrors.ErrUserNotFound)
		}
		log.Error("failed to get user")
		return fmt.Errorf("%s: %w", op, err)
	}

	if !pwdgen.Check([]byte(oldPassword), user.PassHash) {
		log.Error("invalid credentials")
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidCredentials)
	}

	newPasshash := pwdgen.Generate([]byte(newPassword))
	if err := s.usrManager.UpdatePassword(ctx, user.Id, newPasshash); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	log.Info("update password successfully")
	return nil
}
