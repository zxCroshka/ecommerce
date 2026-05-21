package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	jwt5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service/pwdgen"
)

// ========== HELPER FUNCTIONS ==========

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

// testHash генерирует хеш для тестового пароля
func testHash(t *testing.T, password string) []byte {
	hash := pwdgen.Generate([]byte(password))
	return hash
}

// ========== MOCKS ==========

type MockUserManager struct {
	mock.Mock
}

func (m *MockUserManager) RegisterUserTX(ctx context.Context, email string, passHash []byte, name string, isAdmin bool) (int64, error) {
	args := m.Called(ctx, email, passHash, name, isAdmin)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockUserManager) User(ctx context.Context, email string) (domain.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(domain.User), args.Error(1)
}

func (m *MockUserManager) UserByID(ctx context.Context, userID int64) (domain.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.User), args.Error(1)
}

func (m *MockUserManager) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserManager) UpdateName(ctx context.Context, userID int64, newName string) error {
	args := m.Called(ctx, userID, newName)
	return args.Error(0)
}

func (m *MockUserManager) UpdateEmail(ctx context.Context, userID int64, newEmail string) error {
	args := m.Called(ctx, userID, newEmail)
	return args.Error(0)
}

func (m *MockUserManager) UpdatePassword(ctx context.Context, userId int64, newPassHash []byte) error {
	args := m.Called(ctx, userId, newPassHash)
	return args.Error(0)
}

type MockTokenManager struct {
	mock.Mock
}

func (m *MockTokenManager) SaveRefreshToken(ctx context.Context, userID int64, tokenID string, ttl time.Duration) error {
	args := m.Called(ctx, userID, tokenID, ttl)
	return args.Error(0)
}

func (m *MockTokenManager) ValidateRefreshToken(ctx context.Context, userID int64, tokenID string) (bool, error) {
	args := m.Called(ctx, userID, tokenID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTokenManager) DeleteRefreshToken(ctx context.Context, userID int64, tokenID string) error {
	args := m.Called(ctx, userID, tokenID)
	return args.Error(0)
}

func (m *MockTokenManager) AddToBlacklist(ctx context.Context, tokenID string, ttl time.Duration) error {
	args := m.Called(ctx, tokenID, ttl)
	return args.Error(0)
}

func (m *MockTokenManager) IsBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	args := m.Called(ctx, tokenID)
	return args.Bool(0), args.Error(1)
}

type MockProducer struct {
	mock.Mock
}

func (m *MockProducer) Close() {
	m.Called()
}

func (m *MockProducer) Produce(userID int64, email string, name string) error {
	args := m.Called(userID, email, name)
	return args.Error(0)
}

type MockJWTManager struct {
	mock.Mock
}

func (m *MockJWTManager) GenerateTokenPair(userID int64, email string, isAdmin bool) (*jwt.TokenPair, string, error) {
	args := m.Called(userID, email, isAdmin)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*jwt.TokenPair), args.String(1), args.Error(2)
}

func (m *MockJWTManager) ValidateToken(tokenString string) (*jwt.TokenClaims, error) {
	args := m.Called(tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenClaims), args.Error(1)
}

func (m *MockJWTManager) GetRefreshTTL() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

// ========== TESTS ==========

