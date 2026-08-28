package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
)

func RequestLogging(log *slog.Logger) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}

	return func(ctx *gin.Context) {
		startedAt := time.Now()
		ctx.Next()

		requestID, _ := RequestIDFromContext(ctx)
		route := ctx.FullPath()
		if route == "" {
			route = "unmatched"
		}
		attributes := []any{
			"request_id", requestID,
			"method", ctx.Request.Method,
			"path", ctx.Request.URL.Path,
			"route", route,
			"status", ctx.Writer.Status(),
			"latency", time.Since(startedAt),
			"client_ip", ctx.ClientIP(),
		}
		if principal, ok := gatewayauth.PrincipalFromContext(ctx); ok {
			attributes = append(attributes, "user_id", principal.Identity.UserID)
		}

		switch status := ctx.Writer.Status(); {
		case status >= http.StatusInternalServerError:
			log.Error("http request completed", attributes...)
		case status >= http.StatusBadRequest:
			log.Warn("http request completed", attributes...)
		default:
			log.Info("http request completed", attributes...)
		}
	}
}
