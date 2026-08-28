package middleware

import (
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	RequestIDHeader = "X-Request-ID"
	requestIDKey    = "request.id"
	maxRequestIDLen = 128
)

func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := strings.TrimSpace(ctx.GetHeader(RequestIDHeader))
		if !validRequestID(requestID) {
			requestID = uuid.NewString()
		}

		ctx.Set(requestIDKey, requestID)
		ctx.Header(RequestIDHeader, requestID)
		ctx.Next()
	}
}

func RequestIDFromContext(ctx *gin.Context) (string, bool) {
	value, exists := ctx.Get(requestIDKey)
	if !exists {
		return "", false
	}
	requestID, ok := value.(string)
	return requestID, ok && requestID != ""
}

func validRequestID(value string) bool {
	if value == "" || len(value) > maxRequestIDLen {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return false
		}
	}
	return true
}
