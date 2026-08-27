package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig_EnvironmentOverridesNestedValues(t *testing.T) {
	t.Setenv("APP_POSTGRES_HOST", "postgres")
	t.Setenv("APP_REDIS_HOST", "redis")
	t.Setenv("APP_KAFKA_BROKERS", "kafka:29092")
	t.Setenv("APP_JWT_SECRET", "docker-local-secret")
	t.Setenv("APP_PPROF_ENABLED", "true")

	cfg, err := LoadConfig("../../config/config.yaml")

	require.NoError(t, err)
	require.Equal(t, "postgres", cfg.Postgres.Host)
	require.Equal(t, "redis", cfg.Redis.Host)
	require.Equal(t, []string{"kafka:29092"}, cfg.Kafka.Brokers)
	require.Equal(t, "docker-local-secret", cfg.JWT.Secret)
	require.True(t, cfg.Pprof.Enabled)
}
