package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/db"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/product"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/reservations"
)

type Storage struct {
	pool         *pgxpool.Pool
	products     *product.Storage
	reservations *reservations.Storage
}

func NewForTests(ctx context.Context, pool *pgxpool.Pool) (*Storage, error) {
	return &Storage{
		pool:         pool,
		products:     product.New(pool),
		reservations: reservations.New(pool),
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
		pool:         pool,
		products:     product.New(pool),
		reservations: reservations.New(pool),
	}, nil
}

func (s *Storage) ReserveStockTX(ctx context.Context, reservationID string, productID int64, quantity int64) (int64, bool, error) {
	var newStock int64
	applied := true
	err := db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		products := s.products.WithTX(tx)
		reservations := s.reservations.WithTX(tx)
		inserted, err := reservations.Insert(ctx, reservationID, productID, quantity)
		if err != nil {
			return err
		}
		if !inserted {
			reservation, err := reservations.GetForUpdate(
				ctx, reservationID, productID,
			)
			if err != nil {
				return err
			}

			if reservation.GetQuantity() != quantity {
				return customerrors.ErrReservationConflict
			}

			if reservation.GetStatus() != domain.StatusReserved {
				return customerrors.ErrReservationConflict
			}

			applied = false
			return nil
		}
		stock, err := products.ReserveStock(ctx, productID, quantity)
		if err != nil {
			return err
		}
		newStock = stock
		return nil
	})
	return newStock, applied, err
}

func (s *Storage) ReleaseStockTX(
	ctx context.Context,
	reservationID string,
	productID int64,
) (int64, bool, error) {
	var newStock int64
	var applied bool

	err := db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		products := s.products.WithTX(tx)
		reservations := s.reservations.WithTX(tx)

		reservation, err := reservations.GetForUpdate(
			ctx,
			reservationID,
			productID,
		)
		if err != nil {
			return err
		}

		if reservation.GetStatus() == domain.StatusReleased {
			return nil
		}

		if reservation.GetStatus() != domain.StatusReserved {
			return customerrors.ErrReservationConflict
		}

		stock, err := products.ReleaseStock(
			ctx,
			productID,
			reservation.GetQuantity(),
		)
		if err != nil {
			return err
		}

		marked, err := reservations.MarkReleased(
			ctx,
			reservationID,
			productID,
		)
		if err != nil {
			return err
		}
		if !marked {
			return customerrors.ErrReservationConflict
		}

		newStock = stock
		applied = true
		return nil
	})

	return newStock, applied, err
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
