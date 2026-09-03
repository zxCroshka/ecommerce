package domain

import "time"

type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusFailed    Status = "failed"
)

func (s Status) CanTransitionTo(next Status) bool {
	return s == StatusPending && (next == StatusConfirmed || next == StatusFailed)
}

type Item struct {
	ProductID   int64
	ProductName string
	UnitPrice   int64
	Quantity    int64
	LineTotal   int64
}

type Order struct {
	ID             int64
	UserID         int64
	Status         Status
	TotalAmount    int64
	Currency       string
	IdempotencyKey string
	CartRevision   int64
	Items          []Item
	FailureReason  string
	ProcessingBy   string
	ProcessingAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type NewOrder struct {
	UserID         int64
	TotalAmount    int64
	Currency       string
	IdempotencyKey string
	CartRevision   int64
	Items          []Item
}
