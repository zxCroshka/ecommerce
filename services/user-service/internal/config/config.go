package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Service              ServiceConfig  `mapstructure:"service"`
	HTTP                 HTTPConfig     `mapstructure:"http"`
	GRPC                 GRPCConfig     `mapstructure:"grpc"`
	Postgres             PostgresConfig `mapstructure:"postgres"`
	Redis                RedisConfig    `mapstructure:"redis"`
	Kafka                KafkaConfig    `mapstructure:"kafka"`
	KafkaUI              KafkaUIConfig  `mapstructure:"kafka_ui"`
	TokenTTL             time.Duration  `mapstructure:"token_ttl"`
	AccessTokenExpireIn  time.Duration  `mapstructure:"access_token_expire_in"`
	RefreshTokenExpireIn time.Duration  `mapstructure:"refresh_token_expire_in"`
	JwtSecret            string         `mapstructure:"jwt_secret"`
}

type ServiceConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

type HTTPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
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

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
}

type KafkaUIConfig struct {
	URL string `mapstructure:"url"`
}

func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	viper.AutomaticEnv()

	viper.SetEnvPrefix("APP")

	_ = viper.BindEnv("postgres.password", "APP_POSTGRES_PASSWORD")
	_ = viper.BindEnv("redis.password", "APP_REDIS_PASSWORD")
	_ = viper.BindEnv("kafka.brokers", "APP_KAFKA_BROKERS")
	_ = viper.BindEnv("jwt_secret", "APP_JWT_SECRET")
	viper.SetDefault("jwt_secret", "change-me-in-production")

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
	if c.TokenTTL <= 0 {
		return fmt.Errorf("token_ttl must be positive")
	}
	if c.AccessTokenExpireIn <= 0 {
		return fmt.Errorf("access_token_expire_in must be positive")
	}
	if c.RefreshTokenExpireIn <= 0 {
		return fmt.Errorf("refresh_token_expire_in must be positive")
	}

	if c.AccessTokenExpireIn > c.RefreshTokenExpireIn {
		return fmt.Errorf("access_token_expire_in (%v) cannot be greater than refresh_token_expire_in (%v)",
			c.AccessTokenExpireIn, c.RefreshTokenExpireIn)
	}

	return nil
}
