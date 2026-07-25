package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/db"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/product"
)

type Storage struct {
	pool     *pgxpool.Pool
	products *product.Storage
}

func NewForTests(ctx context.Context, pool *pgxpool.Pool) (*Storage, error) {
	return &Storage{
		pool:     pool,
		products: product.New(pool),
	}, nil
}

func New(ctx context.Context, storageURL string) (*Storage, error) {
	cfg, err := pgxpool.ParseConfig(storageURL)
	if err != nil {
		slog.Error(fmt.Sprintf("error parsing connection config: %v", err))
		return nil, err

	}
	slog.Info("Database config",
		"host", cfg.ConnConfig.Host,
		"port", cfg.ConnConfig.Port,
		"user", cfg.ConnConfig.User,
		"database", cfg.ConnConfig.Database)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.Error(fmt.Sprintf("error creating connection pool: %v", err))
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		slog.Error("Failed to ping database", "error", err)
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	slog.Info("Database connection established successfully")
	return &Storage{
		pool:     pool,
		products: product.New(pool),
	}, nil
}

func (s *Storage) ReserveStockTX(ctx context.Context, productID int64, quantity int64) (int64, error) {
	var newStock int64
	err := db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		products := s.products.WithTX(tx)
		stock, err := products.ReserveStock(ctx, productID, quantity)
		if err != nil {
			return err
		}
		newStock = stock
		return nil
	})
	return newStock, err
}

func (s *Storage) ReleaseStockTX(ctx context.Context, productID int64, quantity int64) (int64, error) {
	var newStock int64
	err := db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		products := s.products.WithTX(tx)
		stock, err := products.ReleaseStock(ctx, productID, quantity)
		if err != nil {
			return err
		}
		newStock = stock
		return nil
	})
	return newStock, err
}

func (s *Storage) SaveProduct(
	ctx context.Context,
	name, description string,
	price, stock int64,
	category string,
	images []string,
	isActive bool,
) (int64, error) {
	return s.products.Insert(ctx, name, description, price, stock, category, images, isActive, time.Now(), time.Now())
}

func (s *Storage) UpdateProductFields(
	ctx context.Context,
	productID int64,
	patch domain.ProductPatch,
) error {
	return s.products.UpdateProductFields(ctx, productID, patch)
}

func (s *Storage) ListProducts(ctx context.Context, req domain.ProductListRequest) ([]*domain.Product, int64, error) {
	return s.products.ListProducts(ctx, req)
}

func (s *Storage) SoftDelete(ctx context.Context, productID int64) error {
	return s.products.SoftDelete(ctx, productID)
}

func (s *Storage) GetProduct(ctx context.Context, productID int64) (*domain.Product, error) {
	return s.products.GetProduct(ctx, productID)
}
