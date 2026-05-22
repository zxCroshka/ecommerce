package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	log *slog.Logger
	secret []byte
}

func NewAuthMiddleware(log *slog.Logger, secret string) *AuthMiddleware {
	return &AuthMiddleware{
		log:    log,
		secret: []byte(secret),
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

		tokenString := parts[1]

		// Парсим токен
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.secret, nil
		})

		if err != nil || !token.Valid {
			m.log.Error("invalid token", "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			c.Abort()
			return
		}

		// Извлекаем claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token claims",
			})
			c.Abort()
			return
		}

		// Извлекаем user_id и is_admin
		userID, userIDOk := claims["user_id"].(float64)
		isAdmin, adminOk := claims["is_admin"].(bool)

		if !userIDOk {
			m.log.Error("user_id not found in token")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token: user_id missing",
			})
			c.Abort()
			return
		}

		// Кладём в контекст
		c.Set("user_id", int64(userID))
		c.Set("isAdmin", isAdmin && adminOk)
		
		m.log.Debug("token validated", 
			"user_id", userID, 
			"is_admin", isAdmin && adminOk)

		c.Next()
	}
}