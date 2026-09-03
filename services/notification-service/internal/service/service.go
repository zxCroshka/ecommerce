package service

import (
	"context"
	"fmt"

	"github.com/zxCroshka/ecommerce/services/notification-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/domain"
)

type Repository interface {
	ListForUser(context.Context, int64, int, int) ([]*domain.Notification, int64, error)
	MarkAsRead(context.Context, int64, int64) (*domain.Notification, error)
}

type Service struct {
	repository Repository
}

func New(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("notification repository is required")
	}
	return &Service{repository: repository}, nil
}

func (s *Service) ListNotifications(
	ctx context.Context,
	limit, offset int,
) ([]*domain.Notification, int64, error) {
	identity, ok := auth.UserIdentityFromContext(ctx)
	if !ok {
		return nil, 0, domain.ErrUnauthenticated
	}
	if limit <= 0 || offset < 0 {
		return nil, 0, domain.ErrInvalidNotification
	}
	return s.repository.ListForUser(ctx, identity.UserID, limit, offset)
}

func (s *Service) MarkAsRead(ctx context.Context, notificationID int64) (*domain.Notification, error) {
	identity, ok := auth.UserIdentityFromContext(ctx)
	if !ok {
		return nil, domain.ErrUnauthenticated
	}
	if notificationID <= 0 {
		return nil, domain.ErrInvalidNotification
	}
	return s.repository.MarkAsRead(ctx, notificationID, identity.UserID)
}
