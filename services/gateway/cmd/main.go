package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/zxCroshka/ecommerce/services/gateway/internal/app"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/config"
)

const configPath = "./config/config.yaml"

func main() {
	if err := run(); err != nil {
		slog.Error("api gateway stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log, err := newLogger(cfg.Logging)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	slog.SetDefault(log)
	log.Info(
		"api gateway initialization started",
		"service", cfg.Service.Name,
		"environment", cfg.Service.Environment,
	)

	application, err := app.New(cfg, log)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}

	signalContext, stopSignals := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stopSignals()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- application.Run()
	}()

	select {
	case serveErr := <-serverErrors:
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			application.ShutdownTimeout(),
		)
		defer cancel()
		return errors.Join(serveErr, application.Shutdown(shutdownContext))

	case <-signalContext.Done():
		log.Info("shutdown signal received")
		shutdownContext, cancel := context.WithTimeout(
			context.Background(),
			application.ShutdownTimeout(),
		)
		defer cancel()

		shutdownErr := application.Shutdown(shutdownContext)
		serveErr := <-serverErrors
		if shutdownErr == nil && serveErr == nil {
			log.Info("api gateway stopped gracefully")
		}
		return errors.Join(shutdownErr, serveErr)
	}
}

func newLogger(cfg config.LoggingConfig) (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.TrimSpace(cfg.Level))); err != nil {
		return nil, fmt.Errorf("invalid logging level %q: %w", cfg.Level, err)
	}

	options := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(cfg.Format)) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, options)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, options)
	default:
		return nil, fmt.Errorf("unsupported logging format %q", cfg.Format)
	}
	return slog.New(handler), nil
}
