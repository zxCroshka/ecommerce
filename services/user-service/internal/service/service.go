package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	producer   Producer
	jwtManager JWTManager
}

func NewUserService(log *slog.Logger, usrManager UserManager, tknManager TokenManager, producer Producer, jwtManager JWTManager) *UserService {
	return &UserService{
		log:        log,
		usrManager: usrManager,
		tknManager: tknManager,
		producer:   producer,
		jwtManager: jwtManager,
	}
}

type UserManager interface {
	RegisterUserTX(
		ctx context.Context,
		email string,
		passHash []byte,
		name string,
		isAdmin bool,
	) (int64, error)
	User(ctx context.Context, email string) (domain.User, error)
	UserByID(ctx context.Context, userID int64) (domain.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
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
}

type Producer interface {
	Close()
	Produce(userID int64, email string, name string) error
}

type JWTManager interface {
	GenerateTokenPair(userID int64, email string, isAdmin bool) (*jwt.TokenPair, string, error)
	ValidateToken(tokenString string) (*jwt.TokenClaims, error)
	GetRefreshTTL() time.Duration
}

func (s *UserService) Register(ctx context.Context, email string, password string, name string, isAdmin bool) error {
	const op = "service.Register"
	log := s.log.With(slog.String("op", op))
	passHash := pwdgen.Generate([]byte(password))
	userID, err := s.usrManager.RegisterUserTX(ctx, email, passHash, name, isAdmin)
	if err != nil {
		if errors.Is(err, customerrors.ErrUserExists) {
			log.Warn("user already exists")
			return fmt.Errorf("%s: %w", op, err)
		}
		return fmt.Errorf("%s: %w", op, err)
	}

	go func() {
		if err := s.producer.Produce(userID, email, name); err != nil {
			s.log.Error("Kafka produce failed", "error", err, "user_id", userID)
		}
	}()
	return nil
}

func (s *UserService) Login(ctx context.Context, email string, password string) (*jwt.TokenPair, error) {
	const op = "service.Login"
	log := s.log.With(slog.String("op", op))

	user, err := s.usrManager.User(ctx, email)
	if err != nil {
		log.Error("invalid credentials")
		return nil, fmt.Errorf("%s: %w",op, customerrors.ErrInvalidCredentials)
	}

	if !pwdgen.Check([]byte(password), user.PassHash) {
		log.Error("invalid credentials")
		return nil, fmt.Errorf("%s: %w",op, customerrors.ErrInvalidCredentials)
	}

	tokenPair, refreshTokenID, err := s.jwtManager.GenerateTokenPair(user.Id, user.Email, user.IsAdmin)
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

	claims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		log.Error("invalid refresh token")
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	exists, err := s.tknManager.ValidateRefreshToken(ctx, claims.UserID, claims.ID)
	if err != nil || !exists {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := s.tknManager.DeleteRefreshToken(ctx, claims.UserID, claims.ID); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	newTokenPair, refreshTokenID, err := s.jwtManager.GenerateTokenPair(claims.UserID, claims.Email, claims.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := s.tknManager.SaveRefreshToken(ctx, claims.UserID, refreshTokenID, s.jwtManager.GetRefreshTTL()); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return newTokenPair, nil
}

func (s *UserService) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	const op = "service.Logout"
	log := s.log.With(slog.String("op", op))

	accessClaims, err := s.jwtManager.ValidateToken(accessToken)
	if err != nil {
		log.Error("")
		return fmt.Errorf("%s: %w", op, err)
	}
	remainingTTL := time.Until(accessClaims.ExpiresAt.Time)

	if err := s.tknManager.AddToBlacklist(ctx, accessClaims.ID, remainingTTL); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	refreshClaims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.tknManager.DeleteRefreshToken(ctx, refreshClaims.UserID, refreshClaims.ID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *UserService) ValidateToken(ctx context.Context, token string) (int64, bool, error) {
	const op = "service.ValidateToken"
	log := s.log.With(slog.String("op", op))

	claims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		log.Error("failed to validate token", "error", err)
		return 0, false, fmt.Errorf("%s: %w", op, err)
	}

	exists, err := s.tknManager.IsBlacklisted(ctx, claims.ID)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", op, err)
	}
	if exists {
		log.Warn("token is blacklisted", "token_id", claims.ID)
		return 0, false, fmt.Errorf("%s: %w", op, customerrors.ErrTokenBlacklisted)
	}

	return claims.UserID, claims.IsAdmin, nil
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

func (s *UserService) UpdateEmail(ctx context.Context, token, newEmail string) error {
	const op = "service.UpdateEmail"
	log := s.log.With(slog.String("op", op))

	tokenClaims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	existingUser, err := s.usrManager.User(ctx, newEmail)
	if err != nil && !errors.Is(err, customerrors.ErrUserNotFound) {
		log.Error("failed to check email uniqueness", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if existingUser.Id != 0 && existingUser.Id != tokenClaims.UserID {
		return fmt.Errorf("%s: %w", op, customerrors.ErrDuplicateEmail)
	}

	if existingUser.Id == tokenClaims.UserID {
		log.Info("email is the same, skipping update")
		return nil
	}

	if err := s.usrManager.UpdateEmail(ctx, tokenClaims.UserID, newEmail); err != nil {
		log.Error("failed to update email", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("email updated successfully", "user_id", tokenClaims.UserID)
	return nil
}

func (s *UserService) UpdateName(ctx context.Context, token, newName string) error {
	const op = "service.UpdateName"
	log := s.log.With(slog.String("op", op))

	tokenClaims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := s.usrManager.UpdateName(ctx, tokenClaims.UserID, newName); err != nil {
		log.Error("failed to update name")
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *UserService) UpdatePassword(ctx context.Context, token string, oldPassword string, newPassword string) error {
	const op = "service.UpdatePassword"
	log := s.log.With(slog.String("op", op))

	if oldPassword == newPassword {
		return fmt.Errorf("%s: %w", op, customerrors.ErrSamePassword)
	}

	tokenClaims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		log.Error("invalid token", "error", err)
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidToken)
	}
	user, err := s.usrManager.User(ctx, tokenClaims.Email)
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
		return fmt.Errorf("%s: %w",op, customerrors.ErrInvalidCredentials)
	}

	newPasshash := pwdgen.Generate([]byte(newPassword))
	if err := s.usrManager.UpdatePassword(ctx, user.Id, newPasshash); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	log.Info("update password successfully")
	return nil
}
