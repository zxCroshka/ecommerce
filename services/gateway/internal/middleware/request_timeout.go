package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestTimeout establishes one end-to-end budget inherited by all gRPC calls,
// retries and downstream work.
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if timeout <= 0 {
			ctx.Next()
			return
		}
		requestCtx, cancel := context.WithTimeout(ctx.Request.Context(), timeout)
		defer cancel()
		ctx.Request = ctx.Request.WithContext(requestCtx)
		ctx.Next()
	}
}
