package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/config"
)

func TestNewBuildsHTTPServer(t *testing.T) {
	cfg := validConfig()
	application, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	require.Equal(t, "127.0.0.1:18085", application.server.Addr)
	require.Equal(t, cfg.HTTP.ReadTimeout, application.server.ReadTimeout)
	require.Equal(t, cfg.HTTP.ReadTimeout, application.server.ReadHeaderTimeout)
	require.Equal(t, cfg.HTTP.WriteTimeout, application.server.WriteTimeout)
	require.Equal(t, cfg.HTTP.IdleTimeout, application.server.IdleTimeout)
	require.Equal(t, cfg.HTTP.ShutdownTimeout, application.ShutdownTimeout())
	require.NotNil(t, application.server.Handler)
	require.NotNil(t, application.userClient)
	require.NotNil(t, application.productClient)
	require.NotNil(t, application.cartClient)
	require.NotNil(t, application.orderClient)
	require.NotNil(t, application.notificationClient)

	require.NoError(t, application.Shutdown(context.Background()))
	require.NoError(t, application.Shutdown(context.Background()))
}

func TestNewRejectsNilConfig(t *testing.T) {
	application, err := New(nil, nil)
	require.Error(t, err)
	require.Nil(t, application)
}

func TestRunRejectsUninitializedApp(t *testing.T) {
	var application *App
	require.Error(t, application.Run())
}

func validConfig() *config.Config {
	return &config.Config{
		HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            18085,
			ReadTimeout:     time.Second,
			WriteTimeout:    2 * time.Second,
			IdleTimeout:     3 * time.Second,
			ShutdownTimeout: 4 * time.Second,
			RequestTimeout:  time.Second,
		},
		UserService: config.UserServiceConfig{
			Address: "127.0.0.1:19090", RetryCount: 1, Timeout: time.Second,
		},
		ProductService: config.ProductServiceConfig{
			Address: "127.0.0.1:19091", InternalToken: "internal-token", RetryCount: 1, Timeout: time.Second,
		},
		CartService: config.CartServiceConfig{
			Address: "127.0.0.1:19093", RetryCount: 1, Timeout: time.Second,
		},
		OrderService: config.OrderServiceConfig{
			Address: "127.0.0.1:19094", RetryCount: 1, Timeout: time.Second,
		},
		NotificationService: config.NotificationServiceConfig{
			Address: "127.0.0.1:19095", RetryCount: 1, Timeout: time.Second,
		},
	}
}
