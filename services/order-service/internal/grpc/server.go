package grpc

import (
	"context"
	"errors"

	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/service"
	orderservicev1 "github.com/zxCroshka/ecommerce/shared/orderservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrderService interface {
	CreateOrder(context.Context, string) (*service.CreateResult, error)
	GetOrder(context.Context, int64) (*domain.Order, error)
	ListOrders(context.Context, int, int) ([]*domain.Order, int64, error)
}

type Server struct {
	orders           OrderService
	defaultListLimit int
	maxListLimit     int
	orderservicev1.UnimplementedOrdersServer
}

func Register(
	grpcServer *grpc.Server,
	orders OrderService,
	defaultListLimit, maxListLimit int,
) {
	orderservicev1.RegisterOrdersServer(grpcServer, &Server{
		orders:           orders,
		defaultListLimit: defaultListLimit,
		maxListLimit:     maxListLimit,
	})
}

func (s *Server) CreateOrder(
	ctx context.Context,
	request *orderservicev1.CreateOrderRequest,
) (*orderservicev1.CreateOrderResponse, error) {
	if request == nil || request.GetIdempotencyKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}
	result, err := s.orders.CreateOrder(ctx, request.GetIdempotencyKey())
	if err != nil {
		return nil, mapError(err)
	}
	if result == nil || result.Order == nil {
		return nil, status.Error(codes.Internal, "Order Service returned an empty result")
	}
	return &orderservicev1.CreateOrderResponse{
		Order:   orderToProto(result.Order),
		Created: result.Created,
	}, nil
}

func (s *Server) GetOrder(
	ctx context.Context,
	request *orderservicev1.GetOrderRequest,
) (*orderservicev1.GetOrderResponse, error) {
	if request == nil || request.GetOrderId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "order_id must be positive")
	}
	order, err := s.orders.GetOrder(ctx, request.GetOrderId())
	if err != nil {
		return nil, mapError(err)
	}
	return &orderservicev1.GetOrderResponse{Order: orderToProto(order)}, nil
}

func (s *Server) ListOrders(
	ctx context.Context,
	request *orderservicev1.ListOrdersRequest,
) (*orderservicev1.ListOrdersResponse, error) {
	if request == nil || request.GetLimit() < 0 || request.GetOffset() < 0 ||
		int(request.GetLimit()) > s.maxListLimit {
		return nil, status.Error(codes.InvalidArgument, "invalid pagination")
	}
	limit := int(request.GetLimit())
	if limit == 0 {
		limit = s.defaultListLimit
	}
	orders, total, err := s.orders.ListOrders(ctx, limit, int(request.GetOffset()))
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]*orderservicev1.Order, 0, len(orders))
	for _, order := range orders {
		result = append(result, orderToProto(order))
	}
	return &orderservicev1.ListOrdersResponse{
		Orders: result,
		Total:  total,
		Limit:  int32(limit),
		Offset: request.GetOffset(),
	}, nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		return status.Error(codes.Unauthenticated, "authentication required")
	case errors.Is(err, domain.ErrInvalidOrder),
		errors.Is(err, domain.ErrInvalidIdempotency),
		errors.Is(err, domain.ErrAmountOverflow):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrOrderNotFound):
		return status.Error(codes.NotFound, "order not found")
	case errors.Is(err, domain.ErrCartEmpty),
		errors.Is(err, domain.ErrProductUnavailable),
		errors.Is(err, domain.ErrInsufficientStock),
		errors.Is(err, domain.ErrOrderFailed):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrWorkflowLeaseLost),
		errors.Is(err, domain.ErrInvalidTransition):
		return status.Error(codes.Aborted, "order workflow changed concurrently")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request timed out")
	case errors.Is(err, domain.ErrDownstream),
		errors.Is(err, domain.ErrCompensationPending):
		return status.Error(codes.Unavailable, "order workflow will be retried")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func orderToProto(order *domain.Order) *orderservicev1.Order {
	if order == nil {
		return nil
	}
	items := make([]*orderservicev1.OrderItem, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, &orderservicev1.OrderItem{
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			UnitPrice:   item.UnitPrice,
			Quantity:    item.Quantity,
			LineTotal:   item.LineTotal,
		})
	}
	statusValue := orderservicev1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	switch order.Status {
	case domain.StatusPending:
		statusValue = orderservicev1.OrderStatus_ORDER_STATUS_PENDING
	case domain.StatusConfirmed:
		statusValue = orderservicev1.OrderStatus_ORDER_STATUS_CONFIRMED
	case domain.StatusFailed:
		statusValue = orderservicev1.OrderStatus_ORDER_STATUS_FAILED
	}
	return &orderservicev1.Order{
		Id:             order.ID,
		Status:         statusValue,
		TotalAmount:    order.TotalAmount,
		Currency:       order.Currency,
		IdempotencyKey: order.IdempotencyKey,
		CartRevision:   order.CartRevision,
		Items:          items,
		CreatedAt:      timestamppb.New(order.CreatedAt),
		UpdatedAt:      timestamppb.New(order.UpdatedAt),
	}
}
