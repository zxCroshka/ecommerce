package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Setenv("APP_POSTGRES_PASSWORD", "notification-test-password")
	t.Setenv("APP_KAFKA_BROKERS", "kafka.test:29092")
	t.Setenv("APP_USER_SERVICE_ADDRESS", "user.test:9090")

	configuration, err := Load("../../config/config.yaml")
	require.NoError(t, err)
	require.Equal(t, 9095, configuration.GRPC.Port)
	require.Equal(t, "notification-test-password", configuration.Postgres.Password)
	require.Equal(t, []string{"kafka.test:29092"}, configuration.Kafka.Brokers)
	require.Equal(t, "notification-service-v1", configuration.Kafka.GroupID)
	require.Equal(t, []string{"user.registered", "order.created"}, configuration.Kafka.Topics)
	require.Equal(t, "user.test:9090", configuration.UserService.Address)
	require.Equal(t, 3*time.Second, configuration.Kafka.HandlerTimeout)
	require.Equal(t, 5, configuration.Kafka.MaxRetries)
}

func TestValidateRejectsMissingConsumerGroup(t *testing.T) {
	configuration, err := Load("../../config/config.yaml")
	require.NoError(t, err)
	configuration.Kafka.GroupID = ""
	require.Error(t, configuration.Validate())
}