func TestUserService_Register(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		password    string
		nameInput   string
		isAdmin     bool
		setupMock   func(*MockUserManager, *MockProducer)
		expectedErr error
	}{
		{
			name:      "success - register customer",
			email:     "test@example.com",
			password:  "password123",
			nameInput: "Test User",
			isAdmin:   false,
			setupMock: func(um *MockUserManager, p *MockProducer) {
				um.On("RegisterUserTX", mock.Anything, "test@example.com", mock.Anything, "Test User", false).
					Return(int64(1), nil)
				// ✅ Используем Maybe() для горутины
				p.On("Produce", int64(1), "test@example.com", "Test User").
					Return(nil).Maybe()
			},
			expectedErr: nil,
		},
		{
			name:      "success - register admin",
			email:     "admin@example.com",
			password:  "password123",
			nameInput: "Admin User",
			isAdmin:   true,
			setupMock: func(um *MockUserManager, p *MockProducer) {
				um.On("RegisterUserTX", mock.Anything, "admin@example.com", mock.Anything, "Admin User", true).
					Return(int64(2), nil)
				// ✅ Используем Maybe() для горутины
				p.On("Produce", int64(2), "admin@example.com", "Admin User").
					Return(nil).Maybe()
			},
			expectedErr: nil,
		},
		{
			name:      "error - duplicate email",
			email:     "existing@example.com",
			password:  "password123",
			nameInput: "Test User",
			isAdmin:   false,
			setupMock: func(um *MockUserManager, p *MockProducer) {
				um.On("RegisterUserTX", mock.Anything, "existing@example.com", mock.Anything, "Test User", false).
					Return(int64(0), customerrors.ErrDuplicateEmail)
			},
			expectedErr: customerrors.ErrDuplicateEmail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockUM, mockProd)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			err := svc.Register(context.Background(), tt.email, tt.password, tt.nameInput, tt.isAdmin)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			} else {
				assert.NoError(t, err)
			}

			mockUM.AssertExpectations(t)
			mockProd.AssertExpectations(t)
		})
	}
}

func TestUserService_Login(t *testing.T) {
	// ✅ Генерируем хеш для пароля "password123"
	hashedPassword := testHash(t, "password123")

	tests := []struct {
		name        string
		email       string
		password    string
		setupMock   func(*MockUserManager, *MockJWTManager, *MockTokenManager)
		expectedErr error
	}{
		{
			name:     "success - valid credentials",
			email:    "test@example.com",
			password: "password123",
			setupMock: func(um *MockUserManager, jm *MockJWTManager, tm *MockTokenManager) {
				um.On("User", mock.Anything, "test@example.com").
					Return(domain.User{
						Id:        1,
						Email:     "test@example.com",
						PassHash:  hashedPassword,
						Name:      "Test User",
						IsAdmin:   false,
						CreatedAt: time.Now(),
					}, nil)
				jm.On("GenerateTokenPair", int64(1), "test@example.com", false).
					Return(&jwt.TokenPair{AccessToken: "access-token", RefreshToken: "refresh-token"}, "refresh-id", nil)
				jm.On("GetRefreshTTL").Return(168 * time.Hour)
				tm.On("SaveRefreshToken", mock.Anything, int64(1), "refresh-id", 168*time.Hour).
					Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:     "error - user not found",
			email:    "notfound@example.com",
			password: "password123",
			setupMock: func(um *MockUserManager, jm *MockJWTManager, tm *MockTokenManager) {
				um.On("User", mock.Anything, "notfound@example.com").
					Return(domain.User{}, customerrors.ErrUserNotFound)
			},
			expectedErr: customerrors.ErrInvalidCredentials,
		},
		{
			name:     "error - invalid password",
			email:    "test@example.com",
			password: "wrongpassword",
			setupMock: func(um *MockUserManager, jm *MockJWTManager, tm *MockTokenManager) {
				um.On("User", mock.Anything, "test@example.com").
					Return(domain.User{
						Id:       1,
						Email:    "test@example.com",
						PassHash: hashedPassword,
						Name:     "Test User",
						IsAdmin:  false,
					}, nil)
			},
			expectedErr: customerrors.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockUM, mockJWT, mockTM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			_, err := svc.Login(context.Background(), tt.email, tt.password)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			} else {
				assert.NoError(t, err)
			}

			mockUM.AssertExpectations(t)
			mockJWT.AssertExpectations(t)
			mockTM.AssertExpectations(t)
		})
	}
}

