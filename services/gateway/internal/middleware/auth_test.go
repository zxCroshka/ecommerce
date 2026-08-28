package middleware

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type fakeTokenValidator struct {
	identity *domain.Identity
	err      error
	called   bool
	token    string
	ctx      context.Context
}

func (f *fakeTokenValidator) ValidateToken(ctx context.Context, token string) (*domain.Identity, error) {
	f.called = true
	f.token = token
	f.ctx = ctx
	return f.identity, f.err
}

func TestAuthRequiredSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator := &fakeTokenValidator{identity: &domain.Identity{UserID: 42, Role: "admin"}}
	middleware := NewAuthMiddleware(testLogger(), validator)
	router := gin.New()
	router.Use(middleware.AuthRequired())

	var principal gatewayauth.Principal
	var principalFound bool
	router.GET("/protected", func(ctx *gin.Context) {
		principal, principalFound = gatewayauth.PrincipalFromContext(ctx)
		ctx.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "bEaReR    access-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, validator.called)
	require.Equal(t, "access-token", validator.token)
	require.Equal(t, request.Context(), validator.ctx)
	require.True(t, principalFound)
	require.Equal(t, int64(42), principal.Identity.UserID)
	require.Equal(t, "admin", principal.Identity.Role)
	require.Equal(t, "access-token", principal.AccessToken)
}

func TestAuthRequiredRejectsInvalidAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		header string
	}{
		{name: "missing header"},
		{name: "bearer without token", header: "Bearer"},
		{name: "wrong scheme", header: "Basic token"},
		{name: "too many fields", header: "Bearer token extra"},
		{name: "only whitespace", header: "   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := &fakeTokenValidator{identity: &domain.Identity{UserID: 42, Role: "user"}}
			middleware := NewAuthMiddleware(testLogger(), validator)
			downstreamCalled := false
			router := gin.New()
			router.Use(middleware.AuthRequired())
			router.GET("/protected", func(ctx *gin.Context) {
				downstreamCalled = true
				ctx.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.header != "" {
				request.Header.Set("Authorization", tt.header)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.JSONEq(t, `{
				"success": false,
				"error": {
					"code": "UNAUTHENTICATED",
					"message": "authentication required"
				}
			}`, recorder.Body.String())
			require.False(t, validator.called)
			require.False(t, downstreamCalled)
		})
	}
}

func TestAuthRequiredHandlesValidatorFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		validator TokenValidator
		status    int
		code      string
	}{
		{
			name: "invalid token",
			validator: &fakeTokenValidator{
				err: fmt.Errorf("validate token: %w", customerrors.ErrUnauthenticated),
			},
			status: http.StatusUnauthorized,
			code:   "UNAUTHENTICATED",
		},
		{
			name: "user service unavailable",
			validator: &fakeTokenValidator{
				err: fmt.Errorf("validate token: %w", customerrors.ErrServiceUnavailable),
			},
			status: http.StatusServiceUnavailable,
			code:   "SERVICE_UNAVAILABLE",
		},
		{
			name:      "validator missing",
			validator: nil,
			status:    http.StatusInternalServerError,
			code:      "INTERNAL_ERROR",
		},
		{
			name:      "nil identity",
			validator: &fakeTokenValidator{},
			status:    http.StatusInternalServerError,
			code:      "INTERNAL_ERROR",
		},
		{
			name: "invalid identity",
			validator: &fakeTokenValidator{
				identity: &domain.Identity{UserID: 0, Role: "user"},
			},
			status: http.StatusInternalServerError,
			code:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewAuthMiddleware(testLogger(), tt.validator)
			downstreamCalled := false
			router := gin.New()
			router.Use(middleware.AuthRequired())
			router.GET("/protected", func(ctx *gin.Context) {
				downstreamCalled = true
				ctx.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer access-token")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, tt.status, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+tt.code+`"`)
			require.False(t, downstreamCalled)
		})
	}
}

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		principal        *gatewayauth.Principal
		allowedRoles     []string
		expectedStatus   int
		downstreamCalled bool
	}{
		{
			name: "role allowed",
			principal: &gatewayauth.Principal{
				Identity: domain.Identity{UserID: 42, Role: "admin"},
			},
			allowedRoles:     []string{"admin"},
			expectedStatus:   http.StatusNoContent,
			downstreamCalled: true,
		},
		{
			name: "one of multiple roles allowed",
			principal: &gatewayauth.Principal{
				Identity: domain.Identity{UserID: 42, Role: "USER"},
			},
			allowedRoles:     []string{"admin", "user"},
			expectedStatus:   http.StatusNoContent,
			downstreamCalled: true,
		},
		{
			name: "role forbidden",
			principal: &gatewayauth.Principal{
				Identity: domain.Identity{UserID: 42, Role: "user"},
			},
			allowedRoles:   []string{"admin"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "principal missing",
			allowedRoles:   []string{"admin"},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "allowed roles missing",
			principal: &gatewayauth.Principal{
				Identity: domain.Identity{UserID: 42, Role: "admin"},
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewAuthMiddleware(testLogger(), nil)
			downstreamCalled := false
			router := gin.New()
			if tt.principal != nil {
				router.Use(func(ctx *gin.Context) {
					gatewayauth.SetPrincipal(ctx, *tt.principal)
					ctx.Next()
				})
			}
			router.Use(middleware.RequireRole(tt.allowedRoles...))
			router.GET("/protected", func(ctx *gin.Context) {
				downstreamCalled = true
				ctx.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, tt.expectedStatus, recorder.Code)
			require.Equal(t, tt.downstreamCalled, downstreamCalled)
		})
	}
}

func TestAuthRequiredAndRequireRoleChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator := &fakeTokenValidator{identity: &domain.Identity{UserID: 42, Role: "admin"}}
	middleware := NewAuthMiddleware(testLogger(), validator)
	router := gin.New()
	router.Use(middleware.AuthRequired(), middleware.RequireRole("admin"))
	router.GET("/admin", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
