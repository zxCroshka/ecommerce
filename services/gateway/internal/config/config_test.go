package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("APP_USER_SERVICE_ADDRESS", "user-service.test:9090")
	t.Setenv("APP_PRODUCT_SERVICE_ADDRESS", "product-service.test:9091")
	t.Setenv("APP_PRODUCT_SERVICE_INTERNAL_TOKEN", "test-internal-token")
	t.Setenv("APP_CART_SERVICE_ADDRESS", "cart-service.test:9093")

	cfg, err := LoadConfig("../../config/config.yaml")
	require.NoError(t, err)
	require.Equal(t, 8085, cfg.HTTP.Port)
	require.Equal(t, "0.0.0.0:8085", cfg.HTTP.Address())
	require.Equal(t, 5*time.Second, cfg.HTTP.ReadTimeout)
	require.Equal(t, 10*time.Second, cfg.HTTP.WriteTimeout)
	require.Equal(t, 60*time.Second, cfg.HTTP.IdleTimeout)
	require.Equal(t, 10*time.Second, cfg.HTTP.ShutdownTimeout)
	require.Equal(t, "user-service.test:9090", cfg.UserService.Address)
	require.Equal(t, "product-service.test:9091", cfg.ProductService.Address)
	require.Equal(t, "test-internal-token", cfg.ProductService.InternalToken)
	require.Equal(t, "cart-service.test:9093", cfg.CartService.Address)
	require.Equal(t, 2, cfg.UserService.RetryCount)
	require.Equal(t, 2*time.Second, cfg.UserService.Timeout)
	require.Equal(t, 2*time.Second, cfg.ProductService.Timeout)
	require.Equal(t, 2*time.Second, cfg.CartService.Timeout)
}
