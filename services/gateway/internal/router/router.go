package router

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/cart"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/product"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/user"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/middleware"
)

type Router struct {
	engine         *gin.Engine
	authHandler    *auth.AuthHandlers
	userHandler    *user.UserHandlers
	productHandler *product.ProductHandlers
	cartHandler    *cart.CartHandlers
	authMiddleware *middleware.AuthMiddleware
	setupOnce      sync.Once
}

func NewRouter(
	log *slog.Logger,
	authHandler *auth.AuthHandlers,
	userHandler *user.UserHandlers,
	authMiddleware *middleware.AuthMiddleware,
	productHandler *product.ProductHandlers,
	cartHandler *cart.CartHandlers,
) *Router {
	engine := gin.New()
	engine.Use(
		middleware.RequestID(),
		middleware.RequestLogging(log),
		gin.Recovery(),
	)

	router := &Router{
		engine:         engine,
		authHandler:    authHandler,
		userHandler:    userHandler,
		productHandler: productHandler,
		cartHandler:    cartHandler,
		authMiddleware: authMiddleware,
	}
	router.SetupRoutes()
	return router
}

func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}

func (r *Router) SetupRoutes() {
	r.setupOnce.Do(func() {
		r.engine.GET("/healthz", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api := r.engine.Group("/api/v1")
		r.setupAuthRoutes(api)
		r.setupUserRoutes(api)
		r.setupProductRoutes(api)
		r.setupCartRoutes(api)
	})
}

func (r *Router) setupAuthRoutes(api *gin.RouterGroup) {
	authGroup := api.Group("/auth")
	authGroup.POST("/register", r.authHandler.Register)
	authGroup.POST("/login", r.authHandler.Login)
	authGroup.POST("/refresh", r.authHandler.RefreshToken)
	authGroup.POST(
		"/logout",
		r.authMiddleware.AuthRequired(),
		r.authHandler.Logout,
	)
}

func (r *Router) setupUserRoutes(api *gin.RouterGroup) {
	users := api.Group("/users/me")
	users.Use(r.authMiddleware.AuthRequired())
	users.GET("", r.userHandler.GetUser)
	users.PATCH("/email", r.userHandler.UpdateEmail)
	users.PATCH("/name", r.userHandler.UpdateName)
	users.PATCH("/password", r.userHandler.UpdatePassword)
}

func (r *Router) setupProductRoutes(api *gin.RouterGroup) {
	products := api.Group("/products")
	products.GET("", r.productHandler.ListProducts)
	products.GET("/:id", r.productHandler.GetProduct)

	adminProducts := products.Group("")
	adminProducts.Use(
		r.authMiddleware.AuthRequired(),
		r.authMiddleware.RequireRole("admin"),
	)
	adminProducts.POST("", r.productHandler.CreateProduct)
	adminProducts.PATCH("/:id", r.productHandler.UpdateProduct)
	adminProducts.DELETE("/:id", r.productHandler.DeleteProduct)
}

func (r *Router) setupCartRoutes(api *gin.RouterGroup) {
	cartGroup := api.Group("/cart")
	cartGroup.Use(r.authMiddleware.AuthRequired())
	cartGroup.GET("", r.cartHandler.GetCart)
	cartGroup.POST("/items", r.cartHandler.AddProduct)
	cartGroup.PATCH("/items/:product_id", r.cartHandler.ChangeQuantity)
	cartGroup.DELETE("/items/:product_id", r.cartHandler.RemoveProduct)
}

func (r *Router) Run(address string) error {
	return r.engine.Run(address)
}
