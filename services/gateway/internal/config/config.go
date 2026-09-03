package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service             ServiceConfig             `mapstructure:"service"`
	HTTP                HTTPConfig                `mapstructure:"http"`
	UserService         UserServiceConfig         `mapstructure:"user_service"`
	ProductService      ProductServiceConfig      `mapstructure:"product_service"`
	CartService         CartServiceConfig         `mapstructure:"cart_service"`
	OrderService        OrderServiceConfig        `mapstructure:"order_service"`
	NotificationService NotificationServiceConfig `mapstructure:"notification_service"`
	Logging             LoggingConfig             `mapstructure:"logging"`
}

type HTTPConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	RequestTimeout  time.Duration `mapstructure:"request_timeout"`
}

func (h HTTPConfig) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type UserServiceConfig struct {
	Address    string        `mapstructure:"address"`
	RetryCount int           `mapstructure:"retry_count"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type ProductServiceConfig struct {
	Address       string        `mapstructure:"address"`
	InternalToken string        `mapstructure:"internal_token"`
	RetryCount    int           `mapstructure:"retry_count"`
	Timeout       time.Duration `mapstructure:"timeout"`
}

type CartServiceConfig struct {
	Address    string        `mapstructure:"address"`
	RetryCount int           `mapstructure:"retry_count"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type OrderServiceConfig struct {
	Address    string        `mapstructure:"address"`
	RetryCount int           `mapstructure:"retry_count"`
	Timeout    time.Duration `mapstructure:"timeout"`
}

type NotificationServiceConfig struct {
	Address    string        `mapstructure:"address"`
	RetryCount int           `mapstructure:"retry_count"`
	Timeout    time.Duration `mapstructure:"timeout"`
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

	_ = v.BindEnv("user_service.address", "APP_USER_SERVICE_ADDRESS")
	_ = v.BindEnv("product_service.address", "APP_PRODUCT_SERVICE_ADDRESS")
	_ = v.BindEnv("product_service.internal_token", "APP_PRODUCT_SERVICE_INTERNAL_TOKEN")
	_ = v.BindEnv("cart_service.address", "APP_CART_SERVICE_ADDRESS")
	_ = v.BindEnv("order_service.address", "APP_ORDER_SERVICE_ADDRESS")
	_ = v.BindEnv("notification_service.address", "APP_NOTIFICATION_SERVICE_ADDRESS")
	v.SetDefault("http.read_timeout", "5s")
	v.SetDefault("http.write_timeout", "10s")
	v.SetDefault("http.idle_timeout", "60s")
	v.SetDefault("http.shutdown_timeout", "10s")
	v.SetDefault("http.request_timeout", "8s")
	v.SetDefault("user_service.retry_count", 2)
	v.SetDefault("user_service.timeout", "2s")
	v.SetDefault("product_service.retry_count", 2)
	v.SetDefault("product_service.timeout", "2s")
	v.SetDefault("cart_service.retry_count", 2)
	v.SetDefault("cart_service.timeout", "2s")
	v.SetDefault("order_service.retry_count", 2)
	v.SetDefault("order_service.timeout", "2s")
	v.SetDefault("notification_service.retry_count", 2)
	v.SetDefault("notification_service.timeout", "2s")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

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
	if strings.TrimSpace(c.HTTP.Host) == "" {
		return fmt.Errorf("http.host is required")
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		return fmt.Errorf("http.port must be between 1 and 65535")
	}
	if c.HTTP.ReadTimeout <= 0 || c.HTTP.WriteTimeout <= 0 ||
		c.HTTP.IdleTimeout <= 0 || c.HTTP.ShutdownTimeout <= 0 || c.HTTP.RequestTimeout <= 0 {
		return fmt.Errorf("http timeouts must be positive")
	}
	if c.HTTP.RequestTimeout > c.HTTP.WriteTimeout {
		return fmt.Errorf("http.request_timeout cannot exceed http.write_timeout")
	}
	if strings.TrimSpace(c.UserService.Address) == "" {
		return fmt.Errorf("user_service.address is required")
	}
	if strings.TrimSpace(c.ProductService.Address) == "" {
		return fmt.Errorf("product_service.address is required")
	}
	if strings.TrimSpace(c.ProductService.InternalToken) == "" {
		return fmt.Errorf("product_service.internal_token is required")
	}
	if strings.TrimSpace(c.CartService.Address) == "" {
		return fmt.Errorf("cart_service.address is required")
	}
	if strings.TrimSpace(c.OrderService.Address) == "" {
		return fmt.Errorf("order_service.address is required")
	}
	if strings.TrimSpace(c.NotificationService.Address) == "" {
		return fmt.Errorf("notification_service.address is required")
	}
	if c.UserService.RetryCount < 0 || c.ProductService.RetryCount < 0 ||
		c.CartService.RetryCount < 0 || c.OrderService.RetryCount < 0 || c.NotificationService.RetryCount < 0 {
		return fmt.Errorf("service retry_count cannot be negative")
	}
	if c.UserService.Timeout <= 0 || c.ProductService.Timeout <= 0 ||
		c.CartService.Timeout <= 0 || c.OrderService.Timeout <= 0 || c.NotificationService.Timeout <= 0 {
		return fmt.Errorf("service timeout must be positive")
	}
	if strings.TrimSpace(c.Logging.Level) == "" {
		return fmt.Errorf("logging.level is required")
	}
	format := strings.ToLower(strings.TrimSpace(c.Logging.Format))
	if format != "json" && format != "text" {
		return fmt.Errorf("logging.format must be json or text")
	}
	return nil
}
