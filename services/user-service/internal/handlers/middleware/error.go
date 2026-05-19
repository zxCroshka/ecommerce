package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	errs "github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/err"
)

type ErrorMiddleware struct{}


func NewErrorMiddleware() *ErrorMiddleware{
	return &ErrorMiddleware{}
}


func (m *ErrorMiddleware) ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		var appErr *errs.AppError
		if errors.As(err, &appErr) {
			c.JSON(appErr.Status, gin.H{
				"success": false,
				"error":   gin.H{"code": appErr.Code, "message": appErr.Message},
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   gin.H{"code": "INTERNAL", "message": "an unexpected error occurred"},
			})
		}
	}
}
