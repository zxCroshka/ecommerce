package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/service"
	orderservicev1 "github.com/zxCroshka/ecommerce/shared/orderservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type orderServiceStub struct {
	create func(context.Context, string) (*service.CreateResult, error)
	get    func(context.Context, int64) (*domain.Order, error)
	list   func(context.Context, int, int) ([]*domain.Order, int64, error)
}

func (s *orderServiceStub) CreateOrder(ctx context.Context, key string) (*service.CreateResult, error) {
	return s.create(ctx, key)
}

func (s *orderServiceStub) GetOrder(ctx context.Context, orderID int64) (*domain.Order, error) {
	return s.get(ctx, orderID)
}

func (s *orderServiceStub) ListOrders(ctx context.Context, limit, offset int) ([]*domain.Order, int64, error) {
	return s.list(ctx, limit, offset)
}

func TestCreateOrderMapsDomainResultToProto(t *testing.T) {
	now := time.Now().UTC()
	server := &Server{orders: &orderServiceStub{create: func(_ context.Context, key string) (*service.CreateResult, error) {
		require.Equal(t, "checkout-1", key)
		return &service.CreateResult{Created: true, Order: &domain.Order{
			ID: 1, UserID: 42, Status: domain.StatusConfirmed, TotalAmount: 250,
			Currency: "USD", IdempotencyKey: key, CartRevision: 7,
			Items:     []domain.Item{{ProductID: 9, ProductName: "item", UnitPrice: 125, Quantity: 2, LineTotal: 250}},
			CreatedAt: now, UpdatedAt: now,
		}}, nil
	}}, defaultListLimit: 20, maxListLimit: 100}

	response, err := server.CreateOrder(context.Background(), &orderservicev1.CreateOrderRequest{IdempotencyKey: "checkout-1"})
	require.NoError(t, err)
	require.True(t, response.GetCreated())
	require.Equal(t, orderservicev1.OrderStatus_ORDER_STATUS_CONFIRMED, response.GetOrder().GetStatus())
	require.Equal(t, int64(125), response.GetOrder().GetItems()[0].GetUnitPrice())
}

func TestDomainErrorsMapToStableGRPCCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{name: "auth", err: domain.ErrUnauthenticated, code: codes.Unauthenticated},
		{name: "invalid", err: domain.ErrInvalidIdempotency, code: codes.InvalidArgument},
		{name: "not found", err: domain.ErrOrderNotFound, code: codes.NotFound},
		{name: "stock", err: domain.ErrInsufficientStock, code: codes.FailedPrecondition},
		{name: "workflow", err: domain.ErrWorkflowLeaseLost, code: codes.Aborted},
		{name: "downstream", err: domain.ErrDownstream, code: codes.Unavailable},
		{name: "deadline", err: context.DeadlineExceeded, code: codes.DeadlineExceeded},
		{name: "unknown", err: errors.New("database failure"), code: codes.Internal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.code, status.Code(mapError(test.err)))
		})
	}
}
