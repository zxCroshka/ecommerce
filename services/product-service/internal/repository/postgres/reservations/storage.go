package reservations

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/postgres/db"
)

type Storage struct {
	db db.DBTX
}

func New(db db.DBTX) *Storage {
	return &Storage{db: db}
}

func (s *Storage) WithTX(tx pgx.Tx) *Storage {
	return &Storage{db: tx}
}

func (s *Storage) Insert(
	ctx context.Context,
	reservationID string,
	productID int64,
	quantity int64,
) (bool, error) {
	const op = "repository.reservations.Insert"
	sql := `INSERT INTO productservice.stock_reservations(
		reservation_id,
		product_id,
		quantity,
		status
	)
	VALUES($1,$2,$3,$4)
	ON CONFLICT(reservation_id,product_id) DO NOTHING`
	tag, err := s.db.Exec(ctx, sql, reservationID, productID, quantity, domain.StatusReserved)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return tag.RowsAffected() == 1, nil
}

func (s *Storage) GetForUpdate(ctx context.Context, reservationID string, productID int64) (*domain.Reservation, error) {
	const op = "repository.reservations.GetForUpdate"
	sql := `SELECT reservation_id, product_id, quantity, status
			FROM productservice.stock_reservations
			WHERE reservation_id = $1 AND product_id = $2
			FOR UPDATE`

	row := s.db.QueryRow(ctx, sql, reservationID, productID)
	reservation := domain.ReservationFactory()
	if err := reservation.FromRow(row); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%s: %w", op, customerrors.ErrReservationNotFound)
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return reservation, nil

}

func (s *Storage) MarkReleased(ctx context.Context, reservationID string, productID int64) (bool, error) {
	const op = "repository.reservations.MarkReleased"
	sql := `UPDATE productservice.stock_reservations
			SET status=$3, released_at = NOW()
			WHERE reservation_id=$1 AND product_id=$2 AND status=$4;
	`
	tag, err := s.db.Exec(ctx, sql, reservationID, productID, domain.StatusReleased, domain.StatusReserved)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	return tag.RowsAffected() == 1, nil
}
