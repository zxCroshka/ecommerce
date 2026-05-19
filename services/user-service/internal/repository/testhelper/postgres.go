package testhelper

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDB - тестовая база данных
type TestDB struct {
	Pool      *pgxpool.Pool
	Container testcontainers.Container
	DBName    string
}

// SetupTestPostgres - поднимает тестовый PostgreSQL контейнер
func SetupTestPostgres(t *testing.T) *TestDB {
	ctx := context.Background()

	// Запрос на создание контейнера
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	// Получаем порт
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	// Строка подключения
	dsn := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())

	// Подключаемся
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)

	config.MaxConns = 5
	config.MinConns = 1

	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)

	// Проверяем подключение
	err = pool.Ping(ctx)
	require.NoError(t, err)

	// Накатываем миграции
	err = runMigrations(ctx, pool)
	require.NoError(t, err)

	// Очистка после тестов
	t.Cleanup(func() {
		pool.Close()
		_ = container.Terminate(ctx)
	})

	return &TestDB{
		Pool:      pool,
		Container: container,
		DBName:    "testdb",
	}
}

// runMigrations - применяет миграции (упрощенная версия)
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Создаем схему если нужно
	_, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS userservice`)
	if err != nil {
		return err
	}

	// Создаем таблицу users
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS userservice.users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash BYTEA NOT NULL,
			name VARCHAR(255) NOT NULL,
			is_admin BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	return err
}

// TruncateTables - очищает таблицы между тестами
func (tdb *TestDB) TruncateTables(ctx context.Context) error {
	_, err := tdb.Pool.Exec(ctx, `TRUNCATE TABLE userservice.users RESTART IDENTITY CASCADE`)
	return err
}

// Alternative: Использование существующего PostgreSQL для тестов (без Docker)
// Для CI/CD или локальной разработки
func SetupTestPostgresExisting(t *testing.T) *TestDB {
	// Используем отдельную тестовую БД в вашем существующем PostgreSQL
	dsn := "postgres://postgres:postgres@localhost:5432/ecommerce_test?sslmode=disable"

	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)

	ctx := context.Background()

	// Создаем тестовую БД если её нет
	_, err = pool.Exec(ctx, `CREATE DATABASE IF NOT EXISTS ecommerce_test`)
	if err != nil {
		// Игнорируем ошибку, если БД уже существует
		_ = err
	}

	// Переключаемся на тестовую БД
	pool.Close()

	testDSN := "postgres://postgres:postgres@localhost:5432/ecommerce_test?sslmode=disable"
	config, _ = pgxpool.ParseConfig(testDSN)
	pool, err = pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)

	// Накатываем миграции
	err = runMigrations(ctx, pool)
	require.NoError(t, err)

	t.Cleanup(func() {
		// Очищаем тестовую БД после тестов
		_,_ =pool.Exec(ctx, `DROP SCHEMA userservice CASCADE`)
		pool.Close()
	})

	return &TestDB{
		Pool:   pool,
		DBName: "ecommerce_test",
	}
}
