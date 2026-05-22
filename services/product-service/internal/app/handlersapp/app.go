package handlersapp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/handlers"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/handlers/middleware"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/handlers/product"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/service"
)

type App struct {
	log    *slog.Logger
	router *handlers.Router
	port   int
}

func New(
	log *slog.Logger,
	productService *service.ProductService,
	port int,
	jwtSecret string,  // ← добавляем
) *App {
	productHandler := product.New(log, productService)
	errorMiddleware := middleware.NewErrorMiddleware()
	authMiddleware := middleware.NewAuthMiddleware(log, jwtSecret)  // ← создаём
	router := handlers.NewRouter(productHandler, errorMiddleware, authMiddleware)  // ← передаём
	
	return &App{
		log:    log,
		router: router,
		port:   port,
	}
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "handlersapp.Run"
	log := a.log.With(slog.String("op", op), slog.Int("port", a.port))
	addr := fmt.Sprintf(":%d", a.port)

	log.Info("Handler server is running", slog.String("addr", addr))

	if err := a.router.Run(addr); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	log := a.log.With(slog.Int("port", a.port))
	log.Info("stopping handler server gracefully")
	return nil
}