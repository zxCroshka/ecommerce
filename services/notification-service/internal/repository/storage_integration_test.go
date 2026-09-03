//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/repository"
)

func setupNotificationPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
				"POSTGRES_DB":       "testdb",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(45 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://testuser:testpass@%s:%s/testdb?sslmode=disable",
		host,
		port.Port(),
	))
	require.NoError(t, err)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	require.NoError(t, applyNotificationMigration(ctx, pool))
	t.Cleanup(func() {
		pool.Close()
		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		require.NoError(t, container.Terminate(terminateCtx))
	})
	return pool
}

func applyNotificationMigration(ctx context.Context, pool *pgxpool.Pool) error {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("resolve Notification integration test path")
	}
	migrationPath := filepath.Join(
		filepath.Dir(currentFile), "..", "..", "migrations", "000001_init.up.sql",
	)
	content, err := os.ReadFile(filepath.Clean(migrationPath))
	if err != nil {
		return fmt.Errorf("read Notification migration: %w", err)
	}
	if _, err := pool.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("apply Notification migration: %w", err)
	}
	return nil
}

func TestSaveDeduplicatesAndListsByOwner(t *testing.T) {
	pool := setupNotificationPostgres(t)
	storage := repository.NewForTests(pool)
	eventID := uuid.New()
	input := domain.NewNotification{
		EventID: eventID,
		UserID:  42,
		Type:    domain.TypeWelcome,
		Title:   "Welcome",
		Body:    "Hello",
	}

	created, err := storage.Save(context.Background(), input)
	require.NoError(t, err)
	require.True(t, created)
	created, err = storage.Save(context.Background(), input)
	require.NoError(t, err)
	require.False(t, created)

	owned, total, err := storage.ListForUser(context.Background(), 42, 10, 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, owned, 1)
	require.Equal(t, eventID, owned[0].EventID)

	foreign, total, err := storage.ListForUser(context.Background(), 7, 10, 0)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, foreign)

	var count int
	require.NoError(t, pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*) FROM notificationservice.notifications WHERE event_id=$1`,
		eventID,
	).Scan(&count))
	require.Equal(t, 1, count)
}

func TestMarkAsReadRequiresOwnerAndIsIdempotent(t *testing.T) {
	pool := setupNotificationPostgres(t)
	storage := repository.NewForTests(pool)
	created, err := storage.Save(context.Background(), domain.NewNotification{
		EventID: uuid.New(),
		UserID:  42,
		Type:    domain.TypeOrderCreated,
		Title:   "Order",
		Body:    "Created",
	})
	require.NoError(t, err)
	require.True(t, created)
	values, _, err := storage.ListForUser(context.Background(), 42, 10, 0)
	require.NoError(t, err)
	require.Len(t, values, 1)

	_, err = storage.MarkAsRead(context.Background(), values[0].ID, 7)
	require.ErrorIs(t, err, domain.ErrNotificationNotFound)

	first, err := storage.MarkAsRead(context.Background(), values[0].ID, 42)
	require.NoError(t, err)
	require.NotNil(t, first.ReadAt)
	second, err := storage.MarkAsRead(context.Background(), values[0].ID, 42)
	require.NoError(t, err)
	require.NotNil(t, second.ReadAt)
	require.True(t, first.ReadAt.Equal(*second.ReadAt))
}
