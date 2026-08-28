package logging

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/response"
)

func WriteLog(ctx *gin.Context, log *slog.Logger, err error) {
	if err == nil {
		return
	}
	if log == nil {
		log = slog.Default()
	}

	apiErr := response.MapError(err)
	attributes := []any{
		"error", err,
		"http_status", apiErr.Status,
		"error_code", apiErr.Code,
	}
	if ctx != nil && ctx.Request != nil {
		attributes = append(attributes,
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
		)
		if requestID := ctx.Writer.Header().Get("X-Request-ID"); requestID != "" {
			attributes = append(attributes, "request_id", requestID)
		}
		if principal, ok := gatewayauth.PrincipalFromContext(ctx); ok {
			attributes = append(attributes, "user_id", principal.Identity.UserID)
		}
	}

	if apiErr.Status >= http.StatusInternalServerError {
		log.Error("request failed", attributes...)
		return
	}
	log.Warn("request rejected", attributes...)
}
