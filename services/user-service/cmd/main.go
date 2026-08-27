package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/zxCroshka/ecommerce/services/user-service/app"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/config"
	kaf "github.com/zxCroshka/ecommerce/services/user-service/internal/kafka"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg, err := config.LoadConfig("./config/config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	log := setupLogger(cfg.Logging)
	log.Info("Starting user-service", "environment", cfg.Service.Environment)

	kafkaProducer, err := kaf.NewProducer(cfg.Kafka.Brokers)
	if err != nil {
		log.Error("Failed to create Kafka producer", "error", err)
		os.Exit(1)
	}
	defer kafkaProducer.Close()

	postgresURL := cfg.Postgres.GetPostgresURL()

	log.Info(
		"starting application",
		slog.String("service", cfg.Service.Name),
		slog.String("environment", cfg.Service.Environment),
		slog.Group("http",
			slog.String("address", cfg.HTTP.Address()),
		),
		slog.Group("grpc",
			slog.String("address", cfg.GRPC.Address()),
		),
		slog.Group("postgres",
			slog.String("host", cfg.Postgres.Host),
			slog.Int("port", cfg.Postgres.Port),
			slog.String("user", cfg.Postgres.User),
			slog.String("database", cfg.Postgres.Database),
			slog.String("sslmode", cfg.Postgres.SSLMode),
		),
		slog.Group("redis",
			slog.String("address", cfg.Redis.Address()),
			slog.Int("db", cfg.Redis.DB),
		),
		slog.Group("kafka",
			slog.Any("brokers", cfg.Kafka.Brokers),
			slog.String("user_registered_topic", cfg.Kafka.Topic.UserRegistered),
		),
		slog.Group("jwt",
			slog.Duration("access_ttl", cfg.JWT.AccessTTL),
			slog.Duration("refresh_ttl", cfg.JWT.RefreshTTL),
		),
		slog.Group("logging",
			slog.String("level", cfg.Logging.Level),
			slog.String("format", cfg.Logging.Format),
		),
		slog.Bool("pprof_enabled", cfg.Pprof.Enabled),
	)
	application := app.New(
		ctx,
		log,
		cfg.GRPC.Port,
		cfg.HTTP.Port,
		kafkaProducer,
		postgresURL,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
		cfg.Redis.Host,
		cfg.Redis.Port,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.JWT.Secret,
		cfg.Pprof.Enabled,
	)
	application.Start(ctx)
	log.Info("application stopped")
}

func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = slog.LevelInfo
	}

	options := &slog.HandlerOptions{Level: level}
	if cfg.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}
