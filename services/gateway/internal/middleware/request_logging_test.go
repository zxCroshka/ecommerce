package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

func TestRequestLogging(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	router := gin.New()
	router.Use(RequestID(), RequestLogging(logger))
	router.GET("/users/:id", func(ctx *gin.Context) {
		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity: domain.Identity{UserID: 42, Role: "user"},
		})
		ctx.Status(http.StatusCreated)
	})

	request := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	request.Header.Set(RequestIDHeader, "request-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	logOutput := output.String()
	require.Contains(t, logOutput, "level=INFO")
	require.Contains(t, logOutput, "request_id=request-123")
	require.Contains(t, logOutput, "method=GET")
	require.Contains(t, logOutput, "path=/users/42")
	require.Contains(t, logOutput, "route=/users/:id")
	require.Contains(t, logOutput, "status=201")
	require.Contains(t, logOutput, "user_id=42")
	require.NotContains(t, logOutput, "Authorization")
}
