package logging

import (
	"bytes"
	"errors"
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

func TestWriteLog(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		err      error
		wantText []string
	}{
		{
			name: "client error is warning",
			err:  customerrors.ErrInvalidArgument,
			wantText: []string{
				"level=WARN", "request rejected", "http_status=400", "error_code=INVALID_ARGUMENT",
			},
		},
		{
			name: "server error is error",
			err:  errors.New("database failed"),
			wantText: []string{
				"level=ERROR", "request failed", "http_status=500", "error_code=INTERNAL_ERROR",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&output, nil))
			request := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = request
			gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
				Identity: domain.Identity{UserID: 42, Role: "user"},
			})

			WriteLog(ctx, logger, tt.err)

			for _, expected := range tt.wantText {
				require.Contains(t, output.String(), expected)
			}
			require.Contains(t, output.String(), "method=GET")
			require.Contains(t, output.String(), "path=/api/v1/test")
			require.Contains(t, output.String(), "user_id=42")
		})
	}
}

func TestWriteLogIgnoresNilError(t *testing.T) {
	var output bytes.Buffer
	WriteLog(nil, slog.New(slog.NewTextHandler(&output, nil)), nil)
	require.Empty(t, output.String())
}
