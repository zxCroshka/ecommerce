package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service  ServiceConfig  `mapstructure:"service"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	KafkaUI  KafkaUIConfig  `mapstructure:"kafka_ui"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	Pprof    PprofConfig    `mapstructure:"pprof"`
	Outbox   OutboxConfig   `mapstructure:"outbox"`
}

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type GRPCConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func (g GRPCConfig) Address() string {
	return fmt.Sprintf("%s:%d", g.Host, g.Port)
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"sslmode"`
}

func (p *PostgresConfig) GetPostgresURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User,
		p.Password,
		p.Host,
		p.Port,
		p.Database,
		p.SSLMode,
	)
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

func (r RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type KafkaConfig struct {
	Brokers []string         `mapstructure:"brokers"`
	Topic   KafkaTopicConfig `mapstructure:"topic"`
}

type KafkaTopicConfig struct {
	UserRegistered string `mapstructure:"user_registered"`
}

type KafkaUIConfig struct {
	URL string `mapstructure:"url"`
}

type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	AccessTTL  time.Duration `mapstructure:"access_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type PprofConfig struct {
	Enabled bool `mapstructure:"enabled"`
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

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	_ = v.BindEnv("postgres.password", "APP_POSTGRES_PASSWORD")
	_ = v.BindEnv("redis.password", "APP_REDIS_PASSWORD")
	_ = v.BindEnv("kafka.brokers", "APP_KAFKA_BROKERS")
	_ = v.BindEnv("jwt.secret", "APP_JWT_SECRET")

	v.SetDefault("jwt.secret", "change-me-in-production")
	v.SetDefault("jwt.access_ttl", "15m")
	v.SetDefault("jwt.refresh_ttl", "168h")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("pprof.enabled", false)
	v.SetDefault("outbox.poll_interval", "500ms")
	v.SetDefault("outbox.publish_timeout", "5s")
	v.SetDefault("outbox.store_timeout", "2s")
	v.SetDefault("outbox.lock_timeout", "30s")
	v.SetDefault("outbox.retry_base_delay", "1s")
	v.SetDefault("outbox.retry_max_delay", "1m")
	v.SetDefault("outbox.batch_size", 50)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
	if c.JWT.AccessTTL <= 0 {
		return fmt.Errorf("jwt.access_ttl must be positive")
	}
	if c.JWT.RefreshTTL <= 0 {
		return fmt.Errorf("jwt.refresh_ttl must be positive")
	}
	if c.JWT.AccessTTL > c.JWT.RefreshTTL {
		return fmt.Errorf("jwt.access_ttl (%v) cannot be greater than jwt.refresh_ttl (%v)",
			c.JWT.AccessTTL, c.JWT.RefreshTTL)
	}
	if c.JWT.Secret == "" || c.JWT.Secret == "change-me-in-production" {
		return fmt.Errorf("jwt.secret must be set and not be default value")
	}

	// Validate Kafka brokers
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers cannot be empty")
	}
	if c.Kafka.Topic.UserRegistered == "" {
		return fmt.Errorf("kafka.topic.user_registered is required")
	}
	if c.Outbox.PollInterval <= 0 || c.Outbox.PublishTimeout <= 0 ||
		c.Outbox.StoreTimeout <= 0 || c.Outbox.LockTimeout <= 0 ||
		c.Outbox.RetryBaseDelay <= 0 || c.Outbox.RetryMaxDelay <= 0 ||
		c.Outbox.BatchSize <= 0 {
		return fmt.Errorf("outbox settings must be positive")
	}
	if c.Outbox.RetryBaseDelay > c.Outbox.RetryMaxDelay {
		return fmt.Errorf("outbox.retry_base_delay cannot exceed retry_max_delay")
	}

	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be between 1 and 65535")
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
