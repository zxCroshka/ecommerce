package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	log    *slog.Logger
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

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return m.secret, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

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

		// Извлекаем user_id и role
		userID, userIDOk := claims["user_id"].(float64)
		role, roleOK := claims["role"].(string)
		tokenType, tokenTypeOK := claims["token_type"].(string)

		if !userIDOk || !roleOK || !tokenTypeOK || tokenType != "access" {
			m.log.Error("user_id not found in token")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid access token claims",
			})
			c.Abort()
			return
		}

		// Кладём в контекст
		c.Set("user_id", int64(userID))
		c.Set("role", role)
		c.Set("isAdmin", role == "admin")

		m.log.Debug("token validated",
			"user_id", userID,
			"role", role)

		c.Next()
	}
}
