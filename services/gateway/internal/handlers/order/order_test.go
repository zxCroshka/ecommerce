package order

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type orderServiceStub struct {
	create func(context.Context, string, string) (*domain.CreateOrderResult, error)
	get    func(context.Context, string, int64) (*domain.Order, error)
	list   func(context.Context, string, int32, int32) (*domain.OrderList, error)
}

func (s *orderServiceStub) CreateOrder(ctx context.Context, token, key string) (*domain.CreateOrderResult, error) {
	return s.create(ctx, token, key)
}

func (s *orderServiceStub) GetOrder(ctx context.Context, token string, orderID int64) (*domain.Order, error) {
	return s.get(ctx, token, orderID)
}

func (s *orderServiceStub) ListOrders(ctx context.Context, token string, limit, offset int32) (*domain.OrderList, error) {
	return s.list(ctx, token, limit, offset)
}

func authenticatedOrderRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity:    domain.Identity{UserID: 42, Role: "customer"},
			AccessToken: "access-token",
		})
		ctx.Next()
	})
	return router
}

func testOrder(status domain.OrderStatus) *domain.Order {
	now := time.Now().UTC()
	return &domain.Order{
		ID:             1,
		Status:         status,
		TotalAmount:    250,
		Currency:       "USD",
		IdempotencyKey: "checkout-1",
		CartRevision:   7,
		Items: []domain.OrderItem{{
			ProductID:   1,
			ProductName: "product",
			UnitPrice:   125,
			Quantity:    2,
			LineTotal:   250,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCreateOrderRequiresValidIdempotencyKey(t *testing.T) {
	called := false
	service := &orderServiceStub{create: func(context.Context, string, string) (*domain.CreateOrderResult, error) {
		called = true
		return nil, nil
	}}
	router := authenticatedOrderRouter()
	router.POST("/orders", New(service).CreateOrder)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/orders", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, called)
}

func TestCreateOrderForwardsBearerAndIdempotencyKey(t *testing.T) {
	service := &orderServiceStub{create: func(_ context.Context, token, key string) (*domain.CreateOrderResult, error) {
		require.Equal(t, "access-token", token)
		require.Equal(t, "checkout-1", key)
		return &domain.CreateOrderResult{Order: testOrder(domain.OrderStatusConfirmed), Created: true}, nil
	}}
	router := authenticatedOrderRouter()
	router.POST("/orders", New(service).CreateOrder)
	request := httptest.NewRequest(http.MethodPost, "/orders", nil)
	request.Header.Set("Idempotency-Key", "checkout-1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"status":"confirmed"`)
}

func TestCreateOrderReturnsAcceptedForPendingWorkflow(t *testing.T) {
	service := &orderServiceStub{create: func(context.Context, string, string) (*domain.CreateOrderResult, error) {
		return &domain.CreateOrderResult{Order: testOrder(domain.OrderStatusPending), Created: false}, nil
	}}
	router := authenticatedOrderRouter()
	router.POST("/orders", New(service).CreateOrder)
	request := httptest.NewRequest(http.MethodPost, "/orders", nil)
	request.Header.Set("Idempotency-Key", "checkout-1")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusAccepted, recorder.Code)
}

func TestListOrdersConvertsPageToOffset(t *testing.T) {
	service := &orderServiceStub{list: func(_ context.Context, token string, limit, offset int32) (*domain.OrderList, error) {
		require.Equal(t, "access-token", token)
		require.Equal(t, int32(10), limit)
		require.Equal(t, int32(20), offset)
		return &domain.OrderList{Orders: []*domain.Order{testOrder(domain.OrderStatusConfirmed)}, Total: 21, Limit: limit, Offset: offset}, nil
	}}
	router := authenticatedOrderRouter()
	router.GET("/orders", New(service).ListOrders)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders?page=3&page_size=10", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"total_pages":3`)
}

func TestGetOrderRejectsInvalidID(t *testing.T) {
	called := false
	service := &orderServiceStub{get: func(context.Context, string, int64) (*domain.Order, error) {
		called = true
		return nil, nil
	}}
	router := authenticatedOrderRouter()
	router.GET("/orders/:id", New(service).GetOrder)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/orders/not-an-id", nil))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, called)
}
