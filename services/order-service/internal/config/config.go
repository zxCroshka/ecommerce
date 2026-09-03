package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service        ServiceConfig         `mapstructure:"service"`
	GRPC           GRPCConfig            `mapstructure:"grpc"`
	Postgres       PostgresConfig        `mapstructure:"postgres"`
	Kafka          KafkaConfig           `mapstructure:"kafka"`
	Outbox         OutboxConfig          `mapstructure:"outbox"`
	UserService    DownstreamConfig      `mapstructure:"user_service"`
	CartService    InternalServiceConfig `mapstructure:"cart_service"`
	ProductService InternalServiceConfig `mapstructure:"product_service"`
	Order          OrderConfig           `mapstructure:"order"`
	Workflow       WorkflowConfig        `mapstructure:"workflow"`
	Recovery       RecoveryConfig        `mapstructure:"recovery"`
	Logging        LoggingConfig         `mapstructure:"logging"`
}

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type GRPCConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"sslmode"`
}

func (p PostgresConfig) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.Database, p.SSLMode,
	)
}

type KafkaConfig struct {
	Brokers []string         `mapstructure:"brokers"`
	Topic   KafkaTopicConfig `mapstructure:"topic"`
}

type KafkaTopicConfig struct {
	OrderCreated string `mapstructure:"order_created"`
}

type OutboxConfig struct {
	PollInterval   time.Duration `mapstructure:"poll_interval"`
	PublishTimeout time.Duration `mapstructure:"publish_timeout"`
	StoreTimeout   time.Duration `mapstructure:"store_timeout"`
	LockTimeout    time.Duration `mapstructure:"lock_timeout"`
	RetryBaseDelay time.Duration `mapstructure:"retry_base_delay"`
	RetryMaxDelay  time.Duration `mapstructure:"retry_max_delay"`
	BatchSize      int           `mapstructure:"batch_size"`
}

type DownstreamConfig struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type InternalServiceConfig struct {
	Address       string        `mapstructure:"address"`
	InternalToken string        `mapstructure:"internal_token"`
	Timeout       time.Duration `mapstructure:"timeout"`
}

type OrderConfig struct {
	Currency             string `mapstructure:"currency"`
	MaxItems             int    `mapstructure:"max_items"`
	MaxIdempotencyLength int    `mapstructure:"max_idempotency_length"`
	DefaultListLimit     int    `mapstructure:"default_list_limit"`
	MaxListLimit         int    `mapstructure:"max_list_limit"`
}

type WorkflowConfig struct {
	LeaseTimeout        time.Duration `mapstructure:"lease_timeout"`
	FinalizeTimeout     time.Duration `mapstructure:"finalize_timeout"`
	CompensationTimeout time.Duration `mapstructure:"compensation_timeout"`
	CartCleanupTimeout  time.Duration `mapstructure:"cart_cleanup_timeout"`
}

type RecoveryConfig struct {
	PollInterval time.Duration `mapstructure:"poll_interval"`
	RecoveryAge  time.Duration `mapstructure:"recovery_age"`
	OrderTimeout time.Duration `mapstructure:"order_timeout"`
	BatchSize    int           `mapstructure:"batch_size"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("postgres.password", "APP_POSTGRES_PASSWORD")
	_ = v.BindEnv("kafka.brokers", "APP_KAFKA_BROKERS")
	_ = v.BindEnv("user_service.address", "APP_USER_SERVICE_ADDRESS")
	_ = v.BindEnv("cart_service.address", "APP_CART_SERVICE_ADDRESS")
	_ = v.BindEnv("cart_service.internal_token", "APP_CART_SERVICE_INTERNAL_TOKEN")
	_ = v.BindEnv("product_service.address", "APP_PRODUCT_SERVICE_ADDRESS")
	_ = v.BindEnv("product_service.internal_token", "APP_PRODUCT_SERVICE_INTERNAL_TOKEN")

	v.SetDefault("outbox.poll_interval", "500ms")
	v.SetDefault("outbox.publish_timeout", "5s")
	v.SetDefault("outbox.store_timeout", "2s")
	v.SetDefault("outbox.lock_timeout", "30s")
	v.SetDefault("outbox.retry_base_delay", "1s")
	v.SetDefault("outbox.retry_max_delay", "1m")
	v.SetDefault("outbox.batch_size", 50)
	v.SetDefault("order.currency", "USD")
	v.SetDefault("order.max_items", 100)
	v.SetDefault("order.max_idempotency_length", 128)
	v.SetDefault("order.default_list_limit", 20)
	v.SetDefault("order.max_list_limit", 100)
	v.SetDefault("workflow.lease_timeout", "30s")
	v.SetDefault("workflow.finalize_timeout", "3s")
	v.SetDefault("workflow.compensation_timeout", "15s")
	v.SetDefault("workflow.cart_cleanup_timeout", "3s")
	v.SetDefault("recovery.poll_interval", "5s")
	v.SetDefault("recovery.recovery_age", "5s")
	v.SetDefault("recovery.order_timeout", "30s")
	v.SetDefault("recovery.batch_size", 20)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("user_service.timeout", "2s")
	v.SetDefault("cart_service.timeout", "2s")
	v.SetDefault("product_service.timeout", "2s")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read Order config: %w", err)
	}
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unmarshal Order config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate Order config: %w", err)
	}
	return &config, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Service.Name) == "" {
		return fmt.Errorf("service.name is required")
	}
	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.Postgres.Host) == "" || c.Postgres.Port <= 0 || c.Postgres.Port > 65535 ||
		strings.TrimSpace(c.Postgres.User) == "" || strings.TrimSpace(c.Postgres.Database) == "" {
		return fmt.Errorf("valid PostgreSQL settings are required")
	}
	if len(c.Kafka.Brokers) == 0 || strings.TrimSpace(c.Kafka.Topic.OrderCreated) == "" {
		return fmt.Errorf("Kafka brokers and order.created topic are required")
	}
	if c.Outbox.PollInterval <= 0 || c.Outbox.PublishTimeout <= 0 || c.Outbox.StoreTimeout <= 0 ||
		c.Outbox.LockTimeout <= 0 || c.Outbox.RetryBaseDelay <= 0 || c.Outbox.RetryMaxDelay <= 0 ||
		c.Outbox.BatchSize <= 0 || c.Outbox.RetryBaseDelay > c.Outbox.RetryMaxDelay {
		return fmt.Errorf("valid outbox settings are required")
	}
	for name, downstream := range map[string]DownstreamConfig{
		"user_service":    c.UserService,
		"cart_service":    {Address: c.CartService.Address, Timeout: c.CartService.Timeout},
		"product_service": {Address: c.ProductService.Address, Timeout: c.ProductService.Timeout},
	} {
		if strings.TrimSpace(downstream.Address) == "" || downstream.Timeout <= 0 {
			return fmt.Errorf("%s address and timeout are required", name)
		}
	}
	if strings.TrimSpace(c.CartService.InternalToken) == "" || strings.TrimSpace(c.ProductService.InternalToken) == "" {
		return fmt.Errorf("Cart and Product internal tokens are required")
	}
	currency := strings.ToUpper(strings.TrimSpace(c.Order.Currency))
	if len(currency) != 3 {
		return fmt.Errorf("order.currency must be a three-letter code")
	}
	if c.Order.MaxItems <= 0 || c.Order.MaxIdempotencyLength <= 0 || c.Order.MaxIdempotencyLength > 128 ||
		c.Order.DefaultListLimit <= 0 || c.Order.MaxListLimit < c.Order.DefaultListLimit {
		return fmt.Errorf("valid Order limits are required")
	}
	if c.Workflow.LeaseTimeout <= 0 || c.Workflow.FinalizeTimeout <= 0 ||
		c.Workflow.CompensationTimeout <= 0 || c.Workflow.CartCleanupTimeout <= 0 {
		return fmt.Errorf("workflow timeouts must be positive")
	}
	if c.Recovery.PollInterval <= 0 || c.Recovery.RecoveryAge <= 0 ||
		c.Recovery.OrderTimeout <= 0 || c.Recovery.BatchSize <= 0 {
		return fmt.Errorf("valid recovery settings are required")
	}
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		return fmt.Errorf("logging.format must be json or text")
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Logging.Level)); err != nil {
		return fmt.Errorf("logging.level is invalid: %w", err)
	}
	return nil
}
