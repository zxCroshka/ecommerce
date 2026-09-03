package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service      ServiceConfig    `mapstructure:"service"`
	GRPC         GRPCConfig       `mapstructure:"grpc"`
	Postgres     PostgresConfig   `mapstructure:"postgres"`
	Kafka        KafkaConfig      `mapstructure:"kafka"`
	UserService  DownstreamConfig `mapstructure:"user_service"`
	Notification LimitsConfig     `mapstructure:"notification"`
	Logging      LoggingConfig    `mapstructure:"logging"`
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
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", p.User, p.Password, p.Host, p.Port, p.Database, p.SSLMode)
}

type KafkaConfig struct {
	Brokers         []string      `mapstructure:"brokers"`
	GroupID         string        `mapstructure:"group_id"`
	Topics          []string      `mapstructure:"topics"`
	AutoOffsetReset string        `mapstructure:"auto_offset_reset"`
	PollInterval    time.Duration `mapstructure:"poll_interval"`
	HandlerTimeout  time.Duration `mapstructure:"handler_timeout"`
	MaxRetries      int           `mapstructure:"max_retries"`
	RetryBaseDelay  time.Duration `mapstructure:"retry_base_delay"`
	RetryMaxDelay   time.Duration `mapstructure:"retry_max_delay"`
}

type DownstreamConfig struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type LimitsConfig struct {
	DefaultListLimit int `mapstructure:"default_list_limit"`
	MaxListLimit     int `mapstructure:"max_list_limit"`
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
	v.SetDefault("kafka.auto_offset_reset", "earliest")
	v.SetDefault("kafka.poll_interval", "250ms")
	v.SetDefault("kafka.handler_timeout", "3s")
	v.SetDefault("kafka.max_retries", 5)
	v.SetDefault("kafka.retry_base_delay", "100ms")
	v.SetDefault("kafka.retry_max_delay", "2s")
	v.SetDefault("user_service.timeout", "2s")
	v.SetDefault("notification.default_list_limit", 20)
	v.SetDefault("notification.max_list_limit", 100)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read Notification config: %w", err)
	}
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unmarshal Notification config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("validate Notification config: %w", err)
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
	if len(c.Kafka.Brokers) == 0 || strings.TrimSpace(c.Kafka.GroupID) == "" || len(c.Kafka.Topics) < 2 ||
		(c.Kafka.AutoOffsetReset != "earliest" && c.Kafka.AutoOffsetReset != "latest") ||
		c.Kafka.PollInterval <= 0 || c.Kafka.HandlerTimeout <= 0 || c.Kafka.MaxRetries <= 0 ||
		c.Kafka.RetryBaseDelay <= 0 || c.Kafka.RetryMaxDelay <= 0 || c.Kafka.RetryBaseDelay > c.Kafka.RetryMaxDelay {
		return fmt.Errorf("valid Kafka consumer settings are required")
	}
	if strings.TrimSpace(c.UserService.Address) == "" || c.UserService.Timeout <= 0 {
		return fmt.Errorf("User Service address and timeout are required")
	}
	if c.Notification.DefaultListLimit <= 0 || c.Notification.MaxListLimit < c.Notification.DefaultListLimit {
		return fmt.Errorf("valid notification list limits are required")
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
