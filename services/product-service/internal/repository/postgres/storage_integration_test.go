//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	productrepository "github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres"
)

type postgresTestDB struct {
	pool      *pgxpool.Pool
	container testcontainers.Container
}

func setupPostgres(t *testing.T) *postgresTestDB {
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

	dsn := fmt.Sprintf(
		"postgres://testuser:testpass@%s:%s/testdb?sslmode=disable",
		host,
		port.Port(),
	)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.MaxConns = 25

	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	require.NoError(t, applyProductMigrations(ctx, pool))

	t.Cleanup(func() {
		pool.Close()

		terminateCtx, terminateCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer terminateCancel()
		require.NoError(t, container.Terminate(terminateCtx))
	})

	return &postgresTestDB{
		pool:      pool,
		container: container,
	}
}

func applyProductMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("resolve integration test path")
	}

	migrationsDir := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "migrations")
	for _, migrationName := range []string{
		"000001_init.up.sql",
		"000002_reservations.up.sql",
		"000003_outbox.up.sql",
	} {
		migrationPath := filepath.Join(migrationsDir, migrationName)
		migration, err := os.ReadFile(filepath.Clean(migrationPath))
		if err != nil {
			return fmt.Errorf("read product migration %s: %w", migrationName, err)
		}

		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			return fmt.Errorf("apply product migration %s: %w", migrationName, err)
		}
	}
	return nil
}

func truncateProducts(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(
		context.Background(),
		`TRUNCATE TABLE productservice.products, productservice.outbox_events RESTART IDENTITY CASCADE`,
	)
	require.NoError(t, err)
}

func saveTestProduct(
	t *testing.T,
	ctx context.Context,
	storage *productrepository.Storage,
	stock int64,
	active bool,
) int64 {
	t.Helper()
	id, err := storage.SaveProduct(
		ctx,
		"Integration product",
		"Product created by an integration test",
		15_000,
		stock,
		"testing",
		[]string{"https://example.com/product.jpg"},
		active,
	)
	require.NoError(t, err)
	return id
}

