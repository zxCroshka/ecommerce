package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/handlers/middleware"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/handlers/product"
)

type Router struct {
	engine          *gin.Engine
	productHandler  *product.ProductHandlers
	errorMiddleware *middleware.ErrorMiddleware
	authMiddleware  *middleware.AuthMiddleware  // ← добавляем
}

func NewRouter(
	productHandler *product.ProductHandlers,
	errorMiddleware *middleware.ErrorMiddleware,
	authMiddleware *middleware.AuthMiddleware,  // ← добавляем
) *Router {
	engine := gin.Default()
	return &Router{
		engine:          engine,
		productHandler:  productHandler,
		errorMiddleware: errorMiddleware,
		authMiddleware:  authMiddleware,  // ← добавляем
	}
}

func (r *Router) SetupRoutes() {
	api := r.engine.Group("/api/v1")

	// Публичные маршруты (не требуют авторизации)
	products := api.Group("/products")
	{
		products.GET("/", r.productHandler.ListProducts)
		products.GET("/:id", r.productHandler.GetProduct)
	}

	// Защищённые маршруты (требуют авторизации)
	authProducts := api.Group("/products")
	authProducts.Use(r.authMiddleware.AuthRequired())  // ← middleware
	{
		authProducts.POST("/", r.productHandler.CreateProduct)
		authProducts.PUT("/:id", r.productHandler.UpdateProduct)
		authProducts.DELETE("/:id", r.productHandler.DeleteProduct)
	}
}

func (r *Router) Run(addr string) error {
	r.engine.Use(r.errorMiddleware.ErrorHandler())
	r.SetupRoutes()
	return r.engine.Run(addr)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}