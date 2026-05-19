package handlersapp

import (
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
		port: port,
	}

}

func (a *App) MustRun(){
	if err := a.Run(); err != nil{
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "handlersapp.Run"
	log := a.log.With(slog.String("op", op), slog.Int("port", a.port))
	addr := fmt.Sprintf(":%d", a.port)
	log.Info("Handler server is running", slog.String("addr", addr))

	if err := a.router.Run(addr); err != nil{
		return fmt.Errorf("%sL %w",op,err)
	}
	return nil
}
