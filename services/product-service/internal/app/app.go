package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/app/grpcapp"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/config"
	productkafka "github.com/zxCroshka/ecommerce/services/product-service/internal/kafka"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres"
	redisrepository "github.com/zxCroshka/ecommerce/services/product-service/internal/repository/redis"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/service"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

const shutdownTimeout = 10 * time.Second

type App struct {
	GRPCSrv *grpcapp.App

	storage      *postgres.Storage
	redisClient  *redisrepository.Client
	producer     *productkafka.Producer
	relay        *outbox.Relay
	shutdownOnce sync.Once
	shutdownErr  error
}

func New(ctx context.Context, log *slog.Logger, cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("product service config is required")
	}
	if log == nil {
		log = slog.Default()
	}

	storage, err := postgres.New(
		ctx,
		cfg.Postgres.GetPostgresURL(),
		cfg.Kafka.Topic.ProductUpdated,
	)
	if err != nil {
		return nil, fmt.Errorf("create product storage: %w", err)
	}

	redisClient, err := redisrepository.NewClient(redisrepository.Config{
		Host: cfg.Redis.Host, Port: cfg.Redis.Port, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
		ProductTTL: cfg.Redis.TTL.ProductCache, ProductsListTTL: cfg.Redis.TTL.ProductsListCache,
	})
	if err != nil {
		storage.Close()
		return nil, fmt.Errorf("create product Redis client: %w", err)
	}

	producer, err := productkafka.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		_ = redisClient.Close()
		storage.Close()
		return nil, fmt.Errorf("create product Kafka producer: %w", err)
	}

	relay, err := outbox.NewRelay(log, storage.OutboxStore(), producer, outbox.RelayConfig{
		PollInterval:   cfg.Outbox.PollInterval,
		PublishTimeout: cfg.Outbox.PublishTimeout,
		StoreTimeout:   cfg.Outbox.StoreTimeout,
		LockTimeout:    cfg.Outbox.LockTimeout,
		RetryBaseDelay: cfg.Outbox.RetryBaseDelay,
		RetryMaxDelay:  cfg.Outbox.RetryMaxDelay,
		BatchSize:      cfg.Outbox.BatchSize,
	})
	if err != nil {
		_ = producer.Close()
		_ = redisClient.Close()
		storage.Close()
		return nil, fmt.Errorf("create product outbox relay: %w", err)
	}

	productService := service.New(log, storage, redisClient)
	grpcApp, err := grpcapp.New(
		log,
		productService,
		cfg.GRPC.Port,
		cfg.GRPC.InternalToken,
		cfg.UserService.Address,
		cfg.Pagination.DefaultLimit,
		cfg.Pagination.MaxLimit,
	)
	if err != nil {
		_ = producer.Close()
		_ = redisClient.Close()
		storage.Close()
		return nil, fmt.Errorf("create product gRPC server: %w", err)
	}

	return &App{
		GRPCSrv:     grpcApp,
		storage:     storage,
		redisClient: redisClient,
		producer:    producer,
		relay:       relay,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	if err := a.relay.Start(ctx); err != nil {
		return err
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- a.GRPCSrv.Run() }()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-serverErrors:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return errors.Join(runErr, a.Shutdown(shutdownCtx))
}

func (a *App) Shutdown(ctx context.Context) error {
	a.shutdownOnce.Do(func() {
		relayErr := a.relay.Stop(ctx)
		grpcErr := a.GRPCSrv.Stop(ctx)
		producerErr := a.producer.Close()
		redisErr := a.redisClient.Close()
		a.storage.Close()
		a.shutdownErr = errors.Join(relayErr, grpcErr, producerErr, redisErr)
	})
	return a.shutdownErr
}
