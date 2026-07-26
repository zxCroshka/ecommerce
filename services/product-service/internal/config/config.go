package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service    ServiceConfig    `mapstructure:"service"`
	HTTP       HTTPConfig       `mapstructure:"http"`
	GRPC       GRPCConfig       `mapstructure:"grpc"`
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Pagination PaginationConfig `mapstructure:"pagination"`
	Jwt        JwtConfig        `mapstructure:"jwt"`
}
type JwtConfig struct {
	Secret string `mapstructure:"secret"`
}

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type HTTPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func (h HTTPConfig) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

type GRPCConfig struct {
	Host          string        `mapstructure:"host"`
	Port          int           `mapstructure:"port"`
	Ttl           time.Duration `mapstructure:"ttl"`
	InternalToken string        `mapstructure:"internal_token"`
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
	Host     string         `mapstructure:"host"`
	Port     int            `mapstructure:"port"`
	Password string         `mapstructure:"password"`
	DB       int            `mapstructure:"db"`
	TTL      RedisTTLConfig `mapstructure:"ttl"`
}

type RedisTTLConfig struct {
	ProductCache      time.Duration `mapstructure:"product_cache"`
	ProductsListCache time.Duration `mapstructure:"products_list_cache"`
}

func (r RedisConfig) Address() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type KafkaConfig struct {
	Brokers []string         `mapstructure:"brokers"`
	Topic   KafkaTopicConfig `mapstructure:"topic"`
}

type KafkaTopicConfig struct {
	ProductUpdated string `mapstructure:"product_updated"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type PaginationConfig struct {
	DefaultLimit int `mapstructure:"default_limit"`
	MaxLimit     int `mapstructure:"max_limit"`
}

func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("APP")

	_ = viper.BindEnv("postgres.password", "APP_POSTGRES_PASSWORD")
	_ = viper.BindEnv("redis.password", "APP_REDIS_PASSWORD")
	_ = viper.BindEnv("kafka.brokers", "APP_KAFKA_BROKERS")
	_ = viper.BindEnv("grpc.internal_token", "APP_GRPC_INTERNAL_TOKEN")

	viper.SetDefault("redis.ttl.product_cache", "5m")
	viper.SetDefault("redis.ttl.products_list_cache", "5m")
	viper.SetDefault("pagination.default_limit", 20)
	viper.SetDefault("pagination.max_limit", 100)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
	// Validate Postgres
	if c.Postgres.Host == "" {
		return fmt.Errorf("postgres.host is required")
	}
	if c.Postgres.Port <= 0 || c.Postgres.Port > 65535 {
		return fmt.Errorf("postgres.port must be between 1 and 65535")
	}
	if c.Postgres.User == "" {
		return fmt.Errorf("postgres.user is required")
	}
	if c.Postgres.Database == "" {
		return fmt.Errorf("postgres.database is required")
	}

	// Validate Redis
	if c.Redis.Host == "" {
		return fmt.Errorf("redis.host is required")
	}
	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		return fmt.Errorf("redis.port must be between 1 and 65535")
	}

	// Validate Kafka
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers cannot be empty")
	}
	if c.Kafka.Topic.ProductUpdated == "" {
		return fmt.Errorf("kafka.topic.product_updated is required")
	}

	if c.Pagination.DefaultLimit <= 0 {
		return fmt.Errorf("pagination.default_limit must be positive")
	}
	if c.Pagination.MaxLimit <= 0 {
		return fmt.Errorf("pagination.max_limit must be positive")
	}
	if c.Pagination.DefaultLimit > c.Pagination.MaxLimit {
		return fmt.Errorf("pagination.default_limit (%d) cannot be greater than max_limit (%d)",
			c.Pagination.DefaultLimit, c.Pagination.MaxLimit)
	}

	// Validate HTTP and gRPC ports
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535")
	}
	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be between 1 and 65535")
	}
	if c.GRPC.InternalToken == "" {
		return fmt.Errorf("grpc.internal_token is required")
	}

	return nil
}
