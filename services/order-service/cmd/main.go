package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/zxCroshka/ecommerce/services/order-service/internal/app"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	configuration, err := config.Load("./config/config.yaml")
	if err != nil {
		slog.Error("failed to load Order config", "error", err)
		os.Exit(1)
	}
	log := newLogger(configuration.Logging)
	application, err := app.New(ctx, log, configuration)
	if err != nil {
		log.Error("failed to initialize Order Service", "error", err)
		os.Exit(1)
	}
	if err := application.Start(ctx); err != nil {
		log.Error("Order Service stopped with an error", "error", err)
		os.Exit(1)
	}
}

func newLogger(configuration config.LoggingConfig) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.TrimSpace(configuration.Level))); err != nil {
		level = slog.LevelInfo
	}
	options := &slog.HandlerOptions{Level: level}
	if configuration.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
