package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
)

type JWTService struct {
	secretKey  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type TokenType string

const (
	AccessTokenType  TokenType = "access"
	RefreshTokenType TokenType = "refresh"
)

type TokenClaims struct {
	UserID    int64       `json:"user_id"`
	Email     string      `json:"email"`
	Role      domain.Role `json:"role"`
	TokenType TokenType   `json:"token_type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func NewJWTService(secretKey string, accessTTL, refreshTTL time.Duration) *JWTService {
	return &JWTService{
		secretKey:  []byte(secretKey),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (s *JWTService) GenerateTokenPair(userID int64, email string, role domain.Role) (*TokenPair, string, error) {
	accessTokenID := uuid.New().String()
	accessClaims := TokenClaims{
		UserID:    userID,
		Role:      role,
		Email:     email,
		TokenType: AccessTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        accessTokenID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(s.secretKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshTokenID := uuid.New().String()
	refreshClaims := TokenClaims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: RefreshTokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        refreshTokenID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(s.secretKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
	}, refreshTokenID, nil
}

func (s *JWTService) ValidateToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&TokenClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return s.secretKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (s *JWTService) GetRefreshTTL() time.Duration {
	return s.refreshTTL
}

func (s *JWTService) ValidateAccessToken(token string) (*TokenClaims, error) {
	return s.validateTokenType(token, AccessTokenType)
}

func (s *JWTService) ValidateRefreshToken(token string) (*TokenClaims, error) {
	return s.validateTokenType(token, RefreshTokenType)
}

func (s *JWTService) validateTokenType(
	tokenString string,
	expectedType TokenType,
) (*TokenClaims, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	if claims.TokenType != expectedType {
		return nil, fmt.Errorf(
			"unexpected token type: expected %q, got %q",
			expectedType,
			claims.TokenType,
		)
	}

	return claims, nil
}
