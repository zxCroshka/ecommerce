package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/middleware"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/mocks"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Добавляем error middleware для обработки ошибок
	errorMiddleware := middleware.NewErrorMiddleware()
	router.Use(errorMiddleware.ErrorHandler())

	return router
}

func TestAuthHandlers_Register(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success - user registered",
			requestBody: RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("Register", mock.Anything, "test@example.com", "password123", "Test User").
					Return(nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				assert.Equal(t, "user registered successfully", response["message"])
			},
		},
		{
			name: "success - registered with default name",
			requestBody: RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
				Name:     "",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("Register", mock.Anything, "test@example.com", "password123", "anonymous user").
					Return(nil)
			},
			expectedStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				assert.Equal(t, "user registered successfully", response["message"])
			},
		},
		{
			name: "error - duplicate email",
			requestBody: RegisterRequest{
				Email:    "existing@example.com",
				Password: "password123",
				Name:     "Test User",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("Register", mock.Anything, "existing@example.com", "password123", "Test User").
					Return(customerrors.ErrDuplicateEmail)
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "CONFLICT", errorObj["code"])
				assert.Equal(t, "user with this email already exists", errorObj["message"])
			},
		},
		{
			name: "error - invalid request body (missing email)",
			requestBody: map[string]string{
				"password": "password123",
			},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
		{
			name: "error - invalid request body (short password)",
			requestBody: RegisterRequest{
				Email:    "test@example.com",
				Password: "123",
				Name:     "Test",
			},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
		{
			name: "error - invalid email format",
			requestBody: RegisterRequest{
				Email:    "invalid-email",
				Password: "password123",
				Name:     "Test",
			},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(mocks.MockUserService)
			tt.setupMock(mockService)

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			handler := New(logger, mockService)

			router := setupTestRouter()
			router.POST("/register", handler.Register)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestAuthHandlers_Login(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success - valid credentials",
			requestBody: LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("Login", mock.Anything, "test@example.com", "password123").
					Return(&jwt.TokenPair{
						AccessToken:  "access-token-123",
						RefreshToken: "refresh-token-456",
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "access-token-123", data["access_token"])
				assert.Equal(t, "refresh-token-456", data["refresh_token"])
				assert.Equal(t, "Bearer", data["token_type"])
				assert.Equal(t, float64(900), data["expires_in"])
			},
		},
		{
			name: "error - invalid credentials",
			requestBody: LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("Login", mock.Anything, "test@example.com", "wrongpassword").
					Return(nil, customerrors.ErrInvalidCredentials)
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "UNAUTHORIZED", errorObj["code"])
				assert.Equal(t, "invalid email or password", errorObj["message"])
			},
		},
		{
			name: "error - missing email",
			requestBody: map[string]string{
				"password": "password123",
			},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
		{
			name: "error - missing password",
			requestBody: map[string]string{
				"email": "test@example.com",
			},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(mocks.MockUserService)
			tt.setupMock(mockService)

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			handler := New(logger, mockService)

			router := setupTestRouter()
			router.POST("/login", handler.Login)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestAuthHandlers_RefreshToken(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success - token refreshed",
			requestBody: RefreshTokenRequest{
				RefreshToken: "valid-refresh-token",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("RefreshTokens", mock.Anything, "valid-refresh-token").
					Return(&jwt.TokenPair{
						AccessToken:  "new-access-token",
						RefreshToken: "new-refresh-token",
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				data := response["data"].(map[string]interface{})
				assert.Equal(t, "new-access-token", data["access_token"])
				assert.Equal(t, "new-refresh-token", data["refresh_token"])
				assert.Equal(t, "Bearer", data["token_type"])
				assert.Equal(t, float64(900), data["expires_in"])
			},
		},
		{
			name: "error - invalid refresh token",
			requestBody: RefreshTokenRequest{
				RefreshToken: "invalid-token",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("RefreshTokens", mock.Anything, "invalid-token").
					Return(nil, customerrors.ErrInvalidToken)
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "UNAUTHORIZED", errorObj["code"])
			},
		},
		{
			name:           "error - missing refresh token",
			requestBody:    map[string]string{},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(mocks.MockUserService)
			tt.setupMock(mockService)

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			handler := New(logger, mockService)

			router := setupTestRouter()
			router.POST("/auth/refresh", handler.RefreshToken)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestAuthHandlers_Logout(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		requestBody    interface{}
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:       "success - logout",
			authHeader: "Bearer access-token-123",
			requestBody: LogoutRequest{
				RefreshToken: "refresh-token-456",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("Logout", mock.Anything, "access-token-123", "refresh-token-456").
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				assert.Equal(t, "successfully logged out", response["message"])
			},
		},
		{
			name:       "error - missing auth header",
			authHeader: "",
			requestBody: LogoutRequest{
				RefreshToken: "refresh-token",
			},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
		{
			name:           "error - missing refresh token",
			authHeader:     "Bearer access-token",
			requestBody:    map[string]string{},
			setupMock:      func(m *mocks.MockUserService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "BAD_REQUEST", errorObj["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(mocks.MockUserService)
			tt.setupMock(mockService)

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			handler := New(logger, mockService)

			router := setupTestRouter()
			router.POST("/auth/logout", handler.Logout)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.checkResponse != nil {
				tt.checkResponse(t, response)
			}

			mockService.AssertExpectations(t)
		})
	}
}
