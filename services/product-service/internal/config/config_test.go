package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig_EnvironmentOverridesNestedValues(t *testing.T) {
	t.Setenv("APP_POSTGRES_HOST", "postgres")
	t.Setenv("APP_REDIS_HOST", "redis")
	t.Setenv("APP_KAFKA_BROKERS", "kafka:29092")
	t.Setenv("APP_USER_SERVICE_ADDRESS", "user-service:9090")
	t.Setenv("APP_GRPC_INTERNAL_TOKEN", "docker-internal-token")

	cfg, err := LoadConfig("../../config/config.yaml")

	require.NoError(t, err)
	require.Equal(t, "postgres", cfg.Postgres.Host)
	require.Equal(t, "redis", cfg.Redis.Host)
	require.Equal(t, []string{"kafka:29092"}, cfg.Kafka.Brokers)
	require.Equal(t, "user-service:9090", cfg.UserService.Address)
	require.Equal(t, "docker-internal-token", cfg.GRPC.InternalToken)
}
