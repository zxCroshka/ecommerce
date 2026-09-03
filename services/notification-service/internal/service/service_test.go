package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/domain"
)

type repositoryStub struct {
	listUserID int64
	markUserID int64
	markID     int64
}

func (s *repositoryStub) ListForUser(_ context.Context, userID int64, _, _ int) ([]*domain.Notification, int64, error) {
	s.listUserID = userID
	return []*domain.Notification{{ID: 1, UserID: userID}}, 1, nil
}

func (s *repositoryStub) MarkAsRead(_ context.Context, notificationID, userID int64) (*domain.Notification, error) {
	s.markID = notificationID
	s.markUserID = userID
	now := time.Now().UTC()
	return &domain.Notification{ID: notificationID, UserID: userID, ReadAt: &now}, nil
}

func TestServiceUsesOnlyAuthenticatedUserIdentity(t *testing.T) {
	repository := &repositoryStub{}
	notifications, err := New(repository)
	require.NoError(t, err)
	ctx := auth.WithUserIdentity(context.Background(), auth.UserIdentity{UserID: 42, Role: "customer"})

	values, total, err := notifications.ListNotifications(ctx, 20, 0)
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Equal(t, int64(1), total)
	require.Equal(t, int64(42), repository.listUserID)

	_, err = notifications.MarkAsRead(ctx, 17)
	require.NoError(t, err)
	require.Equal(t, int64(17), repository.markID)
	require.Equal(t, int64(42), repository.markUserID)
}

func TestServiceRequiresAuthentication(t *testing.T) {
	notifications, err := New(&repositoryStub{})
	require.NoError(t, err)
	_, _, err = notifications.ListNotifications(context.Background(), 20, 0)
	require.ErrorIs(t, err, domain.ErrUnauthenticated)
	_, err = notifications.MarkAsRead(context.Background(), 1)
	require.ErrorIs(t, err, domain.ErrUnauthenticated)
}
