package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	errs "github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/err"
)

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name           string
		setupRouter    func() *gin.Engine
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "handle AppError - BadRequest",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					_ = c.Error(errs.NewBadRequestError("test error"))
					c.Abort()
				})

				return router
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "BAD_REQUEST",
					"message": "test error",
				},
			},
		},
		{
			name: "handle AppError - Unauthorized",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					_ = c.Error(errs.NewUnauthorizedError("invalid token"))
					c.Abort()
				})

				return router
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "UNAUTHORIZED",
					"message": "invalid token",
				},
			},
		},
		{
			name: "handle AppError - Conflict",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					_ = c.Error(errs.NewConflictError("user already exists"))
					c.Abort()
				})

				return router
			},
			expectedStatus: http.StatusConflict,
			expectedBody: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "CONFLICT",
					"message": "user already exists",
				},
			},
		},
		{
			name: "handle AppError - InternalServer",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					_ = c.Error(errs.NewInternalServerError("database connection failed"))
					c.Abort()
				})

				return router
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "INTERNAL_ERROR",
					"message": "database connection failed",
				},
			},
		},
		{
			name: "handle unknown error",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					_ = c.Error(errors.New("something went wrong"))
					c.Abort()
				})

				return router
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "INTERNAL",
					"message": "an unexpected error occurred",
				},
			},
		},
		{
			name: "multiple errors - only last one is returned",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					_ = c.Error(errs.NewBadRequestError("first error"))
					_ = c.Error(errs.NewUnauthorizedError("second error"))
					_ = c.Error(errs.NewConflictError("third error"))
					c.Abort()
				})

				return router
			},
			expectedStatus: http.StatusConflict,
			expectedBody: map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code":    "CONFLICT",
					"message": "third error",
				},
			},
		},
		{
			name: "no errors",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"success": true})
				})

				return router
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"success": true,
			},
		},
		{
			name: "error after response written",
			setupRouter: func() *gin.Engine {
				router := gin.New()
				errorMiddleware := NewErrorMiddleware()
				router.Use(errorMiddleware.ErrorHandler())

				router.GET("/test", func(c *gin.Context) {
					c.JSON(http.StatusOK, gin.H{"partial": "response"})
					_ = c.Error(errs.NewInternalServerError("error after response"))
					c.Abort()
				})

				return router
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"partial": "response",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := tt.setupRouter()

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)

			if tt.expectedBody != nil {
				require.NoError(t, err, "Response should be valid JSON")
				assert.Equal(t, tt.expectedBody, response)
			} else {
				assert.Empty(t, w.Body.String())
			}
		})
	}
}

func TestErrorHandler_WithMiddlewareChain(t *testing.T) {
	router := gin.New()

	router.Use(gin.Recovery())

	errorMiddleware := NewErrorMiddleware()
	router.Use(errorMiddleware.ErrorHandler())

	router.GET("/panic", func(c *gin.Context) {
		panic("unexpected panic")
	})

	router.GET("/error", func(c *gin.Context) {
		_ = c.Error(errs.NewBadRequestError("validation failed"))
		c.Abort()
	})

	t.Run("panic recovered", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		w := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			router.ServeHTTP(w, req)
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("error handled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/error", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response["success"].(bool))
		errorObj := response["error"].(map[string]interface{})
		assert.Equal(t, "BAD_REQUEST", errorObj["code"])
	})
}

func TestErrorHandler_ResponseFormat(t *testing.T) {
	tests := []struct {
		name         string
		appError     *errs.AppError
		expectedCode string
		expectedMsg  string
	}{
		{
			name:         "BadRequest",
			appError:     errs.NewBadRequestError("invalid input"),
			expectedCode: "BAD_REQUEST",
			expectedMsg:  "invalid input",
		},
		{
			name:         "Unauthorized",
			appError:     errs.NewUnauthorizedError("token expired"),
			expectedCode: "UNAUTHORIZED",
			expectedMsg:  "token expired",
		},
		{
			name:         "NotFound",
			appError:     errs.NewNotFoundError("user not found"),
			expectedCode: "NOT_FOUND",
			expectedMsg:  "user not found",
		},
		{
			name:         "Conflict",
			appError:     errs.NewConflictError("email exists"),
			expectedCode: "CONFLICT",
			expectedMsg:  "email exists",
		},
		{
			name:         "InternalError",
			appError:     errs.NewInternalServerError("db error"),
			expectedCode: "INTERNAL_ERROR",
			expectedMsg:  "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			errorMiddleware := NewErrorMiddleware()
			router.Use(errorMiddleware.ErrorHandler())

			router.GET("/test", func(c *gin.Context) {
				_ = c.Error(tt.appError)
				c.Abort()
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.False(t, response["success"].(bool))

			errorObj, ok := response["error"].(map[string]interface{})
			require.True(t, ok)

			assert.Equal(t, tt.expectedCode, errorObj["code"])
			assert.Equal(t, tt.expectedMsg, errorObj["message"])
		})
	}
}

func BenchmarkErrorHandler(b *testing.B) {
	router := gin.New()
	errorMiddleware := NewErrorMiddleware()
	router.Use(errorMiddleware.ErrorHandler())

	router.GET("/test", func(c *gin.Context) {
		_ = c.Error(errs.NewBadRequestError("test error"))
		c.Abort()
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}
