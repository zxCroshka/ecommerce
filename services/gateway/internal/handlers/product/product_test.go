package product

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type fakeProductService struct {
	getProductFn    func(context.Context, int64, string) (*domain.Product, error)
	listProductsFn  func(context.Context, domain.ProductListRequest, string) (*domain.ProductList, error)
	createProductFn func(context.Context, string, domain.CreateProductInput) (int64, error)
	updateProductFn func(context.Context, string, int64, domain.ProductPatch) error
	softDeleteFn    func(context.Context, string, int64) error
}

func (f *fakeProductService) GetProduct(ctx context.Context, id int64, token string) (*domain.Product, error) {
	return f.getProductFn(ctx, id, token)
}

func (f *fakeProductService) ListProducts(ctx context.Context, request domain.ProductListRequest, token string) (*domain.ProductList, error) {
	return f.listProductsFn(ctx, request, token)
}

func (f *fakeProductService) CreateProduct(ctx context.Context, token string, input domain.CreateProductInput) (int64, error) {
	return f.createProductFn(ctx, token, input)
}

func (f *fakeProductService) UpdateProduct(ctx context.Context, token string, id int64, patch domain.ProductPatch) error {
	return f.updateProductFn(ctx, token, id, patch)
}

func (f *fakeProductService) SoftDelete(ctx context.Context, token string, id int64) error {
	return f.softDeleteFn(ctx, token, id)
}

func TestGetProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeProductService{
		getProductFn: func(_ context.Context, id int64, token string) (*domain.Product, error) {
			require.Equal(t, int64(7), id)
			require.Empty(t, token)
			return &domain.Product{
				ID: 7, Name: "Book", Price: 100, Stock: 3, Category: "books",
				Images: []string{}, IsActive: true, CreatedAt: time.Unix(1, 0), UpdatedAt: time.Unix(2, 0),
			}, nil
		},
	}
	router := gin.New()
	router.GET("/products/:id", New(testHandlerLogger(), service).GetProduct)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/products/7", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":7`)
	require.Contains(t, recorder.Body.String(), `"images":[]`)
}

func TestListProductsMapsPaginationAndFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeProductService{
		listProductsFn: func(_ context.Context, request domain.ProductListRequest, token string) (*domain.ProductList, error) {
			require.Empty(t, token)
			require.Equal(t, int32(10), request.Limit)
			require.Equal(t, int32(10), request.Offset)
			require.Equal(t, domain.ProductSortByPrice, request.Sort)
			require.Equal(t, domain.ProductOrderAsc, request.Order)
			require.NotNil(t, request.Category)
			require.Equal(t, "books", *request.Category)
			require.NotNil(t, request.IsActive)
			require.False(t, *request.IsActive)
			return &domain.ProductList{Products: []domain.Product{}, Total: 21, Limit: 10, Offset: 10}, nil
		},
	}
	router := gin.New()
	router.GET("/products", New(testHandlerLogger(), service).ListProducts)

	request := httptest.NewRequest(
		http.MethodGet,
		"/products?page=2&page_size=10&sort=price&order=asc&category=books&is_active=false",
		nil,
	)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"total_pages":3`)
	require.Contains(t, recorder.Body.String(), `"products":[]`)
}

func TestCreateProductAcceptsZeroPriceAndStock(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeProductService{
		createProductFn: func(_ context.Context, token string, input domain.CreateProductInput) (int64, error) {
			require.Equal(t, "access-token", token)
			require.Zero(t, input.Price)
			require.Zero(t, input.Stock)
			return 15, nil
		},
	}
	router := authenticatedRouter()
	router.POST("/products", New(testHandlerLogger(), service).CreateProduct)

	body := bytes.NewBufferString(`{
		"name":"Free product",
		"description":"test",
		"price":0,
		"stock":0,
		"category":"test"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/products", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"product_id":15`)
}

func TestUpdateProductPreservesExplicitEmptyImages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeProductService{
		updateProductFn: func(_ context.Context, token string, id int64, patch domain.ProductPatch) error {
			require.Equal(t, "access-token", token)
			require.Equal(t, int64(9), id)
			require.NotNil(t, patch.Images)
			require.Empty(t, *patch.Images)
			return nil
		},
	}
	router := authenticatedRouter()
	router.PATCH("/products/:id", New(testHandlerLogger(), service).UpdateProduct)

	request := httptest.NewRequest(http.MethodPatch, "/products/9", bytes.NewBufferString(`{"images":[]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetProductRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serviceCalled := false
	service := &fakeProductService{
		getProductFn: func(context.Context, int64, string) (*domain.Product, error) {
			serviceCalled = true
			return nil, nil
		},
	}
	router := gin.New()
	router.GET("/products/:id", New(testHandlerLogger(), service).GetProduct)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/products/not-a-number", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, serviceCalled)
}

func authenticatedRouter() *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity:    domain.Identity{UserID: 42, Role: "admin"},
			AccessToken: "access-token",
		})
		ctx.Next()
	})
	return router
}

func testHandlerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
