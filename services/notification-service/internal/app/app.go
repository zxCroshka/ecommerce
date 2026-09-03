package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/zxCroshka/ecommerce/services/notification-service/internal/app/grpcapp"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/config"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/events"
	notificationkafka "github.com/zxCroshka/ecommerce/services/notification-service/internal/kafka"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/repository"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/service"
)

const shutdownTimeout = 15 * time.Second

type App struct {
	grpcServer *grpcapp.App
	consumer   *notificationkafka.Consumer
	storage    *repository.Storage

	shutdownOnce sync.Once
	shutdownErr  error
}

func New(ctx context.Context, log *slog.Logger, configuration *config.Config) (*App, error) {
	if configuration == nil {
		return nil, fmt.Errorf("Notification config is required")
	}
	if log == nil {
		log = slog.Default()
	}
	storage, err := repository.New(ctx, configuration.Postgres.URL())
	if err != nil {
		return nil, err
	}
	notifications, err := service.New(storage)
	if err != nil {
		storage.Close()
		return nil, err
	}
	handler, err := events.NewHandler(storage)
	if err != nil {
		storage.Close()
		return nil, err
	}
	consumer, err := notificationkafka.New(log, handler, notificationkafka.Config{
		Brokers:         configuration.Kafka.Brokers,
		GroupID:         configuration.Kafka.GroupID,
		Topics:          configuration.Kafka.Topics,
		AutoOffsetReset: configuration.Kafka.AutoOffsetReset,
		PollInterval:    configuration.Kafka.PollInterval,
		HandlerTimeout:  configuration.Kafka.HandlerTimeout,
		MaxRetries:      configuration.Kafka.MaxRetries,
		RetryBaseDelay:  configuration.Kafka.RetryBaseDelay,
		RetryMaxDelay:   configuration.Kafka.RetryMaxDelay,
	})
	if err != nil {
		storage.Close()
		return nil, err
	}
	grpcServer, err := grpcapp.New(
		log,
		notifications,
		configuration.GRPC.Port,
		configuration.UserService.Address,
		configuration.UserService.Timeout,
		configuration.Notification.DefaultListLimit,
		configuration.Notification.MaxListLimit,
	)
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = consumer.Stop(cleanupCtx)
		cancel()
		storage.Close()
		return nil, err
	}
	return &App{grpcServer: grpcServer, consumer: consumer, storage: storage}, nil
}

func (a *App) Start(ctx context.Context) error {
	if err := a.consumer.Start(ctx); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return errors.Join(err, a.Shutdown(shutdownCtx))
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- a.grpcServer.Run() }()
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-serverErrors:
	case runErr = <-a.consumer.Errors():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return errors.Join(runErr, a.Shutdown(shutdownCtx))
}

func (a *App) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	a.shutdownOnce.Do(func() {
		// Stop ingestion first so no new writes race with database shutdown. The
		// API may continue serving already persisted notifications while the
		// consumer drains its current bounded handler call.
		a.shutdownErr = errors.Join(a.consumer.Stop(ctx), a.grpcServer.Stop(ctx))
		a.storage.Close()
	})
	return a.shutdownErr
}
