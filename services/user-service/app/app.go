package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/zxCroshka/ecommerce/services/user-service/app/grpcapp"
	"github.com/zxCroshka/ecommerce/services/user-service/app/handlersapp"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/redis"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service"
	kaf "github.com/zxCroshka/ecommerce/services/user-service/kafka"
)

type App struct {
	GRPCSrv    *grpcapp.App
	HandlerSrv *handlersapp.App
}

func New(
	ctx context.Context,
	log *slog.Logger,
	grpcPort int,
	handlerPort int,
	producer *kaf.Producer,
	storageURL string,
	tokenTTL time.Duration,
	accessTTL time.Duration,
	refreshTTl time.Duration,
	redisHost string,
	redisPort int,
	redisPassword string,
	redisDB int,
	jwtSecret string,
) *App {
	storage, err := repository.New(ctx, storageURL)
	if err != nil {
		panic(err)
	}
	cfg := redis.Config{
		Host:     redisHost,
		Port:     redisPort,
		Password: redisPassword,
		DB:       redisDB,
	}
	redisClient, err := redis.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	jwtmanager := jwt.NewJWTService(jwtSecret, accessTTL, refreshTTl)
	userService := service.NewUserService(log, storage, redisClient, producer, jwtmanager)
	grpcApp := grpcapp.New(log, userService, grpcPort)
	handlersApp := handlersapp.New(log, userService, handlerPort)
	return &App{
		GRPCSrv:    grpcApp,
		HandlerSrv: handlersApp,
	}

}
