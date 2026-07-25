package user

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
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/middleware"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/mocks"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
)

func setupUserTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Добавляем error middleware
	errorMiddleware := middleware.NewErrorMiddleware()
	router.Use(errorMiddleware.ErrorHandler())

	// Middleware для имитации аутентификации
	router.Use(func(c *gin.Context) {
		c.Set("access_token", "test-token")
		c.Set("user_id", int64(123))
		c.Set("role", domain.RoleCustomer)
		c.Next()
	})

	return router
}

func TestUserHandlers_UpdateEmail(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success - email updated",
			requestBody: UpdateEmailRequest{
				NewEmail: "new@example.com",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("UpdateEmail", mock.Anything, "test-token", "new@example.com").
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				assert.Equal(t, "email updated successfully", response["message"])
			},
		},
		{
			name: "error - duplicate email",
			requestBody: UpdateEmailRequest{
				NewEmail: "existing@example.com",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("UpdateEmail", mock.Anything, "test-token", "existing@example.com").
					Return(customerrors.ErrDuplicateEmail)
			},
			expectedStatus: http.StatusConflict,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "CONFLICT", errorObj["code"])
				assert.Equal(t, "email already exists", errorObj["message"])
			},
		},
		{
			name: "error - invalid email format",
			requestBody: UpdateEmailRequest{
				NewEmail: "invalid-email",
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

			router := setupUserTestRouter()
			router.PUT("/user/email", handler.UpdateEmail)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/user/email", bytes.NewBuffer(jsonBody))
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

func TestUserHandlers_UpdateName(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success - name updated",
			requestBody: UpdateNameRequest{
				NewName: "New Name",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("UpdateName", mock.Anything, "test-token", "New Name").
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				assert.Equal(t, "name updated successfully", response["message"])
			},
		},
		{
			name: "error - name too short",
			requestBody: UpdateNameRequest{
				NewName: "A",
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

			router := setupUserTestRouter()
			router.PUT("/user/name", handler.UpdateName)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/user/name", bytes.NewBuffer(jsonBody))
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

func TestUserHandlers_UpdatePassword(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success - password updated",
			requestBody: UpdatePasswordRequest{
				OldPassword: "oldpass123",
				NewPassword: "newpass123",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("UpdatePassword", mock.Anything, "test-token", "oldpass123", "newpass123").
					Return(nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				assert.Equal(t, "password updated successfully", response["message"])
			},
		},
		{
			name: "error - invalid old password",
			requestBody: UpdatePasswordRequest{
				OldPassword: "wrongpass",
				NewPassword: "newpass123",
			},
			setupMock: func(m *mocks.MockUserService) {
				m.On("UpdatePassword", mock.Anything, "test-token", "wrongpass", "newpass123").
					Return(customerrors.ErrInvalidCredentials)
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "UNAUTHORIZED", errorObj["code"])
				assert.Equal(t, "invalid old password", errorObj["message"])
			},
		},
		{
			name: "error - new password too short",
			requestBody: UpdatePasswordRequest{
				OldPassword: "oldpass123",
				NewPassword: "123",
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

			router := setupUserTestRouter()
			router.PUT("/user/password", handler.UpdatePassword)

			jsonBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPut, "/user/password", bytes.NewBuffer(jsonBody))
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

func TestUserHandlers_GetProfile(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mocks.MockUserService)
		expectedStatus int
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "success - get profile",
			setupMock: func(m *mocks.MockUserService) {
				m.On("GetUser", mock.Anything, int64(123)).
					Return(domain.User{
						Id:    123,
						Email: "test@example.com",
						Name:  "Test User",
						Role:  domain.RoleCustomer,
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.True(t, response["success"].(bool))
				data := response["data"].(map[string]interface{})
				assert.Equal(t, float64(123), data["user_id"])
				assert.Equal(t, "test@example.com", data["email"])
				assert.Equal(t, "Test User", data["name"])
				assert.Equal(t, "customer", data["role"])
			},
		},
		{
			name: "error - user not found",
			setupMock: func(m *mocks.MockUserService) {
				m.On("GetUser", mock.Anything, int64(123)).
					Return(domain.User{}, customerrors.ErrUserNotFound)
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, response map[string]interface{}) {
				assert.False(t, response["success"].(bool))
				errorObj := response["error"].(map[string]interface{})
				assert.Equal(t, "UNAUTHORIZED", errorObj["code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(mocks.MockUserService)
			tt.setupMock(mockService)

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			handler := New(logger, mockService)

			router := setupUserTestRouter()
			router.GET("/user/profile", handler.GetProfile)

			req := httptest.NewRequest(http.MethodGet, "/user/profile", nil)
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
