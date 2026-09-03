package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/domain"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

const (
	UserRegisteredType    = "user.registered"
	UserRegisteredVersion = 1
	OrderCreatedType      = "order.created"
	OrderCreatedVersion   = 1
)

type NotificationWriter interface {
	Save(context.Context, domain.NewNotification) (bool, error)
}

type Handler struct {
	repository NotificationWriter
}

func NewHandler(repository NotificationWriter) (*Handler, error) {
	if repository == nil {
		return nil, fmt.Errorf("notification repository is required")
	}
	return &Handler{repository: repository}, nil
}

type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

func IsPermanent(err error) bool {
	var permanent *PermanentError
	return errors.As(err, &permanent)
}

type userRegisteredPayload struct {
	UserID int64  `json:"user_id"`
	Name   string `json:"name"`
}

type orderCreatedPayload struct {
	OrderID  int64  `json:"order_id"`
	UserID   int64  `json:"user_id"`
	Currency string `json:"currency"`
}

func (h *Handler) Handle(ctx context.Context, message []byte) error {
	var envelope outbox.Envelope
	if err := json.Unmarshal(message, &envelope); err != nil {
		return permanent(fmt.Errorf("decode event envelope: %w", err))
	}
	eventID, err := uuid.Parse(envelope.EventID)
	if err != nil {
		return permanent(fmt.Errorf("invalid event_id: %w", err))
	}

	var notification domain.NewNotification
	switch envelope.EventType {
	case UserRegisteredType:
		if envelope.Version != UserRegisteredVersion {
			return permanent(fmt.Errorf("unsupported %s version %d", envelope.EventType, envelope.Version))
		}
		var payload userRegisteredPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil || payload.UserID <= 0 {
			return permanent(fmt.Errorf("invalid %s payload", envelope.EventType))
		}
		name := strings.TrimSpace(payload.Name)
		if name == "" {
			name = "пользователь"
		}
		notification = domain.NewNotification{
			EventID: eventID,
			UserID:  payload.UserID,
			Type:    domain.TypeWelcome,
			Title:   "Добро пожаловать",
			Body:    fmt.Sprintf("Добро пожаловать, %s!", name),
		}

	case OrderCreatedType:
		if envelope.Version != OrderCreatedVersion {
			return permanent(fmt.Errorf("unsupported %s version %d", envelope.EventType, envelope.Version))
		}
		var payload orderCreatedPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil || payload.UserID <= 0 || payload.OrderID <= 0 {
			return permanent(fmt.Errorf("invalid %s payload", envelope.EventType))
		}
		notification = domain.NewNotification{
			EventID: eventID,
			UserID:  payload.UserID,
			Type:    domain.TypeOrderCreated,
			Title:   "Заказ создан",
			Body:    fmt.Sprintf("Заказ №%d успешно создан", payload.OrderID),
		}

	default:
		return permanent(fmt.Errorf("unsupported event type %q", envelope.EventType))
	}

	_, err = h.repository.Save(ctx, notification)
	return err
}

func permanent(err error) error {
	return &PermanentError{Err: err}
}
