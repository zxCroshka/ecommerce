package cart

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type fakeCartService struct {
	getCartFn               func(context.Context, int64) (*domain.Cart, error)
	addProductFn            func(context.Context, int64, int64, int64) (*domain.AddProductResult, error)
	removeProductFn         func(context.Context, int64, int64) error
	changeProductQuantityFn func(context.Context, int64, int64, int64) error
}

func (f *fakeCartService) GetCart(ctx context.Context, userID int64) (*domain.Cart, error) {
	return f.getCartFn(ctx, userID)
}

func (f *fakeCartService) AddProduct(ctx context.Context, userID, productID, quantity int64) (*domain.AddProductResult, error) {
	return f.addProductFn(ctx, userID, productID, quantity)
}

func (f *fakeCartService) RemoveProduct(ctx context.Context, userID, productID int64) error {
	return f.removeProductFn(ctx, userID, productID)
}

func (f *fakeCartService) ChangeProductQuantity(ctx context.Context, userID, productID, quantity int64) error {
	return f.changeProductQuantityFn(ctx, userID, productID, quantity)
}

func TestGetCartUsesAuthenticatedUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeCartService{
		getCartFn: func(_ context.Context, userID int64) (*domain.Cart, error) {
			require.Equal(t, int64(42), userID)
			return &domain.Cart{Items: []domain.CartItem{{ProductID: 7, Quantity: 2}}}, nil
		},
	}
	router := authenticatedCartRouter()
	router.GET("/cart", New(cartTestLogger(), service).GetCart)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/cart", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"product_id":7`)
	require.Contains(t, recorder.Body.String(), `"quantity":2`)
}

func TestAddProductAcceptsZeroQuantity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeCartService{
		addProductFn: func(_ context.Context, userID, productID, quantity int64) (*domain.AddProductResult, error) {
			require.Equal(t, int64(42), userID)
			require.Equal(t, int64(7), productID)
			require.Zero(t, quantity)
			return &domain.AddProductResult{CurrentQuantity: 2, NewQuantity: 0}, nil
		},
	}
	router := authenticatedCartRouter()
	router.POST("/cart/items", New(cartTestLogger(), service).AddProduct)

	request := httptest.NewRequest(
		http.MethodPost,
		"/cart/items",
		bytes.NewBufferString(`{"product_id":7,"quantity":0}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"previous_quantity":2`)
	require.Contains(t, recorder.Body.String(), `"new_quantity":0`)
}

func TestChangeQuantityUsesPathProductID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeCartService{
		changeProductQuantityFn: func(_ context.Context, userID, productID, quantity int64) error {
			require.Equal(t, int64(42), userID)
			require.Equal(t, int64(8), productID)
			require.Equal(t, int64(3), quantity)
			return nil
		},
	}
	router := authenticatedCartRouter()
	router.PATCH("/cart/items/:product_id", New(cartTestLogger(), service).ChangeQuantity)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/cart/items/8",
		bytes.NewBufferString(`{"quantity":3}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestRemoveProductRejectsInvalidPathID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serviceCalled := false
	service := &fakeCartService{
		removeProductFn: func(context.Context, int64, int64) error {
			serviceCalled = true
			return nil
		},
	}
	router := authenticatedCartRouter()
	router.DELETE("/cart/items/:product_id", New(cartTestLogger(), service).RemoveProduct)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/cart/items/0", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, serviceCalled)
}

func TestCartHandlerRequiresPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serviceCalled := false
	service := &fakeCartService{
		getCartFn: func(context.Context, int64) (*domain.Cart, error) {
			serviceCalled = true
			return nil, nil
		},
	}
	router := gin.New()
	router.GET("/cart", New(cartTestLogger(), service).GetCart)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/cart", nil))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.False(t, serviceCalled)
}

func authenticatedCartRouter() *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity:    domain.Identity{UserID: 42, Role: "user"},
			AccessToken: "access-token",
		})
		ctx.Next()
	})
	return router
}

func cartTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
