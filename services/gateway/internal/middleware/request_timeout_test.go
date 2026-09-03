package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestTimeoutProvidesSingleRequestBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestTimeout(100 * time.Millisecond))
	router.GET("/", func(ctx *gin.Context) {
		deadline, ok := ctx.Request.Context().Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 100*time.Millisecond)
		ctx.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
}
