package jwt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
)

func TestNewJWTService(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour

	svc := NewJWTService(secret, accessTTL, refreshTTL)

	assert.NotNil(t, svc)
	assert.Equal(t, []byte(secret), svc.secretKey)
	assert.Equal(t, accessTTL, svc.accessTTL)
	assert.Equal(t, refreshTTL, svc.refreshTTL)
}

func TestJWTService_GenerateTokenPair(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	tests := []struct {
		name        string
		userID      int64
		email       string
		role        domain.Role
		shouldError bool
	}{
		{
			name:        "success - generate token pair for customer",
			userID:      1,
			email:       "customer@example.com",
			role:        domain.RoleCustomer,
			shouldError: false,
		},
		{
			name:        "success - generate token pair for admin",
			userID:      2,
			email:       "admin@example.com",
			role:        domain.RoleAdmin,
			shouldError: false,
		},
		{
			name:        "success - generate with empty email",
			userID:      3,
			email:       "",
			role:        domain.RoleCustomer,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenPair, refreshTokenID, err := svc.GenerateTokenPair(tt.userID, tt.email, tt.role)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, tokenPair)
				assert.Empty(t, refreshTokenID)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, tokenPair)
				assert.NotEmpty(t, tokenPair.AccessToken)
				assert.NotEmpty(t, tokenPair.RefreshToken)
				assert.NotEmpty(t, refreshTokenID)

				// Проверяем, что refreshTokenID - это UUID
				assert.Len(t, refreshTokenID, 36) // UUID has 36 characters
			}
		})
	}
}

func TestJWTService_ValidateToken_ValidTokens(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	t.Run("validate access token", func(t *testing.T) {
		userID := int64(123)
		email := "test@example.com"
		role := domain.RoleCustomer

		tokenPair, _, err := svc.GenerateTokenPair(userID, email, role)
		require.NoError(t, err)

		claims, err := svc.ValidateToken(tokenPair.AccessToken)

		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, role, claims.Role)
		assert.Equal(t, AccessTokenType, claims.TokenType)
		assert.NotEmpty(t, claims.ID)
		assert.NotZero(t, claims.IssuedAt)
		assert.NotZero(t, claims.ExpiresAt)
	})

	t.Run("validate refresh token", func(t *testing.T) {
		userID := int64(456)
		email := "refresh@example.com"
		role := domain.RoleAdmin

		tokenPair, _, err := svc.GenerateTokenPair(userID, email, role)
		require.NoError(t, err)

		claims, err := svc.ValidateToken(tokenPair.RefreshToken)

		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, role, claims.Role)
		assert.Equal(t, RefreshTokenType, claims.TokenType)
		assert.NotEmpty(t, claims.ID)
	})
}

func TestJWTService_RejectsWrongTokenType(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 7*24*time.Hour)
	pair, _, err := svc.GenerateTokenPair(1, "test@example.com", domain.RoleCustomer)
	require.NoError(t, err)

	claims, err := svc.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, AccessTokenType, claims.TokenType)

	claims, err = svc.ValidateRefreshToken(pair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, RefreshTokenType, claims.TokenType)

	_, err = svc.ValidateAccessToken(pair.RefreshToken)
	assert.Error(t, err)

	_, err = svc.ValidateRefreshToken(pair.AccessToken)
	assert.Error(t, err)
}