func TestStoragePostgresIntegration(t *testing.T) {
	testDB := setupPostgres(t)
	storage, err := productrepository.NewForTests(context.Background(), testDB.pool)
	require.NoError(t, err)

	t.Run("product lifecycle", func(t *testing.T) {
		truncateProducts(t, testDB.pool)
		ctx := context.Background()
		productID := saveTestProduct(t, ctx, storage, 10, true)

		product, err := storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, "Integration product", product.Name)
		require.Equal(t, int64(15_000), product.Price)
		require.Equal(t, int64(10), product.Stock)
		require.True(t, product.IsActive)

		newPrice := int64(20_000)
		newName := "Updated integration product"
		err = storage.UpdateProductFields(ctx, productID, domain.ProductPatch{
			Name:      &newName,
			Price:     &newPrice,
			Images:    []string{},
			ImagesSet: true,
		})
		require.NoError(t, err)

		product, err = storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, newName, product.Name)
		require.Equal(t, newPrice, product.Price)
		require.Empty(t, product.Images)

		var updateEvents int
		require.NoError(t, testDB.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM productservice.outbox_events
			WHERE aggregate_id=$1 AND event_type='product.updated'
		`, fmt.Sprint(productID)).Scan(&updateEvents))
		require.Equal(t, 1, updateEvents)

		active := true
		products, total, err := storage.ListProducts(ctx, domain.ProductListRequest{
			Filter: domain.ProductFilter{IsActive: &active},
			Sort:   domain.SortByCreatedAt,
			Order:  domain.SortDesc,
			Limit:  20,
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, products, 1)

		require.NoError(t, storage.SoftDelete(ctx, productID))

		product, err = storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.False(t, product.IsActive)
		require.NoError(t, testDB.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM productservice.outbox_events
			WHERE aggregate_id=$1 AND event_type='product.updated'
		`, fmt.Sprint(productID)).Scan(&updateEvents))
		require.Equal(t, 2, updateEvents)

		products, total, err = storage.ListProducts(ctx, domain.ProductListRequest{
			Filter: domain.ProductFilter{IsActive: &active},
			Limit:  20,
		})
		require.NoError(t, err)
		require.Zero(t, total)
		require.Empty(t, products)
	})

	t.Run("stock operations and business errors", func(t *testing.T) {
		truncateProducts(t, testDB.pool)
		ctx := context.Background()

		productID := saveTestProduct(t, ctx, storage, 5, true)

		stock, applied, err := storage.ReserveStockTX(ctx, "reservation-1", productID, 2)
		require.NoError(t, err)
		require.True(t, applied)
		require.Equal(t, int64(3), stock)

		_, applied, err = storage.ReserveStockTX(ctx, "reservation-1", productID, 2)
		require.NoError(t, err)
		require.False(t, applied)

		var eventCount int
		require.NoError(t, testDB.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM productservice.outbox_events
			WHERE aggregate_id=$1
		`, fmt.Sprint(productID)).Scan(&eventCount))
		require.Equal(t, 1, eventCount, "an idempotent reserve retry must not duplicate the durable event")

		product, err := storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, int64(3), product.Stock, "repeated reserve must not change stock")

		_, _, err = storage.ReserveStockTX(ctx, "reservation-1", productID, 3)
		require.ErrorIs(t, err, customerrors.ErrReservationConflict)

		stock, applied, err = storage.ReleaseStockTX(ctx, "reservation-1", productID)
		require.NoError(t, err)
		require.True(t, applied)
		require.Equal(t, int64(5), stock)

		_, applied, err = storage.ReleaseStockTX(ctx, "reservation-1", productID)
		require.NoError(t, err)
		require.False(t, applied)
		require.NoError(t, testDB.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM productservice.outbox_events
			WHERE aggregate_id=$1
		`, fmt.Sprint(productID)).Scan(&eventCount))
		require.Equal(t, 2, eventCount, "reserve and release each create one durable event")

		product, err = storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, int64(5), product.Stock, "repeated release must not change stock")

		_, _, err = storage.ReserveStockTX(ctx, "reservation-insufficient", productID, 6)
		require.ErrorIs(t, err, customerrors.ErrInsufficientStock)

		product, err = storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, int64(5), product.Stock, "failed reserve must not change stock")

		inactiveProductID := saveTestProduct(t, ctx, storage, 5, false)
		_, _, err = storage.ReserveStockTX(
			ctx, "reservation-inactive", inactiveProductID, 1,
		)
		require.ErrorIs(t, err, customerrors.ErrProductInactive)

		_, applied, err = storage.ReleaseStockTX(ctx, "unknown-reservation", productID)
		require.NoError(t, err)
		require.False(t, applied, "release-before-reserve is an idempotent compensation no-op")
	})

	t.Run("concurrent reserve never makes stock negative", func(t *testing.T) {
		truncateProducts(t, testDB.pool)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		const (
			initialStock = int64(10)
			requests     = 20
		)
		productID := saveTestProduct(t, ctx, storage, initialStock, true)

		results := make(chan error, requests)
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Add(requests)

		for requestID := range requests {
			go func() {
				defer workers.Done()
				<-start
				_, _, reserveErr := storage.ReserveStockTX(
					ctx,
					fmt.Sprintf("concurrent-reservation-%d", requestID),
					productID,
					1,
				)
				results <- reserveErr
			}()
		}

		close(start)
		workers.Wait()
		close(results)

		var successful, rejected int
		for reserveErr := range results {
			switch {
			case reserveErr == nil:
				successful++
			case errors.Is(reserveErr, customerrors.ErrInsufficientStock):
				rejected++
			default:
				require.NoError(t, reserveErr)
			}
		}

		require.Equal(t, int(initialStock), successful)
		require.Equal(t, requests-int(initialStock), rejected)

		product, err := storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Zero(t, product.Stock)
	})

	t.Run("concurrent retry applies reservation once", func(t *testing.T) {
		truncateProducts(t, testDB.pool)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		const requests = 20
		productID := saveTestProduct(t, ctx, storage, 10, true)

		type reserveResult struct {
			applied bool
			err     error
		}

		results := make(chan reserveResult, requests)
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Add(requests)

		for range requests {
			go func() {
				defer workers.Done()
				<-start
				_, applied, reserveErr := storage.ReserveStockTX(
					ctx,
					"same-reservation",
					productID,
					1,
				)
				results <- reserveResult{applied: applied, err: reserveErr}
			}()
		}

		close(start)
		workers.Wait()
		close(results)

		var appliedCount int
		for result := range results {
			require.NoError(t, result.err)
			if result.applied {
				appliedCount++
			}
		}

		require.Equal(t, 1, appliedCount)

		product, err := storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, int64(9), product.Stock)
	})

	t.Run("business update rolls back when outbox insert fails", func(t *testing.T) {
		truncateProducts(t, testDB.pool)
		ctx := context.Background()
		productID := saveTestProduct(t, ctx, storage, 10, true)

		_, err := testDB.pool.Exec(ctx, `DROP TABLE productservice.outbox_events`)
		require.NoError(t, err)
		changedName := "must roll back"
		err = storage.UpdateProductFields(ctx, productID, domain.ProductPatch{Name: &changedName})
		require.Error(t, err)

		product, err := storage.GetProduct(ctx, productID)
		require.NoError(t, err)
		require.Equal(t, "Integration product", product.Name)
	})
}