func TestUserService_RefreshTokens(t *testing.T) {
	claims := &jwt.TokenClaims{
		UserID:  1,
		Email:   "test@example.com",
		IsAdmin: false,
		RegisteredClaims: jwt5.RegisteredClaims{
			ID: "old-refresh-id",
		},
	}

	tests := []struct {
		name         string
		refreshToken string
		setupMock    func(*MockJWTManager, *MockTokenManager)
		expectedErr  error
	}{
		{
			name:         "success - refresh tokens",
			refreshToken: "valid-refresh-token",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "valid-refresh-token").
					Return(claims, nil)
				tm.On("ValidateRefreshToken", mock.Anything, int64(1), "old-refresh-id").
					Return(true, nil)
				tm.On("DeleteRefreshToken", mock.Anything, int64(1), "old-refresh-id").
					Return(nil)
				jm.On("GenerateTokenPair", int64(1), "test@example.com", false).
					Return(&jwt.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}, "new-refresh-id", nil)
				jm.On("GetRefreshTTL").Return(168 * time.Hour)
				tm.On("SaveRefreshToken", mock.Anything, int64(1), "new-refresh-id", 168*time.Hour).
					Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:         "error - invalid refresh token",
			refreshToken: "invalid-token",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "invalid-token").
					Return(nil, customerrors.ErrInvalidToken)
			},
			expectedErr: customerrors.ErrInvalidToken,
		},
		{
			name:         "error - token not found in redis",
			refreshToken: "valid-refresh-token",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "valid-refresh-token").
					Return(claims, nil)
				tm.On("ValidateRefreshToken", mock.Anything, int64(1), "old-refresh-id").
					Return(false, nil)
			},
			expectedErr: errors.New(""), // Любая ошибка
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockJWT, mockTM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			_, err := svc.RefreshTokens(context.Background(), tt.refreshToken)

			if tt.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockJWT.AssertExpectations(t)
			mockTM.AssertExpectations(t)
		})
	}
}

func TestUserService_Logout(t *testing.T) {
	accessClaims := &jwt.TokenClaims{
		RegisteredClaims: jwt5.RegisteredClaims{
			ID:        "access-id",
			ExpiresAt: jwt5.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	refreshClaims := &jwt.TokenClaims{
		UserID: 1,
		RegisteredClaims: jwt5.RegisteredClaims{
			ID: "refresh-id",
		},
	}

	tests := []struct {
		name         string
		accessToken  string
		refreshToken string
		setupMock    func(*MockJWTManager, *MockTokenManager)
		expectedErr  error
	}{
		{
			name:         "success - logout",
			accessToken:  "valid-access",
			refreshToken: "valid-refresh",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "valid-access").
					Return(accessClaims, nil)
				jm.On("ValidateToken", "valid-refresh").
					Return(refreshClaims, nil)
				tm.On("AddToBlacklist", mock.Anything, "access-id", mock.Anything).
					Return(nil)
				tm.On("DeleteRefreshToken", mock.Anything, int64(1), "refresh-id").
					Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:         "error - invalid access token",
			accessToken:  "invalid-access",
			refreshToken: "valid-refresh",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "invalid-access").
					Return(nil, customerrors.ErrInvalidToken)
			},
			expectedErr: customerrors.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockJWT, mockTM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			err := svc.Logout(context.Background(), tt.accessToken, tt.refreshToken)

			if tt.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockJWT.AssertExpectations(t)
			mockTM.AssertExpectations(t)
		})
	}
}

func TestUserService_ValidateToken(t *testing.T) {
	claims := &jwt.TokenClaims{
		UserID:  1,
		IsAdmin: false,
		RegisteredClaims: jwt5.RegisteredClaims{
			ID: "token-id",
		},
	}

	tests := []struct {
		name            string
		token           string
		setupMock       func(*MockJWTManager, *MockTokenManager)
		expectedUserID  int64
		expectedIsAdmin bool
		expectedErr     error
	}{
		{
			name:  "success - valid token",
			token: "valid-token",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				tm.On("IsBlacklisted", mock.Anything, "token-id").
					Return(false, nil)
			},
			expectedUserID:  1,
			expectedIsAdmin: false,
			expectedErr:     nil,
		},
		{
			name:  "error - blacklisted token",
			token: "blacklisted-token",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "blacklisted-token").
					Return(claims, nil)
				tm.On("IsBlacklisted", mock.Anything, "token-id").
					Return(true, nil)
			},
			expectedErr: customerrors.ErrTokenBlacklisted,
		},
		{
			name:  "error - invalid token",
			token: "invalid-token",
			setupMock: func(jm *MockJWTManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "invalid-token").
					Return(nil, customerrors.ErrInvalidToken)
			},
			expectedErr: customerrors.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockJWT, mockTM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			userID, isAdmin, err := svc.ValidateToken(context.Background(), tt.token)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUserID, userID)
				assert.Equal(t, tt.expectedIsAdmin, isAdmin)
			}

			mockJWT.AssertExpectations(t)
			mockTM.AssertExpectations(t)
		})
	}
}