func TestJWTService_ValidateToken_ExpiredToken(t *testing.T) {
	// Используем очень маленький TTL для теста истечения
	secret := "test-secret-key"
	accessTTL := 1 * time.Second
	refreshTTL := 1 * time.Second
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	tokenPair, _, err := svc.GenerateTokenPair(1, "test@example.com", domain.RoleCustomer)
	require.NoError(t, err)

	// Сразу после создания токен должен быть валиден
	claims, err := svc.ValidateToken(tokenPair.AccessToken)
	assert.NoError(t, err)
	assert.NotNil(t, claims)

	// Ждем истечения TTL
	time.Sleep(1100 * time.Millisecond)

	// После истечения токен должен быть невалиден
	claims, err = svc.ValidateToken(tokenPair.AccessToken)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTService_ValidateToken_InvalidTokens(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	tests := []struct {
		name        string
		token       string
		shouldError bool
	}{
		{
			name:        "empty token",
			token:       "",
			shouldError: true,
		},
		{
			name:        "malformed token",
			token:       "not-a-valid-token",
			shouldError: true,
		},
		{
			name:        "token with wrong signature",
			token:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			shouldError: true,
		},
		{
			name:        "token with wrong algorithm",
			token:       "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := svc.ValidateToken(tt.token)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
			}
		})
	}
}

func TestJWTService_ValidateToken_WrongSecret(t *testing.T) {
	// Создаем сервис с одним секретом
	svc1 := NewJWTService("secret-1", 15*time.Minute, 168*time.Hour)

	// Генерируем токен
	tokenPair, _, err := svc1.GenerateTokenPair(1, "test@example.com", domain.RoleCustomer)
	require.NoError(t, err)

	// Создаем другой сервис с другим секретом
	svc2 := NewJWTService("secret-2", 15*time.Minute, 168*time.Hour)

	// Пытаемся валидировать токен другим секретом
	claims, err := svc2.ValidateToken(tokenPair.AccessToken)

	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTService_GetRefreshTTL(t *testing.T) {
	expectedTTL := 168 * time.Hour
	svc := NewJWTService("secret", 15*time.Minute, expectedTTL)

	assert.Equal(t, expectedTTL, svc.GetRefreshTTL())
}

func TestJWTService_TokenClaimsContent(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	userID := int64(12345)
	email := "full-test@example.com"
	role := domain.RoleAdmin

	tokenPair, refreshTokenID, err := svc.GenerateTokenPair(userID, email, role)
	require.NoError(t, err)

	// Проверяем claims access токена
	accessClaims, err := svc.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)

	assert.Equal(t, userID, accessClaims.UserID)
	assert.Equal(t, email, accessClaims.Email)
	assert.Equal(t, role, accessClaims.Role)
	assert.NotEmpty(t, accessClaims.ID)
	assert.NotZero(t, accessClaims.IssuedAt)
	assert.NotZero(t, accessClaims.ExpiresAt)

	// Проверяем, что ExpiresAt > IssuedAt
	assert.True(t, accessClaims.ExpiresAt.After(accessClaims.IssuedAt.Time))

	// Проверяем claims refresh токена
	refreshClaims, err := svc.ValidateToken(tokenPair.RefreshToken)
	require.NoError(t, err)

	assert.Equal(t, userID, refreshClaims.UserID)
	assert.Equal(t, email, refreshClaims.Email)
	assert.Equal(t, role, refreshClaims.Role)
	assert.Equal(t, refreshTokenID, refreshClaims.ID)
	assert.NotZero(t, refreshClaims.IssuedAt)
	assert.NotZero(t, refreshClaims.ExpiresAt)

	// Проверяем, что refresh token имеет больший TTL
	accessDuration := accessClaims.ExpiresAt.Sub(accessClaims.IssuedAt.Time)
	refreshDuration := refreshClaims.ExpiresAt.Sub(refreshClaims.IssuedAt.Time)
	assert.Less(t, accessDuration, refreshDuration)
}

func TestJWTService_DifferentUsersGetDifferentTokens(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	// Генерируем токены для двух разных пользователей
	tokenPair1, _, err := svc.GenerateTokenPair(1, "user1@example.com", domain.RoleCustomer)
	require.NoError(t, err)

	tokenPair2, _, err := svc.GenerateTokenPair(2, "user2@example.com", domain.RoleAdmin)
	require.NoError(t, err)

	// Токены должны быть разными
	assert.NotEqual(t, tokenPair1.AccessToken, tokenPair2.AccessToken)
	assert.NotEqual(t, tokenPair1.RefreshToken, tokenPair2.RefreshToken)

	// Валидируем и проверяем claims
	claims1, err := svc.ValidateToken(tokenPair1.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(1), claims1.UserID)
	assert.Equal(t, "user1@example.com", claims1.Email)
	assert.Equal(t, domain.RoleCustomer, claims1.Role)

	claims2, err := svc.ValidateToken(tokenPair2.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, int64(2), claims2.UserID)
	assert.Equal(t, "user2@example.com", claims2.Email)
	assert.Equal(t, domain.RoleAdmin, claims2.Role)
}

