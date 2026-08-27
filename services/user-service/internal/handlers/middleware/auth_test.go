package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
)

type MockUserServiceForAuth struct {
	mock.Mock
}

func (m *MockUserServiceForAuth) Register(ctx context.Context, email, password, name string) error {
	args := m.Called(ctx, email, password, name)
	return args.Error(0)
}

func (m *MockUserServiceForAuth) Login(ctx context.Context, email, password string) (*jwt.TokenPair, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenPair), args.Error(1)
}

func (m *MockUserServiceForAuth) RefreshTokens(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenPair), args.Error(1)
}

func (m *MockUserServiceForAuth) Logout(ctx context.Context, identity domain.Identity, refreshToken string) error {
	args := m.Called(ctx, identity, refreshToken)
	return args.Error(0)
}

func (m *MockUserServiceForAuth) UpdateEmail(ctx context.Context, userID int64, newEmail string) error {
	args := m.Called(ctx, userID, newEmail)
	return args.Error(0)
}

func (m *MockUserServiceForAuth) UpdateName(ctx context.Context, userID int64, newName string) error {
	args := m.Called(ctx, userID, newName)
	return args.Error(0)
}

func (m *MockUserServiceForAuth) UpdatePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	args := m.Called(ctx, userID, oldPassword, newPassword)
	return args.Error(0)
}

func (m *MockUserServiceForAuth) GetUser(ctx context.Context, userID int64) (domain.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.User), args.Error(1)
}

func (m *MockUserServiceForAuth) ValidateToken(ctx context.Context, token string) (domain.Identity, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(domain.Identity), args.Error(1)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	errorMiddleware := NewErrorMiddleware()
	router.Use(errorMiddleware.ErrorHandler())
	return router
}

