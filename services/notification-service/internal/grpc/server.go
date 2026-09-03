package grpc

import (
	"context"
	"errors"

	"github.com/zxCroshka/ecommerce/services/notification-service/internal/domain"
	notificationservicev1 "github.com/zxCroshka/ecommerce/shared/notificationservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type NotificationService interface {
	ListNotifications(context.Context, int, int) ([]*domain.Notification, int64, error)
	MarkAsRead(context.Context, int64) (*domain.Notification, error)
}

type Server struct {
	notifications    NotificationService
	defaultListLimit int
	maxListLimit     int
	notificationservicev1.UnimplementedNotificationsServer
}

func Register(server *grpc.Server, notifications NotificationService, defaultListLimit, maxListLimit int) {
	notificationservicev1.RegisterNotificationsServer(server, &Server{
		notifications: notifications, defaultListLimit: defaultListLimit, maxListLimit: maxListLimit,
	})
}

func (s *Server) ListNotifications(
	ctx context.Context,
	request *notificationservicev1.ListNotificationsRequest,
) (*notificationservicev1.ListNotificationsResponse, error) {
	if request == nil || request.GetLimit() < 0 || request.GetOffset() < 0 || int(request.GetLimit()) > s.maxListLimit {
		return nil, status.Error(codes.InvalidArgument, "invalid pagination")
	}
	limit := int(request.GetLimit())
	if limit == 0 {
		limit = s.defaultListLimit
	}
	notifications, total, err := s.notifications.ListNotifications(ctx, limit, int(request.GetOffset()))
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]*notificationservicev1.Notification, 0, len(notifications))
	for _, notification := range notifications {
		result = append(result, notificationToProto(notification))
	}
	return &notificationservicev1.ListNotificationsResponse{
		Notifications: result,
		Total:         total,
		Limit:         int32(limit),
		Offset:        request.GetOffset(),
	}, nil
}

func (s *Server) MarkAsRead(
	ctx context.Context,
	request *notificationservicev1.MarkAsReadRequest,
) (*notificationservicev1.MarkAsReadResponse, error) {
	if request == nil || request.GetNotificationId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "notification_id must be positive")
	}
	notification, err := s.notifications.MarkAsRead(ctx, request.GetNotificationId())
	if err != nil {
		return nil, mapError(err)
	}
	if notification == nil {
		return nil, status.Error(codes.Internal, "Notification Service returned an empty result")
	}
	return &notificationservicev1.MarkAsReadResponse{Notification: notificationToProto(notification)}, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, "authentication required")
	case errors.Is(err, domain.ErrInvalidNotification):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrNotificationNotFound):
		return status.Error(codes.NotFound, "notification not found")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timed out")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func notificationToProto(notification *domain.Notification) *notificationservicev1.Notification {
	if notification == nil {
		return nil
	}
	result := &notificationservicev1.Notification{
		Id: notification.ID, Type: notification.Type, Title: notification.Title, Body: notification.Body,
		CreatedAt: timestamppb.New(notification.CreatedAt),
	}
	if notification.ReadAt != nil {
		result.ReadAt = timestamppb.New(*notification.ReadAt)
	}
	return result
}
