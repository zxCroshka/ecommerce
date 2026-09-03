package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/cartservice"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/notificationservice"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/orderservice"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/productservice"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/userservice"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/config"
	authhandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/auth"
	carthandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/cart"
	notificationhandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/notification"
	orderhandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/order"
	producthandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/product"
	userhandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/user"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/middleware"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/router"
)

type App struct {
	log                *slog.Logger
	server             *http.Server
	userClient         *userservice.UserClient
	productClient      *productservice.ProductClient
	cartClient         *cartservice.CartClient
	orderClient        *orderservice.Client
	notificationClient *notificationservice.Client
	shutdownTimeout    time.Duration
	shutdownOnce       sync.Once
	shutdownErr        error
}

func New(cfg *config.Config, log *slog.Logger) (*App, error) {
	const op = "gateway.app.New"
	if cfg == nil {
		return nil, fmt.Errorf("%s: config is required", op)
	}
	if log == nil {
		log = slog.Default()
	}

	userClient, err := userservice.New(userservice.ClientConfig{
		Address:    cfg.UserService.Address,
		RetryCount: cfg.UserService.RetryCount,
		Timeout:    cfg.UserService.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: create user client: %w", op, err)
	}

	productClient, err := productservice.New(productservice.ClientConfig{
		Address:       cfg.ProductService.Address,
		InternalToken: cfg.ProductService.InternalToken,
		RetryCount:    cfg.ProductService.RetryCount,
		Timeout:       cfg.ProductService.Timeout,
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("%s: create product client: %w", op, err),
			userClient.Close(),
		)
	}

	cartClient, err := cartservice.New(cartservice.ClientConfig{
		Address:    cfg.CartService.Address,
		RetryCount: cfg.CartService.RetryCount,
		Timeout:    cfg.CartService.Timeout,
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("%s: create cart client: %w", op, err),
			productClient.Close(),
			userClient.Close(),
		)
	}

	orderClient, err := orderservice.New(orderservice.Config{
		Address:    cfg.OrderService.Address,
		RetryCount: cfg.OrderService.RetryCount,
		Timeout:    cfg.OrderService.Timeout,
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("%s: create order client: %w", op, err),
			cartClient.Close(),
			productClient.Close(),
			userClient.Close(),
		)
	}

	notificationClient, err := notificationservice.New(notificationservice.Config{
		Address: cfg.NotificationService.Address, RetryCount: cfg.NotificationService.RetryCount, Timeout: cfg.NotificationService.Timeout,
	})
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("%s: create notification client: %w", op, err),
			orderClient.Close(), cartClient.Close(), productClient.Close(), userClient.Close(),
		)
	}

	authMiddleware := middleware.NewAuthMiddleware(log, userClient)
	authHandler := authhandlers.New(log, userClient)
	userHandler := userhandlers.New(log, userClient)
	productHandler := producthandlers.New(log, productClient)
	cartHandler := carthandlers.New(log, cartClient)
	orderHandler := orderhandlers.New(orderClient)
	notificationHandler := notificationhandlers.New(notificationClient)
	httpRouter := router.NewRouter(
		log,
		authHandler,
		userHandler,
		authMiddleware,
		productHandler,
		cartHandler,
		orderHandler,
		notificationHandler,
		cfg.HTTP.RequestTimeout,
	)

	server := &http.Server{
		Addr:              cfg.HTTP.Address(),
		Handler:           httpRouter.GetEngine(),
		ReadHeaderTimeout: cfg.HTTP.ReadTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	return &App{
		log:                log,
		server:             server,
		userClient:         userClient,
		productClient:      productClient,
		cartClient:         cartClient,
		orderClient:        orderClient,
		notificationClient: notificationClient,
		shutdownTimeout:    cfg.HTTP.ShutdownTimeout,
	}, nil
}

func (a *App) Run() error {
	if a == nil || a.server == nil {
		return errors.New("gateway app is not initialized")
	}
	a.log.Info("api gateway is starting", "address", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}

	a.shutdownOnce.Do(func() {
		a.log.Info("api gateway is shutting down")
		var serverErr error
		if a.server != nil {
			serverErr = a.server.Shutdown(ctx)
			if serverErr != nil {
				serverErr = errors.Join(serverErr, a.server.Close())
			}
		}

		a.shutdownErr = errors.Join(
			serverErr,
			a.closeUserClient(),
			a.closeProductClient(),
			a.closeCartClient(),
			a.closeOrderClient(),
			a.closeNotificationClient(),
		)
	})
	return a.shutdownErr
}

func (a *App) closeNotificationClient() error {
	if a.notificationClient == nil {
		return nil
	}
	return a.notificationClient.Close()
}

func (a *App) closeOrderClient() error {
	if a.orderClient == nil {
		return nil
	}
	return a.orderClient.Close()
}

func (a *App) ShutdownTimeout() time.Duration {
	if a == nil {
		return 0
	}
	return a.shutdownTimeout
}

func (a *App) closeUserClient() error {
	if a.userClient == nil {
		return nil
	}
	return a.userClient.Close()
}

func (a *App) closeProductClient() error {
	if a.productClient == nil {
		return nil
	}
	return a.productClient.Close()
}

func (a *App) closeCartClient() error {
	if a.cartClient == nil {
		return nil
	}
	return a.cartClient.Close()
}
