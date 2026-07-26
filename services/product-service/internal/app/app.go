package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/app/grpcapp"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/app/handlersapp"
	kaf "github.com/zxCroshka/ecommerce/services/product-service/internal/kafka"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/redis"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/service"
)

type App struct {
	GRPCSrv    *grpcapp.App
	HandlerSrv *handlersapp.App
}

func New(
	ctx context.Context,
	log *slog.Logger,
	grpcPort int,
	grpcInternalToken string,
	handlerPort int,
	producer *kaf.Producer,
	storageURL string,
	tokenTTL time.Duration,
	redisHost string,
	redisPort int,
	redisPassword string,
	redisDB int,
	productCacheTTL time.Duration,
	productsListCacheTTL time.Duration,
	jwtSecret string,

) *App {
	storage, err := postgres.New(ctx, storageURL)
	if err != nil {
		panic(err)
	}
	cfg := redis.Config{
		Host:            redisHost,
		Port:            redisPort,
		Password:        redisPassword,
		DB:              redisDB,
		ProductTTL:      productCacheTTL,
		ProductsListTTL: productsListCacheTTL,
	}
	redisClient, err := redis.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	productService := service.New(log, storage, redisClient, producer)
	grpcApp := grpcapp.New(log, productService, grpcPort, grpcInternalToken)
	handlersApp := handlersapp.New(log, productService, handlerPort, jwtSecret)
	return &App{
		GRPCSrv:    grpcApp,
		HandlerSrv: handlersApp,
	}

}
