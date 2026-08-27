package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service"
)

type AuthMiddleware struct {
	userService service.UserServiceInterface
}

func NewAuthMiddleware(userService service.UserServiceInterface) *AuthMiddleware {
	return &AuthMiddleware{
		userService: userService,
	}
}

func (m *AuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization header",
			})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format. Use: Bearer <token>",
			})
			c.Abort()
			return
		}

		token := strings.TrimSpace(parts[1])

		identity, err := m.userService.ValidateToken(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			c.Abort()
			return
		}

		c.Set("identity", identity)
		c.Set("user_id", identity.UserID)
		c.Set("role", identity.Role)

		c.Next()
	}
}
