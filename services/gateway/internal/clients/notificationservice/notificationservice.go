package notificationservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/grpcerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	notificationservicev1 "github.com/zxCroshka/ecommerce/shared/notificationservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	api  notificationservicev1.NotificationsClient
	conn *grpc.ClientConn
}

type Config struct {
	Address    string
	RetryCount int
	Timeout    time.Duration
}

func New(config Config) (*Client, error) {
	if strings.TrimSpace(config.Address) == "" || config.RetryCount < 0 || config.Timeout <= 0 {
		return nil, fmt.Errorf("invalid Notification client config")
	}
	retryOptions := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.Unavailable),
		grpcretry.WithMax(uint(config.RetryCount)),
		grpcretry.WithPerRetryTimeout(config.Timeout),
	}
	conn, err := grpc.NewClient(
		config.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOptions...)),
	)
	if err != nil {
		return nil, fmt.Errorf("create Notification gRPC client: %w", err)
	}
	return &Client{api: notificationservicev1.NewNotificationsClient(conn), conn: conn}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) ListNotifications(
	ctx context.Context,
	accessToken string,
	limit, offset int32,
) (*domain.NotificationList, error) {
	response, err := c.api.ListNotifications(
		withBearer(ctx, accessToken),
		&notificationservicev1.ListNotificationsRequest{Limit: limit, Offset: offset},
	)
	if err != nil {
		return nil, grpcerrors.Map("grpc.NotificationClient.ListNotifications", err)
	}
	notifications := make([]*domain.Notification, 0, len(response.GetNotifications()))
	for _, value := range response.GetNotifications() {
		notification, err := notificationFromProto(value)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return &domain.NotificationList{
		Notifications: notifications,
		Total:         response.GetTotal(), Limit: response.GetLimit(), Offset: response.GetOffset(),
	}, nil
}

func (c *Client) MarkAsRead(
	ctx context.Context,
	accessToken string,
	notificationID int64,
) (*domain.Notification, error) {
	response, err := c.api.MarkAsRead(
		withBearer(ctx, accessToken),
		&notificationservicev1.MarkAsReadRequest{NotificationId: notificationID},
		grpcretry.Disable(),
	)
	if err != nil {
		return nil, grpcerrors.Map("grpc.NotificationClient.MarkAsRead", err)
	}
	return notificationFromProto(response.GetNotification())
}

func notificationFromProto(value *notificationservicev1.Notification) (*domain.Notification, error) {
	if value == nil || value.GetId() <= 0 || strings.TrimSpace(value.GetType()) == "" ||
		strings.TrimSpace(value.GetTitle()) == "" || strings.TrimSpace(value.GetBody()) == "" || value.GetCreatedAt() == nil {
		return nil, fmt.Errorf("invalid Notification Service response")
	}
	result := &domain.Notification{
		ID: value.GetId(), Type: value.GetType(), Title: value.GetTitle(), Body: value.GetBody(),
		CreatedAt: value.GetCreatedAt().AsTime(),
	}
	if value.GetReadAt() != nil {
		readAt := value.GetReadAt().AsTime()
		result.ReadAt = &readAt
	}
	return result, nil
}

func withBearer(ctx context.Context, accessToken string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+strings.TrimSpace(accessToken))
}