func TestUserService_GetUser(t *testing.T) {
	expectedUser := domain.User{
		Id:      1,
		Email:   "test@example.com",
		Name:    "Test User",
		IsAdmin: false,
	}

	tests := []struct {
		name        string
		userID      int64
		setupMock   func(*MockUserManager)
		expectedErr error
	}{
		{
			name:   "success - get user",
			userID: 1,
			setupMock: func(um *MockUserManager) {
				um.On("UserByID", mock.Anything, int64(1)).
					Return(expectedUser, nil)
			},
			expectedErr: nil,
		},
		{
			name:   "error - user not found",
			userID: 999,
			setupMock: func(um *MockUserManager) {
				um.On("UserByID", mock.Anything, int64(999)).
					Return(domain.User{}, customerrors.ErrUserNotFound)
			},
			expectedErr: customerrors.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockUM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			user, err := svc.GetUser(context.Background(), tt.userID)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedUser, user)
			}

			mockUM.AssertExpectations(t)
		})
	}
}

func TestUserService_UpdateEmail(t *testing.T) {
	claims := &jwt.TokenClaims{
		UserID: 1,
		Email:  "old@example.com",
	}

	tests := []struct {
		name        string
		token       string
		newEmail    string
		setupMock   func(*MockJWTManager, *MockUserManager)
		expectedErr error
	}{
		{
			name:     "success - update email",
			token:    "valid-token",
			newEmail: "new@example.com",
			setupMock: func(jm *MockJWTManager, um *MockUserManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				um.On("User", mock.Anything, "new@example.com").
					Return(domain.User{Id: 0}, customerrors.ErrUserNotFound)
				um.On("UpdateEmail", mock.Anything, int64(1), "new@example.com").
					Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:     "error - duplicate email",
			token:    "valid-token",
			newEmail: "existing@example.com",
			setupMock: func(jm *MockJWTManager, um *MockUserManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				um.On("User", mock.Anything, "existing@example.com").
					Return(domain.User{Id: 2}, nil)
			},
			expectedErr: customerrors.ErrDuplicateEmail,
		},
		{
			name:     "skip - same email",
			token:    "valid-token",
			newEmail: "old@example.com",
			setupMock: func(jm *MockJWTManager, um *MockUserManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				um.On("User", mock.Anything, "old@example.com").
					Return(domain.User{Id: 1}, nil)
			},
			expectedErr: nil,
		},
		{
			name:     "error - invalid token",
			token:    "invalid-token",
			newEmail: "new@example.com",
			setupMock: func(jm *MockJWTManager, um *MockUserManager) {
				jm.On("ValidateToken", "invalid-token").
					Return(nil, customerrors.ErrInvalidToken)
			},
			expectedErr: customerrors.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockJWT, mockUM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			err := svc.UpdateEmail(context.Background(), tt.token, tt.newEmail)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			} else {
				assert.NoError(t, err)
			}

			mockJWT.AssertExpectations(t)
			mockUM.AssertExpectations(t)
		})
	}
}

