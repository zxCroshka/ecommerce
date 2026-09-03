package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/app/grpcapp"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/config"
	kaf "github.com/zxCroshka/ecommerce/services/user-service/internal/kafka"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository"
	redisrepository "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/redis"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

const shutdownTimeout = 10 * time.Second

type App struct {
	GRPCSrv *grpcapp.App
	log     *slog.Logger

	storage      *repository.Storage
	redisClient  *redisrepository.Client
	producer     *kaf.Producer
	relay        *outbox.Relay
	pprofServer  *http.Server
	pprofDone    chan struct{}
	shutdownOnce sync.Once
	shutdownErr  error
}

func New(ctx context.Context, log *slog.Logger, cfg *config.Config) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("user service config is required")
	}
	if log == nil {
		log = slog.Default()
	}

	storage, err := repository.New(
		ctx,
		cfg.Postgres.GetPostgresURL(),
		cfg.Kafka.Topic.UserRegistered,
	)
	if err != nil {
		return nil, fmt.Errorf("create user storage: %w", err)
	}

	redisClient, err := redisrepository.NewClient(redisrepository.Config{
		Host: cfg.Redis.Host, Port: cfg.Redis.Port, Password: cfg.Redis.Password, DB: cfg.Redis.DB,
	})
	if err != nil {
		storage.Close()
		return nil, fmt.Errorf("create user Redis client: %w", err)
	}

	producer, err := kaf.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		_ = redisClient.Close()
		storage.Close()
		return nil, fmt.Errorf("create user Kafka producer: %w", err)
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
		return nil, fmt.Errorf("create user outbox relay: %w", err)
	}

	jwtManager := jwt.NewJWTService(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	userService := service.NewUserService(log, storage, redisClient, jwtManager)
	application := &App{
		GRPCSrv:     grpcapp.New(log, userService, cfg.GRPC.Port),
		log:         log,
		storage:     storage,
		redisClient: redisClient,
		producer:    producer,
		relay:       relay,
	}
	if cfg.Pprof.Enabled {
		engine := gin.New()
		SetupPProfHandlers(engine)
		application.pprofServer = &http.Server{
			Addr:              "127.0.0.1:3366",
			Handler:           engine,
			ReadHeaderTimeout: 5 * time.Second,
		}
	}
	return application, nil
}

func (a *App) Start(ctx context.Context) error {
	if err := a.relay.Start(ctx); err != nil {
		return err
	}

	serverErrors := make(chan error, 2)
	go func() { serverErrors <- a.GRPCSrv.Run() }()
	if a.pprofServer != nil {
		a.pprofDone = make(chan struct{})
		go func() {
			defer close(a.pprofDone)
			if err := a.pprofServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrors <- fmt.Errorf("serve pprof: %w", err)
			}
		}()
	}

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
		var pprofErr error
		if a.pprofServer != nil {
			pprofErr = a.pprofServer.Shutdown(ctx)
			if a.pprofDone != nil {
				select {
				case <-a.pprofDone:
				case <-ctx.Done():
					pprofErr = errors.Join(pprofErr, ctx.Err())
				}
			}
		}

		grpcErr := a.GRPCSrv.Stop(ctx)
		producerErr := a.producer.Close()
		redisErr := a.redisClient.Close()
		a.storage.Close()
		a.shutdownErr = errors.Join(pprofErr, relayErr, grpcErr, producerErr, redisErr)
	})
	return a.shutdownErr
}

func pprofHandler(name string) gin.HandlerFunc {
	handler := pprof.Handler(name)
	return func(ctx *gin.Context) {
		handler.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

func SetupPProfHandlers(engine *gin.Engine) {
	engine.GET("/debug/pprof/", func(ctx *gin.Context) { pprof.Index(ctx.Writer, ctx.Request) })
	engine.GET("/debug/pprof/cmdline", func(ctx *gin.Context) { pprof.Cmdline(ctx.Writer, ctx.Request) })
	engine.GET("/debug/pprof/profile", func(ctx *gin.Context) { pprof.Profile(ctx.Writer, ctx.Request) })
	engine.Any("/debug/pprof/symbol", func(ctx *gin.Context) { pprof.Symbol(ctx.Writer, ctx.Request) })
	engine.GET("/debug/pprof/trace", func(ctx *gin.Context) { pprof.Trace(ctx.Writer, ctx.Request) })
	engine.GET("/debug/pprof/block", pprofHandler("block"))
	engine.GET("/debug/pprof/goroutine", pprofHandler("goroutine"))
	engine.GET("/debug/pprof/heap", pprofHandler("heap"))
	engine.GET("/debug/pprof/threadcreate", pprofHandler("threadcreate"))
}
