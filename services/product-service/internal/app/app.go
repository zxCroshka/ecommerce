package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/app/grpcapp"
	kaf "github.com/zxCroshka/ecommerce/services/product-service/internal/kafka"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/redis"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/service"
)

type App struct {
	GRPCSrv *grpcapp.App
}

func New(
	ctx context.Context,
	log *slog.Logger,
	grpcPort int,
	grpcInternalToken string,
	userServiceAddress string,
	defaultListLimit int,
	maxListLimit int,
	producer *kaf.Producer,
	storageURL string,
	redisHost string,
	redisPort int,
	redisPassword string,
	redisDB int,
	productCacheTTL time.Duration,
	productsListCacheTTL time.Duration,

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
	grpcApp := grpcapp.New(
		log,
		productService,
		grpcPort,
		grpcInternalToken,
		userServiceAddress,
		defaultListLimit,
		maxListLimit,
	)
	return &App{
		GRPCSrv: grpcApp,
	}

}
