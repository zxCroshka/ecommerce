package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/app"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/config"
)

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg, err := config.LoadConfig("./config/config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	log := setupLogger(cfg.Service.Environment)
	log.Info("Starting product-service", "environment", cfg.Service.Environment)
	log.Info(
		"starting application",
		slog.String("env", cfg.Service.Environment),
	)
	application, err := app.New(ctx, log, cfg)
	if err != nil {
		log.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}
	if err := application.Start(ctx); err != nil {
		log.Error("application stopped with an error", "error", err)
		os.Exit(1)
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