func TestAuthMiddleware_AuthRequired(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		setupMock      func(*MockUserServiceForAuth)
		expectedStatus int
		checkContext   func(*testing.T, *gin.Context)
	}{
		{
			name:       "success - valid token",
			authHeader: "Bearer valid-token-123",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "valid-token-123").
					Return(domain.Identity{UserID: 123, Role: domain.RoleAdmin}, nil)
			},
			expectedStatus: http.StatusOK,
			checkContext: func(t *testing.T, c *gin.Context) {
				userID, exists := c.Get("user_id")
				assert.True(t, exists)
				assert.Equal(t, int64(123), userID)

				role, exists := c.Get("role")
				assert.True(t, exists)
				assert.Equal(t, domain.RoleAdmin, role)

				identity, exists := c.Get("identity")
				assert.True(t, exists)
				assert.Equal(t, int64(123), identity.(domain.Identity).UserID)
			},
		},
		{
			name:       "success - valid token with customer role",
			authHeader: "Bearer customer-token",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "customer-token").
					Return(domain.Identity{UserID: 456, Role: domain.RoleCustomer}, nil)
			},
			expectedStatus: http.StatusOK,
			checkContext: func(t *testing.T, c *gin.Context) {
				userID, _ := c.Get("user_id")
				assert.Equal(t, int64(456), userID)
				role, _ := c.Get("role")
				assert.Equal(t, domain.RoleCustomer, role)
			},
		},
		{
			name:           "error - missing authorization header",
			authHeader:     "",
			setupMock:      func(m *MockUserServiceForAuth) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "error - invalid header format (no Bearer)",
			authHeader:     "Token valid-token",
			setupMock:      func(m *MockUserServiceForAuth) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "error - invalid header format (only token)",
			authHeader:     "valid-token",
			setupMock:      func(m *MockUserServiceForAuth) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "error - invalid token",
			authHeader: "Bearer invalid-token",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "invalid-token").
					Return(domain.Identity{}, customerrors.ErrInvalidToken)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "error - blacklisted token",
			authHeader: "Bearer blacklisted-token",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "blacklisted-token").
					Return(domain.Identity{}, customerrors.ErrTokenBlacklisted)
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:       "error - service error",
			authHeader: "Bearer token",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "token").
					Return(domain.Identity{}, assert.AnError)
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserServiceForAuth)
			tt.setupMock(mockService)

			authMiddleware := NewAuthMiddleware(mockService)

			router := setupTestRouter()
			router.Use(authMiddleware.AuthRequired())
			router.GET("/test", func(c *gin.Context) {
				if tt.checkContext != nil {
					tt.checkContext(t, c)
				}
				c.JSON(http.StatusOK, gin.H{"message": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func TestAuthMiddleware_ContextPropagation(t *testing.T) {
	mockService := new(MockUserServiceForAuth)
	mockService.On("ValidateToken", mock.Anything, "test-token").
		Return(domain.Identity{UserID: 999, Role: domain.RoleAdmin}, nil)

	authMiddleware := NewAuthMiddleware(mockService)

	router := setupTestRouter()
	router.Use(authMiddleware.AuthRequired())
	router.GET("/test", func(c *gin.Context) {
		userID, ok := c.Get("user_id")
		assert.True(t, ok)
		assert.Equal(t, int64(999), userID)

		role, ok := c.Get("role")
		assert.True(t, ok)
		assert.Equal(t, domain.RoleAdmin, role)

		identity, ok := c.Get("identity")
		assert.True(t, ok)
		assert.Equal(t, int64(999), identity.(domain.Identity).UserID)

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestAuthMiddleware_MultipleMiddlewareChain(t *testing.T) {
	mockService := new(MockUserServiceForAuth)
	mockService.On("ValidateToken", mock.Anything, "valid-token").
		Return(domain.Identity{UserID: 123, Role: domain.RoleCustomer}, nil)

	authMiddleware := NewAuthMiddleware(mockService)

	router := setupTestRouter()

	router.Use(func(c *gin.Context) {
		c.Set("request_id", "req-123")
		c.Next()
	})
	router.Use(authMiddleware.AuthRequired())
	router.Use(func(c *gin.Context) {
		requestID, _ := c.Get("request_id")
		userID, _ := c.Get("user_id")
		assert.Equal(t, "req-123", requestID)
		assert.Equal(t, int64(123), userID)
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestAuthMiddleware_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		setupMock  func(*MockUserServiceForAuth)
	}{
		{
			name:       "empty Bearer",
			authHeader: "Bearer ",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "").
					Return(domain.Identity{}, customerrors.ErrInvalidToken)
			},
		},
		{
			name:       "Bearer with spaces",
			authHeader: "Bearer   token-with-spaces  ",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "token-with-spaces").
					Return(domain.Identity{UserID: 123, Role: domain.RoleCustomer}, nil)
			},
		},
		{
			name:       "lowercase bearer",
			authHeader: "bearer lower-case-token",
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, "lower-case-token").
					Return(domain.Identity{UserID: 123, Role: domain.RoleCustomer}, nil)
			},
		},
		{
			name:       "very long token",
			authHeader: "Bearer " + string(make([]byte, 10000)),
			setupMock: func(m *MockUserServiceForAuth) {
				m.On("ValidateToken", mock.Anything, mock.Anything).
					Return(domain.Identity{UserID: 123, Role: domain.RoleCustomer}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserServiceForAuth)
			tt.setupMock(mockService)

			authMiddleware := NewAuthMiddleware(mockService)

			router := setupTestRouter()
			router.Use(authMiddleware.AuthRequired())
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.NotEqual(t, http.StatusNotFound, w.Code)
			mockService.AssertExpectations(t)
		})
	}
}

func BenchmarkAuthMiddleware_ValidToken(b *testing.B) {
	mockService := new(MockUserServiceForAuth)
	mockService.On("ValidateToken", mock.Anything, "bench-token").
		Return(domain.Identity{UserID: 123, Role: domain.RoleCustomer}, nil)

	authMiddleware := NewAuthMiddleware(mockService)

	router := setupTestRouter()
	router.Use(authMiddleware.AuthRequired())
	router.GET("/bench", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/bench", nil)
		req.Header.Set("Authorization", "Bearer bench-token")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
