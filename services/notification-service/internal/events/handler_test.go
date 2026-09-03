package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/domain"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

type notificationWriterStub struct {
	mu     sync.Mutex
	seen   map[uuid.UUID]domain.NewNotification
	writes int
	err    error
}

func (s *notificationWriterStub) Save(_ context.Context, notification domain.NewNotification) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return false, s.err
	}
	if _, exists := s.seen[notification.EventID]; exists {
		return false, nil
	}
	s.seen[notification.EventID] = notification
	s.writes++
	return true, nil
}

func eventMessage(t *testing.T, eventType string, version int, payload any) []byte {
	t.Helper()
	event, err := outbox.NewEvent(eventType, eventType, "aggregate", "1", version, payload, time.Now().UTC())
	require.NoError(t, err)
	message, err := event.Message()
	require.NoError(t, err)
	return message
}

func TestHandlerCreatesWelcomeNotificationIdempotently(t *testing.T) {
	repository := &notificationWriterStub{seen: make(map[uuid.UUID]domain.NewNotification)}
	handler, err := NewHandler(repository)
	require.NoError(t, err)
	message := eventMessage(t, UserRegisteredType, UserRegisteredVersion, map[string]any{"user_id": 42, "name": "Анна"})

	require.NoError(t, handler.Handle(context.Background(), message))
	require.NoError(t, handler.Handle(context.Background(), message))
	require.Equal(t, 1, repository.writes)
	for _, notification := range repository.seen {
		require.Equal(t, int64(42), notification.UserID)
		require.Equal(t, domain.TypeWelcome, notification.Type)
		require.Contains(t, notification.Body, "Анна")
	}
}

func TestHandlerCreatesOrderNotification(t *testing.T) {
	repository := &notificationWriterStub{seen: make(map[uuid.UUID]domain.NewNotification)}
	handler, err := NewHandler(repository)
	require.NoError(t, err)
	message := eventMessage(t, OrderCreatedType, OrderCreatedVersion, map[string]any{
		"order_id": 17, "user_id": 42, "total_amount": 600, "currency": "USD",
	})
	require.NoError(t, handler.Handle(context.Background(), message))
	require.Equal(t, 1, repository.writes)
	for _, notification := range repository.seen {
		require.Equal(t, domain.TypeOrderCreated, notification.Type)
		require.Equal(t, "Заказ №17 успешно создан", notification.Body)
	}
}

func TestHandlerRejectsUnknownVersionWithoutWriting(t *testing.T) {
	repository := &notificationWriterStub{seen: make(map[uuid.UUID]domain.NewNotification)}
	handler, err := NewHandler(repository)
	require.NoError(t, err)
	err = handler.Handle(context.Background(), eventMessage(t, OrderCreatedType, 2, map[string]any{
		"order_id": 17, "user_id": 42,
	}))
	require.True(t, IsPermanent(err))
	require.Zero(t, repository.writes)
}
