package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/app"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/config"
	kaf "github.com/zxCroshka/ecommerce/services/product-service/internal/kafka"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	cfg, err := config.LoadConfig("./config/config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	log := setupLogger(cfg.Service.Environment)
	log.Info("Starting product-service", "environment", cfg.Service.Environment)
	kafkaProducer, err := kaf.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		log.Error("Failed to create Kafka producer", "error", err)
		os.Exit(1)
	}
	defer kafkaProducer.Close()

	postgresURL := cfg.Postgres.GetPostgresURL()
	log.Info(
		"starting application",
		slog.String("env", cfg.Service.Environment),
	)
	application := app.New(
		context.Background(),
		log,
		cfg.GRPC.Port,
		cfg.GRPC.InternalToken,
		cfg.UserService.Address,
		cfg.Pagination.DefaultLimit,
		cfg.Pagination.MaxLimit,
		cfg.HTTP.Port,
		kafkaProducer,
		postgresURL,
		cfg.Redis.Host,
		cfg.Redis.Port,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.TTL.ProductCache,
		cfg.Redis.TTL.ProductsListCache,
		cfg.Jwt.Secret,
	)
	go application.GRPCSrv.MustRun()
	go application.HandlerSrv.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	s := <-stop

	log.Info("stopping application", slog.String("signal", s.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	application.GRPCSrv.Stop()
	log.Info("gRPC server stopped")

	if err := application.HandlerSrv.Stop(ctx); err != nil {
		log.Error("HTTP server stop error", "error", err)
	} else {
		log.Info("HTTP server stopped gracefully")
	}

	log.Info("application stopped")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger
	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	default:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	}
	return log
}