func TestJWTService_EachTokenHasUniqueID(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	// Генерируем несколько пар токенов
	ids := make(map[string]bool)

	for i := 0; i < 10; i++ {
		_, refreshTokenID, err := svc.GenerateTokenPair(int64(i), "test@example.com", domain.RoleCustomer)
		require.NoError(t, err)

		// Проверяем, что ID уникальный
		assert.False(t, ids[refreshTokenID], "Token ID should be unique")
		ids[refreshTokenID] = true
	}
}

func TestJWTService_TokenIDInClaims(t *testing.T) {
	secret := "test-secret-key"
	accessTTL := 15 * time.Minute
	refreshTTL := 168 * time.Hour
	svc := NewJWTService(secret, accessTTL, refreshTTL)

	tokenPair, refreshTokenID, err := svc.GenerateTokenPair(1, "test@example.com", domain.RoleCustomer)
	require.NoError(t, err)

	// Проверяем, что ID сохранен в claims refresh токена
	refreshClaims, err := svc.ValidateToken(tokenPair.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, refreshTokenID, refreshClaims.ID)

	// Проверяем, что ID сохранен в claims access токена (он должен быть другим)
	accessClaims, err := svc.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)
	assert.NotEqual(t, refreshTokenID, accessClaims.ID)
	assert.NotEmpty(t, accessClaims.ID)
}

// Benchmark тесты
func BenchmarkGenerateTokenPair(b *testing.B) {
	svc := NewJWTService("bench-secret", 15*time.Minute, 168*time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := svc.GenerateTokenPair(1, "bench@example.com", domain.RoleCustomer)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateToken(b *testing.B) {
	svc := NewJWTService("bench-secret", 15*time.Minute, 168*time.Hour)
	tokenPair, _, err := svc.GenerateTokenPair(1, "bench@example.com", domain.RoleCustomer)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.ValidateToken(tokenPair.AccessToken)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Table-driven тест для ValidateToken с разными токенами
func TestJWTService_ValidateToken_TableDriven(t *testing.T) {
	svc := NewJWTService("test-secret", 15*time.Minute, 168*time.Hour)

	// Генерируем валидный токен
	validTokenPair, _, err := svc.GenerateTokenPair(1, "test@example.com", domain.RoleCustomer)
	require.NoError(t, err)

	// Создаем токен с истекшим сроком
	expiredSvc := NewJWTService("test-secret", -1*time.Minute, 168*time.Hour)
	expiredTokenPair, _, err := expiredSvc.GenerateTokenPair(1, "test@example.com", domain.RoleCustomer)
	require.NoError(t, err)

	tests := []struct {
		name        string
		token       string
		expectError bool
		errContains string
	}{
		{
			name:        "valid token",
			token:       validTokenPair.AccessToken,
			expectError: false,
			errContains: "",
		},
		{
			name:        "empty token",
			token:       "",
			expectError: true,
			errContains: "failed to parse token",
		},
		{
			name:        "malformed token",
			token:       "not-a-jwt-token",
			expectError: true,
			errContains: "failed to parse token",
		},
		{
			name:        "expired token",
			token:       expiredTokenPair.AccessToken,
			expectError: true,
			errContains: "failed to parse token",
		},
		{
			name:        "invalid signature",
			token:       validTokenPair.AccessToken + "invalid",
			expectError: true,
			errContains: "failed to parse token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := svc.ValidateToken(tt.token)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, claims)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
			}
		})
	}
}
