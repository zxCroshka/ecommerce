package orderservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/grpcerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	orderservicev1 "github.com/zxCroshka/ecommerce/shared/orderservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const authorizationMetadataKey = "authorization"

type Client struct {
	api  orderservicev1.OrdersClient
	conn *grpc.ClientConn
}

type Config struct {
	Address    string
	RetryCount int
	Timeout    time.Duration
}

func New(config Config) (*Client, error) {
	if strings.TrimSpace(config.Address) == "" || config.RetryCount < 0 || config.Timeout <= 0 {
		return nil, fmt.Errorf("invalid Order client config")
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
		return nil, fmt.Errorf("create Order gRPC client: %w", err)
	}
	return &Client{api: orderservicev1.NewOrdersClient(conn), conn: conn}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) CreateOrder(
	ctx context.Context,
	accessToken, idempotencyKey string,
) (*domain.CreateOrderResult, error) {
	response, err := c.api.CreateOrder(
		withBearerToken(ctx, accessToken),
		&orderservicev1.CreateOrderRequest{IdempotencyKey: idempotencyKey},
		grpcretry.Disable(),
	)
	if err != nil {
		return nil, grpcerrors.Map("grpc.OrderClient.CreateOrder", err)
	}
	order, err := orderFromProto(response.GetOrder())
	if err != nil {
		return nil, err
	}
	return &domain.CreateOrderResult{Order: order, Created: response.GetCreated()}, nil
}

func (c *Client) GetOrder(ctx context.Context, accessToken string, orderID int64) (*domain.Order, error) {
	response, err := c.api.GetOrder(
		withBearerToken(ctx, accessToken),
		&orderservicev1.GetOrderRequest{OrderId: orderID},
	)
	if err != nil {
		return nil, grpcerrors.Map("grpc.OrderClient.GetOrder", err)
	}
	return orderFromProto(response.GetOrder())
}

func (c *Client) ListOrders(
	ctx context.Context,
	accessToken string,
	limit, offset int32,
) (*domain.OrderList, error) {
	response, err := c.api.ListOrders(
		withBearerToken(ctx, accessToken),
		&orderservicev1.ListOrdersRequest{Limit: limit, Offset: offset},
	)
	if err != nil {
		return nil, grpcerrors.Map("grpc.OrderClient.ListOrders", err)
	}
	orders := make([]*domain.Order, 0, len(response.GetOrders()))
	for _, protoOrder := range response.GetOrders() {
		order, err := orderFromProto(protoOrder)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return &domain.OrderList{
		Orders: orders,
		Total:  response.GetTotal(),
		Limit:  response.GetLimit(),
		Offset: response.GetOffset(),
	}, nil
}

func orderFromProto(order *orderservicev1.Order) (*domain.Order, error) {
	if order == nil || order.GetId() <= 0 || order.GetTotalAmount() < 0 ||
		len(order.GetCurrency()) != 3 || order.GetCreatedAt() == nil || order.GetUpdatedAt() == nil {
		return nil, fmt.Errorf("invalid Order Service response")
	}
	statusValue := domain.OrderStatus("")
	switch order.GetStatus() {
	case orderservicev1.OrderStatus_ORDER_STATUS_PENDING:
		statusValue = domain.OrderStatusPending
	case orderservicev1.OrderStatus_ORDER_STATUS_CONFIRMED:
		statusValue = domain.OrderStatusConfirmed
	case orderservicev1.OrderStatus_ORDER_STATUS_FAILED:
		statusValue = domain.OrderStatusFailed
	default:
		return nil, fmt.Errorf("invalid Order Service status")
	}
	items := make([]domain.OrderItem, 0, len(order.GetItems()))
	for _, item := range order.GetItems() {
		if item == nil || item.GetProductId() <= 0 || item.GetQuantity() <= 0 ||
			item.GetUnitPrice() < 0 || item.GetLineTotal() < 0 {
			return nil, fmt.Errorf("invalid Order Service item")
		}
		items = append(items, domain.OrderItem{
			ProductID:   item.GetProductId(),
			ProductName: item.GetProductName(),
			UnitPrice:   item.GetUnitPrice(),
			Quantity:    item.GetQuantity(),
			LineTotal:   item.GetLineTotal(),
		})
	}
	return &domain.Order{
		ID:             order.GetId(),
		Status:         statusValue,
		TotalAmount:    order.GetTotalAmount(),
		Currency:       order.GetCurrency(),
		IdempotencyKey: order.GetIdempotencyKey(),
		CartRevision:   order.GetCartRevision(),
		Items:          items,
		CreatedAt:      order.GetCreatedAt().AsTime(),
		UpdatedAt:      order.GetUpdatedAt().AsTime(),
	}, nil
}

func withBearerToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(
		ctx,
		authorizationMetadataKey,
		"Bearer "+strings.TrimSpace(token),
	)
}
