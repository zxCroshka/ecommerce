//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/repository"
)

func setupOrderPostgres(t *testing.T) *pgxpool.Pool {
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
	config.MaxConns = 25
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	require.NoError(t, applyOrderMigrations(ctx, pool))
	t.Cleanup(func() {
		pool.Close()
		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		require.NoError(t, container.Terminate(terminateCtx))
	})
	return pool
}

func applyOrderMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("resolve Order integration test path")
	}
	migrationsDirectory := filepath.Join(filepath.Dir(currentFile), "..", "..", "migrations")
	for _, name := range []string{"000001_init.up.sql", "000002_outbox.up.sql"} {
		content, err := os.ReadFile(filepath.Clean(filepath.Join(migrationsDirectory, name)))
		if err != nil {
			return fmt.Errorf("read Order migration %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply Order migration %s: %w", name, err)
		}
	}
	return nil
}

func orderInput(key string) domain.NewOrder {
	return domain.NewOrder{
		UserID:         42,
		TotalAmount:    600,
		Currency:       "USD",
		IdempotencyKey: key,
		CartRevision:   7,
		Items: []domain.Item{
			{ProductID: 1, ProductName: "first", UnitPrice: 125, Quantity: 2, LineTotal: 250},
			{ProductID: 2, ProductName: "second", UnitPrice: 350, Quantity: 1, LineTotal: 350},
		},
	}
}

func truncateOrders(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE TABLE
		orderservice.order_items, orderservice.orders, orderservice.outbox_events
		RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

func TestConcurrentCreatePendingUsesDatabaseIdempotency(t *testing.T) {
	pool := setupOrderPostgres(t)
	storage, err := repository.NewForTests(pool, "order.created")
	require.NoError(t, err)
	truncateOrders(t, pool)

	const workers = 20
	ids := make(chan int64, workers)
	createdResults := make(chan bool, workers)
	errorsByWorker := make(chan error, workers)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	waitGroup.Add(workers)
	for range workers {
		go func() {
			defer waitGroup.Done()
			<-start
			order, created, createErr := storage.CreatePending(
				context.Background(),
				orderInput("concurrent-key"),
				uuid.New(),
			)
			if createErr == nil {
				ids <- order.ID
				createdResults <- created
			}
			errorsByWorker <- createErr
		}()
	}
	close(start)
	waitGroup.Wait()
	close(ids)
	close(createdResults)
	close(errorsByWorker)

	for workerErr := range errorsByWorker {
		require.NoError(t, workerErr)
	}
	var firstID int64
	for id := range ids {
		if firstID == 0 {
			firstID = id
		}
		require.Equal(t, firstID, id)
	}
	createdCount := 0
	for created := range createdResults {
		if created {
			createdCount++
		}
	}
	require.Equal(t, 1, createdCount)

	var orderCount, itemCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM orderservice.orders`).Scan(&orderCount))
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM orderservice.order_items`).Scan(&itemCount))
	require.Equal(t, 1, orderCount)
	require.Equal(t, 2, itemCount)
}

func TestConfirmAndOutboxCommitAtomically(t *testing.T) {
	pool := setupOrderPostgres(t)
	storage, err := repository.NewForTests(pool, "order.created")
	require.NoError(t, err)
	truncateOrders(t, pool)
	owner := uuid.New()
	order, created, err := storage.CreatePending(context.Background(), orderInput("atomic-confirm"), owner)
	require.NoError(t, err)
	require.True(t, created)

	_, err = pool.Exec(context.Background(), `
		CREATE FUNCTION orderservice.reject_outbox_insert() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'forced outbox insert failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_outbox_insert
		BEFORE INSERT ON orderservice.outbox_events
		FOR EACH ROW EXECUTE FUNCTION orderservice.reject_outbox_insert();
	`)
	require.NoError(t, err)
	transitioned, err := storage.ConfirmWithOutbox(context.Background(), order, owner)
	require.Error(t, err)
	require.False(t, transitioned)

	var statusValue string
	var outboxCount int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT status FROM orderservice.orders WHERE id=$1`, order.ID).Scan(&statusValue))
	require.Equal(t, "pending", statusValue, "status update must roll back when outbox insert fails")
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM orderservice.outbox_events`).Scan(&outboxCount))
	require.Zero(t, outboxCount)

	_, err = pool.Exec(context.Background(), `
		DROP TRIGGER reject_outbox_insert ON orderservice.outbox_events;
		DROP FUNCTION orderservice.reject_outbox_insert();
	`)
	require.NoError(t, err)
	transitioned, err = storage.ConfirmWithOutbox(context.Background(), order, owner)
	require.NoError(t, err)
	require.True(t, transitioned)

	var eventType, aggregateID, payloadOrderID string
	var eventVersion int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT
		event_type, aggregate_id, version, payload->>'order_id'
		FROM orderservice.outbox_events`).Scan(
		&eventType,
		&aggregateID,
		&eventVersion,
		&payloadOrderID,
	))
	require.Equal(t, "order.created", eventType)
	require.Equal(t, fmt.Sprint(order.ID), aggregateID)
	require.Equal(t, 1, eventVersion)
	require.Equal(t, fmt.Sprint(order.ID), payloadOrderID)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT status FROM orderservice.orders WHERE id=$1`, order.ID).Scan(&statusValue))
	require.Equal(t, "confirmed", statusValue)

	transitioned, err = storage.ConfirmWithOutbox(context.Background(), order, owner)
	require.NoError(t, err)
	require.False(t, transitioned)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM orderservice.outbox_events`).Scan(&outboxCount))
	require.Equal(t, 1, outboxCount, "idempotent confirm retry must not duplicate order.created")
}
