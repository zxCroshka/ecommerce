package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/zxCroshka/ecommerce/services/order-service/internal/app/grpcapp"
	cartclient "github.com/zxCroshka/ecommerce/services/order-service/internal/clients/cart"
	productclient "github.com/zxCroshka/ecommerce/services/order-service/internal/clients/product"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/config"
	orderkafka "github.com/zxCroshka/ecommerce/services/order-service/internal/kafka"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/repository"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/service"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

const shutdownTimeout = 15 * time.Second

type App struct {
	grpcServer *grpcapp.App
	storage    *repository.Storage
	cart       *cartclient.Client
	products   *productclient.Client
	producer   *orderkafka.Producer
	relay      *outbox.Relay
	recovery   *service.Recovery

	shutdownOnce sync.Once
	shutdownErr  error
}

func New(ctx context.Context, log *slog.Logger, config *config.Config) (*App, error) {
	if config == nil {
		return nil, fmt.Errorf("Order config is required")
	}
	if log == nil {
		log = slog.Default()
	}

	storage, err := repository.New(ctx, config.Postgres.URL(), config.Kafka.Topic.OrderCreated)
	if err != nil {
		return nil, err
	}
	cart, err := cartclient.New(cartclient.Config{
		Address:       config.CartService.Address,
		InternalToken: config.CartService.InternalToken,
		Timeout:       config.CartService.Timeout,
	})
	if err != nil {
		storage.Close()
		return nil, err
	}
	products, err := productclient.New(productclient.Config{
		Address:       config.ProductService.Address,
		InternalToken: config.ProductService.InternalToken,
		Timeout:       config.ProductService.Timeout,
	})
	if err != nil {
		_ = cart.Close()
		storage.Close()
		return nil, err
	}
	producer, err := orderkafka.New(config.Kafka.Brokers)
	if err != nil {
		_ = products.Close()
		_ = cart.Close()
		storage.Close()
		return nil, err
	}
	relay, err := outbox.NewRelay(log, storage.OutboxStore(), producer, outbox.RelayConfig{
		PollInterval:   config.Outbox.PollInterval,
		PublishTimeout: config.Outbox.PublishTimeout,
		StoreTimeout:   config.Outbox.StoreTimeout,
		LockTimeout:    config.Outbox.LockTimeout,
		RetryBaseDelay: config.Outbox.RetryBaseDelay,
		RetryMaxDelay:  config.Outbox.RetryMaxDelay,
		BatchSize:      config.Outbox.BatchSize,
	})
	if err != nil {
		_ = producer.Close()
		_ = products.Close()
		_ = cart.Close()
		storage.Close()
		return nil, err
	}
	orderService, err := service.New(log, storage, cart, products, service.Config{
		Currency:             config.Order.Currency,
		MaxItems:             config.Order.MaxItems,
		MaxIdempotencyLength: config.Order.MaxIdempotencyLength,
		LeaseTimeout:         config.Workflow.LeaseTimeout,
		FinalizeTimeout:      config.Workflow.FinalizeTimeout,
		CompensationTimeout:  config.Workflow.CompensationTimeout,
		CartCleanupTimeout:   config.Workflow.CartCleanupTimeout,
	})
	if err != nil {
		_ = producer.Close()
		_ = products.Close()
		_ = cart.Close()
		storage.Close()
		return nil, err
	}
	recovery, err := service.NewRecovery(log, orderService, service.RecoveryConfig{
		PollInterval: config.Recovery.PollInterval,
		RecoveryAge:  config.Recovery.RecoveryAge,
		OrderTimeout: config.Recovery.OrderTimeout,
		BatchSize:    config.Recovery.BatchSize,
	})
	if err != nil {
		_ = producer.Close()
		_ = products.Close()
		_ = cart.Close()
		storage.Close()
		return nil, err
	}
	grpcServer, err := grpcapp.New(
		log,
		orderService,
		config.GRPC.Port,
		config.UserService.Address,
		config.UserService.Timeout,
		config.Order.DefaultListLimit,
		config.Order.MaxListLimit,
	)
	if err != nil {
		_ = producer.Close()
		_ = products.Close()
		_ = cart.Close()
		storage.Close()
		return nil, err
	}
	return &App{
		grpcServer: grpcServer,
		storage:    storage,
		cart:       cart,
		products:   products,
		producer:   producer,
		relay:      relay,
		recovery:   recovery,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	if err := a.relay.Start(ctx); err != nil {
		return err
	}
	if err := a.recovery.Start(ctx); err != nil {
		stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return errors.Join(err, a.relay.Stop(stopCtx))
	}

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- a.grpcServer.Run() }()
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
		a.shutdownErr = errors.Join(
			a.recovery.Stop(ctx),
			a.relay.Stop(ctx),
			a.grpcServer.Stop(ctx),
			a.products.Close(),
			a.cart.Close(),
			a.producer.Close(),
		)
		a.storage.Close()
	})
	return a.shutdownErr
}
