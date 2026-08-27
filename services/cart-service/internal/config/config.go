package config

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service        ServiceConfig        `mapstructure:"service"`
	GRPC           GRPCServerConfig     `mapstructure:"grpc"`
	ProductService ProductServiceConfig `mapstructure:"product_service"`
	Cart           CartConfig           `mapstructure:"cart"`
	Redis          RedisConfig          `mapstructure:"redis"`
	Logging        LoggingConfig        `mapstructure:"logging"`
}

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type GRPCServerConfig struct {
	Port int `mapstructure:"port"`
}

type ProductServiceConfig struct {
	Address    string        `mapstructure:"address"`
	RetryCount int           `mapstructure:"retry_count"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type CartConfig struct {
	TTL                time.Duration `mapstructure:"ttl"`
	MaxProductQuantity int64         `mapstructure:"max_product_quantity"`
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

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("cart.ttl", "168h")
	v.SetDefault("cart.max_product_quantity", 99)
	v.SetDefault("product_service.retry_count", 3)
	v.SetDefault("product_service.timeout", "2s")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

func (c Config) Validate() error {
	if c.Service.Name == "" {
		return fmt.Errorf("service.name is required")
	}
	if c.GRPC.Port <= 0 || c.GRPC.Port > 65535 {
		return fmt.Errorf("grpc.port must be between 1 and 65535")
	}
	if c.ProductService.Address == "" {
		return fmt.Errorf("product_service.address is required")
	}
	if c.ProductService.RetryCount < 0 {
		return fmt.Errorf("product_service.retry_count cannot be negative")
	}
	if c.ProductService.Timeout <= 0 {
		return fmt.Errorf("product_service.timeout must be positive")
	}
	if c.Cart.TTL <= 0 {
		return fmt.Errorf("cart.ttl must be positive")
	}
	if c.Cart.MaxProductQuantity <= 0 {
		return fmt.Errorf("cart.max_product_quantity must be positive")
	}
	if c.Redis.Host == "" {
		return fmt.Errorf("redis.host is required")
	}
	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		return fmt.Errorf("redis.port must be between 1 and 65535")
	}
	if c.Redis.DB < 0 {
		return fmt.Errorf("redis.db cannot be negative")
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
