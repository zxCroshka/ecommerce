package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zxCroshka/ecommerce/services/cart-service/internal/app"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/config"
)

func main() {
	cfg, err := config.LoadConfig("./config/config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	log := setupLogger(cfg.Logging)
	log.Info("starting cart-service", "environment", cfg.Service.Environment)

	application, err := app.New(log, cfg)
	if err != nil {
		log.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	go application.GRPCSrv.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	s := <-stop
	log.Info("stopping application", "signal", s.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := application.GRPCSrv.Stop(shutdownCtx); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("failed to stop gRPC server", "error", err)
	}
	if err := application.Close(); err != nil {
		log.Error("failed to close application resources", "error", err)
	}
	log.Info("application stopped")
}

func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.Level))
	options := &slog.HandlerOptions{Level: level}
	if cfg.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
