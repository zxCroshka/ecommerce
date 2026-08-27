package app

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/zxCroshka/ecommerce/services/cart-service/internal/app/grpcapp"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/config"
	cartgrpc "github.com/zxCroshka/ecommerce/services/cart-service/internal/grpc"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/repository"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/service"
)

type App struct {
	GRPCSrv       *grpcapp.App
	cartManager   *repository.Client
	productClient *cartgrpc.Client
}

func New(log *slog.Logger, cfg *config.Config) (*App, error) {
	cartManager, err := repository.NewClient(repository.Config{
		Host:     cfg.Redis.Host,
		Port:     cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("create cart repository: %w", err)
	}

	productClient, err := cartgrpc.New(cartgrpc.ClientConfig{
		Address:    cfg.ProductService.Address,
		RetryCount: cfg.ProductService.RetryCount,
		Timeout:    cfg.ProductService.Timeout,
	})
	if err != nil {
		_ = cartManager.Close()
		return nil, fmt.Errorf("create product service client: %w", err)
	}

	cartService := service.NewCartService(
		log,
		cartManager,
		productClient,
		cfg.Cart.TTL,
		cfg.Cart.MaxProductQuantity,
	)
	return &App{
		GRPCSrv:       grpcapp.New(log, cartService, cfg.GRPC.Port),
		cartManager:   cartManager,
		productClient: productClient,
	}, nil
}

func (a *App) Close() error {
	return errors.Join(a.productClient.Close(), a.cartManager.Close())
}
