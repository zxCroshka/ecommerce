package domain

import "github.com/jackc/pgx/v5"

type Status string

const (
	StatusReserved Status = "reserved"
	StatusReleased Status = "released"
)

type Reservation struct {
	reservationID string
	productID     int64
	quantity      int64
	status        Status
}

func NewReservation(
	reservationID string,
	productID int64,
	quantity int64,
	status Status,
) *Reservation {
	return &Reservation{
		reservationID: reservationID,
		productID:     productID,
		quantity:      quantity,
		status:        status,
	}
}

func ReservationFactory() *Reservation {
	return &Reservation{}
}

func (r *Reservation) FromRow(row pgx.Row) error {
	return row.Scan(&r.reservationID, &r.productID, &r.quantity, &r.status)
}

func (r *Reservation) GetQuantity() int64 {
	return r.quantity
}

func (r *Reservation) GetStatus() Status {
	return r.status
}
