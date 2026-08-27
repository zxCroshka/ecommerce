package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := LoadConfig("../../config/config.yaml")
	require.NoError(t, err)

	assert.Equal(t, 9093, cfg.GRPC.Port)
	assert.Equal(t, "127.0.0.1:9091", cfg.ProductService.Address)
	assert.Equal(t, 2*time.Second, cfg.ProductService.Timeout)
	assert.Equal(t, 7*24*time.Hour, cfg.Cart.TTL)
	assert.EqualValues(t, 99, cfg.Cart.MaxProductQuantity)
	assert.Equal(t, 6379, cfg.Redis.Port)
	assert.Equal(t, 2, cfg.Redis.DB)
}
