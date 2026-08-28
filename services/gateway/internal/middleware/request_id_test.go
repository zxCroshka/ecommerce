package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		incomingID string
		preserved  bool
	}{
		{name: "generates missing id"},
		{name: "preserves valid id", incomingID: "request-123", preserved: true},
		{name: "replaces invalid id", incomingID: "invalid request id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(RequestID())
			var contextID string
			router.GET("/test", func(ctx *gin.Context) {
				contextID, _ = RequestIDFromContext(ctx)
				ctx.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.incomingID != "" {
				request.Header.Set(RequestIDHeader, tt.incomingID)
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			responseID := recorder.Header().Get(RequestIDHeader)
			require.NotEmpty(t, responseID)
			require.Equal(t, responseID, contextID)
			if tt.preserved {
				require.Equal(t, tt.incomingID, responseID)
			} else {
				require.NoError(t, uuid.Validate(responseID))
			}
		})
	}
}
