package handlersapp

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/auth"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/middleware"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/user"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service"
)

type App struct {
	log    *slog.Logger
	router *handlers.Router
	port   int
}

func New(
	log *slog.Logger,
	userService *service.UserService,
	port int,
) *App {
	authHandler := auth.New(log, userService)
	userHandler := user.New(log, userService)
	authMiddleware := middleware.NewAuthMiddleware(userService)
	errorMiddleware := middleware.NewErrorMiddleware()
	router := handlers.NewRouter(authHandler, userHandler, authMiddleware, errorMiddleware)

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

	// Просто вызываем метод Run у роутера
	if err := a.router.Run(addr); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	const op = "handlersapp.Stop"
	log := a.log.With(slog.String("op", op), slog.Int("port", a.port))

	log.Info("stopping handler server gracefully", slog.Int("port", a.port))

	// Здесь можно добавить логику для graceful shutdown, если она реализована в роутере.
	// Пока оставим заглушкой.
	log.Info("handler server stopped")
	return nil
}