func TestUserService_UpdateName(t *testing.T) {
	claims := &jwt.TokenClaims{
		UserID: 1,
	}

	tests := []struct {
		name        string
		token       string
		newName     string
		setupMock   func(*MockJWTManager, *MockUserManager)
		expectedErr error
	}{
		{
			name:    "success - update name",
			token:   "valid-token",
			newName: "New Name",
			setupMock: func(jm *MockJWTManager, um *MockUserManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				um.On("UpdateName", mock.Anything, int64(1), "New Name").
					Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:    "error - invalid token",
			token:   "invalid-token",
			newName: "New Name",
			setupMock: func(jm *MockJWTManager, um *MockUserManager) {
				jm.On("ValidateToken", "invalid-token").
					Return(nil, customerrors.ErrInvalidToken)
			},
			expectedErr: customerrors.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockJWT, mockUM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			err := svc.UpdateName(context.Background(), tt.token, tt.newName)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			} else {
				assert.NoError(t, err)
			}

			mockJWT.AssertExpectations(t)
			mockUM.AssertExpectations(t)
		})
	}
}

func TestUserService_UpdatePassword(t *testing.T) {
	// ✅ Генерируем хеш для пароля "oldpass123"
	hashedOldPassword := testHash(t, "oldpass123")

	claims := &jwt.TokenClaims{
		UserID: 1,
		Email:  "test@example.com",
	}

	tests := []struct {
		name        string
		token       string
		oldPassword string
		newPassword string
		setupMock   func(*MockJWTManager, *MockUserManager, *MockTokenManager)
		expectedErr error
	}{
		{
			name:        "success - update password",
			token:       "valid-token",
			oldPassword: "oldpass123",
			newPassword: "newpass123",
			setupMock: func(jm *MockJWTManager, um *MockUserManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				um.On("User", mock.Anything, "test@example.com").
					Return(domain.User{
						Id:       1,
						Email:    "test@example.com",
						PassHash: hashedOldPassword,
						Name:     "Test User",
					}, nil)
				um.On("UpdatePassword", mock.Anything, int64(1), mock.Anything).
					Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:        "error - same password",
			token:       "valid-token",
			oldPassword: "password123",
			newPassword: "password123",
			setupMock: func(jm *MockJWTManager, um *MockUserManager, tm *MockTokenManager) {
				// No mocks needed - validation fails before
			},
			expectedErr: customerrors.ErrSamePassword,
		},
		{
			name:        "error - invalid old password",
			token:       "valid-token",
			oldPassword: "wrongpass",
			newPassword: "newpass123",
			setupMock: func(jm *MockJWTManager, um *MockUserManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				um.On("User", mock.Anything, "test@example.com").
					Return(domain.User{
						Id:       1,
						Email:    "test@example.com",
						PassHash: hashedOldPassword,
					}, nil)
			},
			expectedErr: customerrors.ErrInvalidCredentials,
		},
		{
			name:        "error - user not found",
			token:       "valid-token",
			oldPassword: "oldpass123",
			newPassword: "newpass123",
			setupMock: func(jm *MockJWTManager, um *MockUserManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "valid-token").
					Return(claims, nil)
				um.On("User", mock.Anything, "test@example.com").
					Return(domain.User{}, customerrors.ErrUserNotFound)
			},
			expectedErr: customerrors.ErrUserNotFound,
		},
		{
			name:        "error - invalid token",
			token:       "invalid-token",
			oldPassword: "oldpass123",
			newPassword: "newpass123",
			setupMock: func(jm *MockJWTManager, um *MockUserManager, tm *MockTokenManager) {
				jm.On("ValidateToken", "invalid-token").
					Return(nil, customerrors.ErrInvalidToken)
			},
			expectedErr: customerrors.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUM := new(MockUserManager)
			mockTM := new(MockTokenManager)
			mockProd := new(MockProducer)
			mockJWT := new(MockJWTManager)

			tt.setupMock(mockJWT, mockUM, mockTM)

			svc := NewUserService(testLogger(), mockUM, mockTM, mockProd, mockJWT)
			err := svc.UpdatePassword(context.Background(), tt.token, tt.oldPassword, tt.newPassword)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tt.expectedErr))
			} else {
				assert.NoError(t, err)
			}

			mockJWT.AssertExpectations(t)
			mockUM.AssertExpectations(t)
		})
	}
}
