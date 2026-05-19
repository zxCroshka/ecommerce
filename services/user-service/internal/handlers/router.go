// router.go
package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/auth"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/middleware"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/user"
)

type Router struct {
	authHandler    *auth.AuthHandlers
	userHandler    *user.UserHandlers
	authMiddleware *middleware.AuthMiddleware
	errorMiddleware *middleware.ErrorMiddleware
}

func NewRouter(
	authHandler *auth.AuthHandlers,
	userHandler *user.UserHandlers,
	authMiddleware *middleware.AuthMiddleware,
	errorMiddleware *middleware.ErrorMiddleware,
) *Router {
	return &Router{
		authHandler:    authHandler,
		userHandler:    userHandler,
		authMiddleware: authMiddleware,
		errorMiddleware: errorMiddleware,
	}
}

func (r *Router) SetupRoutes(router *gin.Engine) {
	api := router.Group("/api/v1")

	auth := api.Group("/auth")
	{
		auth.POST("/register", r.authHandler.Register)
		auth.POST("/login", r.authHandler.Login)
		auth.POST("/refresh", r.authHandler.RefreshToken)
	}

	protected := api.Group("/")
	protected.Use(r.authMiddleware.AuthRequired())
	{
		protected.POST("/auth/logout", r.authHandler.Logout)

		userGroup := protected.Group("/user")
		{
			userGroup.GET("/profile", r.userHandler.GetProfile)
			userGroup.PUT("/email", r.userHandler.UpdateEmail)
			userGroup.PUT("/name", r.userHandler.UpdateName)
			userGroup.PUT("/password", r.userHandler.UpdatePassword)
		}
	}
}

func (r *Router) Run(addr string)error {
	router := gin.Default()

	router.Use(r.errorMiddleware.ErrorHandler())

	r.SetupRoutes(router)

	if err := router.Run(addr); err != nil {
		return err
	}
	return nil
}
