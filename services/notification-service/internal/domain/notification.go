package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	TypeWelcome      = "welcome"
	TypeOrderCreated = "order_created"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
	ErrUnauthenticated      = errors.New("authentication required")
	ErrInvalidNotification  = errors.New("invalid notification")
)

type Notification struct {
	ID        int64
	EventID   uuid.UUID
	UserID    int64
	Type      string
	Title     string
	Body      string
	CreatedAt time.Time
	ReadAt    *time.Time
}

type NewNotification struct {
	EventID uuid.UUID
	UserID  int64
	Type    string
	Title   string
	Body    string
}
