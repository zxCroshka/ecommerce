package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/events"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/db"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/product"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/reservations"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

type Storage struct {
	pool         *pgxpool.Pool
	products     *product.Storage
	reservations *reservations.Storage
	outbox       *outbox.PostgresStore
	topic        string
}

func NewForTests(ctx context.Context, pool *pgxpool.Pool) (*Storage, error) {
	outboxStore, err := outbox.NewPostgresStore(pool, "productservice")
	if err != nil {
		return nil, err
	}
	return &Storage{
		pool:         pool,
		products:     product.New(pool),
		reservations: reservations.New(pool),
		outbox:       outboxStore,
		topic:        events.ProductUpdatedType,
	}, nil
}

func New(ctx context.Context, storageURL, productUpdatedTopic string) (*Storage, error) {
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
		pool.Close()
		slog.Error("Failed to ping database", "error", err)
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	outboxStore, err := outbox.NewPostgresStore(pool, "productservice")
	if err != nil {
		pool.Close()
		return nil, err
	}
	slog.Info("Database connection established successfully")
	return &Storage{
		pool:         pool,
		products:     product.New(pool),
		reservations: reservations.New(pool),
		outbox:       outboxStore,
		topic:        productUpdatedTopic,
	}, nil
}

func (s *Storage) OutboxStore() outbox.Store {
	return s.outbox
}

func (s *Storage) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
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
		updatedProduct, err := products.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		event, err := events.NewProductUpdated(
			s.topic,
			events.OperationStockReserved,
			updatedProduct,
			reservationID,
			quantity,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return s.outbox.Insert(ctx, tx, event)
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
			if errors.Is(err, customerrors.ErrReservationNotFound) {
				// Compensation deliberately releases every order item. An item that
				// was never reserved is already in the desired released state.
				return nil
			}
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
		updatedProduct, err := products.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		event, err := events.NewProductUpdated(
			s.topic,
			events.OperationStockReleased,
			updatedProduct,
			reservationID,
			reservation.GetQuantity(),
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return s.outbox.Insert(ctx, tx, event)
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
	return db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		products := s.products.WithTX(tx)
		if err := products.UpdateProductFields(ctx, productID, patch); err != nil {
			return err
		}
		updatedProduct, err := products.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		event, err := events.NewProductUpdated(
			s.topic,
			events.OperationUpdated,
			updatedProduct,
			"",
			0,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return s.outbox.Insert(ctx, tx, event)
	})
}

func (s *Storage) ListProducts(ctx context.Context, req domain.ProductListRequest) ([]*domain.Product, int64, error) {
	return s.products.ListProducts(ctx, req)
}

func (s *Storage) SoftDelete(ctx context.Context, productID int64) error {
	return db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		products := s.products.WithTX(tx)
		if err := products.SoftDelete(ctx, productID); err != nil {
			return err
		}
		updatedProduct, err := products.GetProduct(ctx, productID)
		if err != nil {
			return err
		}
		event, err := events.NewProductUpdated(
			s.topic,
			events.OperationSoftDeleted,
			updatedProduct,
			"",
			0,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return s.outbox.Insert(ctx, tx, event)
	})
}

func (s *Storage) GetProduct(ctx context.Context, productID int64) (*domain.Product, error) {
	return s.products.GetProduct(ctx, productID)
}
