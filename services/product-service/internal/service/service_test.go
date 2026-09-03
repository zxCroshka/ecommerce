package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
)

type productManagerMock struct {
	saveFn         func(context.Context, string, string, int64, int64, string, []string, bool) (int64, error)
	updateFn       func(context.Context, int64, domain.ProductPatch) error
	listFn         func(context.Context, domain.ProductListRequest) ([]*domain.Product, int64, error)
	softDeleteFn   func(context.Context, int64) error
	getFn          func(context.Context, int64) (*domain.Product, error)
	reserveStockFn func(context.Context, string, int64, int64) (int64, bool, error)
	releaseStockFn func(context.Context, string, int64) (int64, bool, error)
}

func (m *productManagerMock) SaveProduct(
	ctx context.Context,
	name, description string,
	price, stock int64,
	category string,
	images []string,
	isActive bool,
) (int64, error) {
	if m.saveFn == nil {
		return 0, errors.New("unexpected SaveProduct call")
	}
	return m.saveFn(ctx, name, description, price, stock, category, images, isActive)
}

func (m *productManagerMock) UpdateProductFields(
	ctx context.Context,
	productID int64,
	patch domain.ProductPatch,
) error {
	if m.updateFn == nil {
		return errors.New("unexpected UpdateProductFields call")
	}
	return m.updateFn(ctx, productID, patch)
}

func (m *productManagerMock) ListProducts(
	ctx context.Context,
	req domain.ProductListRequest,
) ([]*domain.Product, int64, error) {
	if m.listFn == nil {
		return nil, 0, errors.New("unexpected ListProducts call")
	}
	return m.listFn(ctx, req)
}

func (m *productManagerMock) SoftDelete(ctx context.Context, productID int64) error {
	if m.softDeleteFn == nil {
		return errors.New("unexpected SoftDelete call")
	}
	return m.softDeleteFn(ctx, productID)
}

func (m *productManagerMock) GetProduct(ctx context.Context, productID int64) (*domain.Product, error) {
	if m.getFn == nil {
		return nil, errors.New("unexpected GetProduct call")
	}
	return m.getFn(ctx, productID)
}

func (m *productManagerMock) ReserveStockTX(
	ctx context.Context,
	reservationID string,
	productID, quantity int64,
) (int64, bool, error) {
	if m.reserveStockFn == nil {
		return 0, false, errors.New("unexpected ReserveStockTX call")
	}
	return m.reserveStockFn(ctx, reservationID, productID, quantity)
}

func (m *productManagerMock) ReleaseStockTX(
	ctx context.Context,
	reservationID string,
	productID int64,
) (int64, bool, error) {
	if m.releaseStockFn == nil {
		return 0, false, errors.New("unexpected ReleaseStockTX call")
	}
	return m.releaseStockFn(ctx, reservationID, productID)
}

type cacheManagerMock struct {
	setListFn       func(context.Context, string, []*domain.Product, int64) error
	getListFn       func(context.Context, string) ([]*domain.Product, int64, error)
	setProductFn    func(context.Context, int64, *domain.Product) error
	getProductFn    func(context.Context, int64) (*domain.Product, error)
	invalidateAllFn func(context.Context) error
	buildKeyFn      func(domain.ProductFilter, domain.SortField, domain.SortOrder, int, int) string
	generationFn    func(context.Context) (int64, error)
}

func (m *cacheManagerMock) SetListProductsCacheIfGeneration(
	ctx context.Context,
	key string,
	products []*domain.Product,
	total int64,
	_ int64,
) (bool, error) {
	return true, m.SetListProductsCache(ctx, key, products, total)
}

func (m *cacheManagerMock) SetListProductsCache(
	ctx context.Context,
	key string,
	products []*domain.Product,
	total int64,
) error {
	if m.setListFn == nil {
		return nil
	}
	return m.setListFn(ctx, key, products, total)
}

func (m *cacheManagerMock) SetProductCacheIfGeneration(
	ctx context.Context,
	productID int64,
	product *domain.Product,
	_ int64,
) (bool, error) {
	return true, m.SetProductCache(ctx, productID, product)
}

func (m *cacheManagerMock) GetListProductsCache(
	ctx context.Context,
	key string,
) ([]*domain.Product, int64, error) {
	if m.getListFn == nil {
		return nil, 0, customerrors.ErrCacheMiss
	}
	return m.getListFn(ctx, key)
}

func (m *cacheManagerMock) InvalidateProductsCache(context.Context, string) error {
	return nil
}

func (m *cacheManagerMock) InvalidateProductsCacheByPattern(context.Context, string) error {
	return nil
}

func (m *cacheManagerMock) SetProductCache(
	ctx context.Context,
	productID int64,
	product *domain.Product,
) error {
	if m.setProductFn == nil {
		return nil
	}
	return m.setProductFn(ctx, productID, product)
}

func (m *cacheManagerMock) GetProductCache(
	ctx context.Context,
	productID int64,
) (*domain.Product, error) {
	if m.getProductFn == nil {
		return nil, customerrors.ErrCacheMiss
	}
	return m.getProductFn(ctx, productID)
}

func (m *cacheManagerMock) InvalidateProductCache(context.Context, int64) error {
	return nil
}

func (m *cacheManagerMock) InvalidateAllProductCache(ctx context.Context) error {
	if m.invalidateAllFn == nil {
		return nil
	}
	return m.invalidateAllFn(ctx)
}

func (m *cacheManagerMock) CacheGeneration(ctx context.Context) (int64, error) {
	if m.generationFn == nil {
		return 0, nil
	}
	return m.generationFn(ctx)
}

func (m *cacheManagerMock) BuildListCacheKey(
	filter domain.ProductFilter,
	sort domain.SortField,
	order domain.SortOrder,
	limit, offset int,
) string {
	if m.buildKeyFn == nil {
		return "products:test"
	}
	return m.buildKeyFn(filter, sort, order, limit, offset)
}

type producerMock struct {
	produceFn func(int64, map[string]any) error
	calls     atomic.Int32
}

func (m *producerMock) ProduceProductUpdated(productID int64, changes map[string]any) error {
	m.calls.Add(1)
	if m.produceFn == nil {
		return nil
	}
	return m.produceFn(productID, changes)
}

func newTestService(
	products ProductManager,
	cache CacheManager,
	_ any,
) *ProductService {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(log, products, cache)
}

func validProduct() *domain.Product {
	return &domain.Product{
		Id:          42,
		Name:        "Mechanical keyboard",
		Description: "Test description",
		Price:       10_000,
		Stock:       20,
		Category:    "keyboards",
		Images:      []string{"https://example.com/keyboard.jpg"},
		IsActive:    true,
	}
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for asynchronous operation")
	}
}

func waitChanges(t *testing.T, ch <-chan map[string]any) map[string]any {
	t.Helper()
	select {
	case changes := <-ch:
		return changes
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Kafka event")
		return nil
	}
}

func TestCreateProduct(t *testing.T) {
	t.Run("customer is forbidden", func(t *testing.T) {
		srv := newTestService(&productManagerMock{}, &cacheManagerMock{}, &producerMock{})

		id, err := srv.CreateProduct(
			context.Background(), "Keyboard", "", 100, 5, "devices", nil, true, false,
		)

		require.Zero(t, id)
		require.ErrorIs(t, err, customerrors.ErrForbidden)
	})

	validationTests := []struct {
		name        string
		productName string
		price       int64
		stock       int64
		category    string
		images      []string
	}{
		{name: "empty name", productName: " ", price: 100, stock: 5, category: "devices"},
		{name: "negative price", productName: "Keyboard", price: -1, stock: 5, category: "devices"},
		{name: "negative stock", productName: "Keyboard", price: 100, stock: -1, category: "devices"},
		{name: "empty category", productName: "Keyboard", price: 100, stock: 5, category: " "},
		{
			name:        "invalid image",
			productName: "Keyboard",
			price:       100,
			stock:       5,
			category:    "devices",
			images:      []string{"example.com/image.jpg"},
		},
	}

	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestService(&productManagerMock{}, &cacheManagerMock{}, &producerMock{})

			_, err := srv.CreateProduct(
				context.Background(), tt.productName, "", tt.price, tt.stock,
				tt.category, tt.images, true, true,
			)

			require.ErrorIs(t, err, customerrors.ErrInvalidProductData)
		})
	}

	t.Run("repository error is preserved", func(t *testing.T) {
		repositoryErr := errors.New("postgres unavailable")
		products := &productManagerMock{
			saveFn: func(
				context.Context, string, string, int64, int64, string, []string, bool,
			) (int64, error) {
				return 0, repositoryErr
			},
		}
		srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

		_, err := srv.CreateProduct(
			context.Background(), "Keyboard", "", 100, 5, "devices", nil, true, true,
		)

		require.ErrorIs(t, err, repositoryErr)
	})

	t.Run("success invalidates cache", func(t *testing.T) {
		invalidated := make(chan struct{}, 1)
		products := &productManagerMock{
			saveFn: func(
				_ context.Context,
				name, description string,
				price, stock int64,
				category string,
				images []string,
				isActive bool,
			) (int64, error) {
				require.Equal(t, "Keyboard", name)
				require.Equal(t, int64(100), price)
				require.Equal(t, int64(5), stock)
				require.Equal(t, "devices", category)
				require.True(t, isActive)
				return 42, nil
			},
		}
		cache := &cacheManagerMock{
			invalidateAllFn: func(context.Context) error {
				invalidated <- struct{}{}
				return nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		id, err := srv.CreateProduct(
			context.Background(), "Keyboard", "", 100, 5, "devices", nil, true, true,
		)

		require.NoError(t, err)
		require.Equal(t, int64(42), id)
		waitSignal(t, invalidated)
	})
}

func TestUpdateProductFields(t *testing.T) {
	t.Run("customer is forbidden", func(t *testing.T) {
		price := int64(200)
		srv := newTestService(&productManagerMock{}, &cacheManagerMock{}, &producerMock{})

		err := srv.UpdateProductFields(
			context.Background(), 42, domain.ProductPatch{Price: &price}, false,
		)

		require.ErrorIs(t, err, customerrors.ErrForbidden)
	})

	t.Run("empty patch is rejected", func(t *testing.T) {
		srv := newTestService(&productManagerMock{}, &cacheManagerMock{}, &producerMock{})

		err := srv.UpdateProductFields(context.Background(), 42, domain.ProductPatch{}, true)

		require.ErrorIs(t, err, customerrors.ErrInvalidProductData)
	})

	t.Run("missing product", func(t *testing.T) {
		price := int64(200)
		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return nil, customerrors.ErrProductNotFound
			},
		}
		srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

		err := srv.UpdateProductFields(
			context.Background(), 42, domain.ProductPatch{Price: &price}, true,
		)

		require.ErrorIs(t, err, customerrors.ErrProductNotFound)
	})

	validationTests := []struct {
		name  string
		patch domain.ProductPatch
	}{
		{name: "invalid name", patch: domain.ProductPatch{Name: pointer("x")}},
		{name: "invalid price", patch: domain.ProductPatch{Price: pointer(int64(-1))}},
		{name: "invalid stock", patch: domain.ProductPatch{Stock: pointer(int64(-1))}},
		{name: "invalid category", patch: domain.ProductPatch{Category: pointer("x")}},
		{
			name: "invalid images",
			patch: domain.ProductPatch{
				Images:    []string{""},
				ImagesSet: true,
			},
		},
	}

	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			products := &productManagerMock{
				getFn: func(context.Context, int64) (*domain.Product, error) {
					return validProduct(), nil
				},
			}
			srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

			err := srv.UpdateProductFields(context.Background(), 42, tt.patch, true)

			require.ErrorIs(t, err, customerrors.ErrInvalidProductData)
		})
	}

	t.Run("success invalidates cache synchronously", func(t *testing.T) {
		price := int64(12_000)
		stock := int64(15)
		active := false
		invalidated := make(chan struct{}, 1)

		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return validProduct(), nil
			},
			updateFn: func(_ context.Context, productID int64, patch domain.ProductPatch) error {
				require.Equal(t, int64(42), productID)
				require.Equal(t, price, *patch.Price)
				require.Equal(t, stock, *patch.Stock)
				return nil
			},
		}
		cache := &cacheManagerMock{
			invalidateAllFn: func(context.Context) error {
				invalidated <- struct{}{}
				return nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		err := srv.UpdateProductFields(
			context.Background(),
			42,
			domain.ProductPatch{Price: &price, Stock: &stock, IsActive: &active},
			true,
		)

		require.NoError(t, err)
		waitSignal(t, invalidated)
	})

	t.Run("repository update error is preserved", func(t *testing.T) {
		updateErr := errors.New("update failed")
		description := "new description"
		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return validProduct(), nil
			},
			updateFn: func(context.Context, int64, domain.ProductPatch) error {
				return updateErr
			},
		}
		srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

		err := srv.UpdateProductFields(
			context.Background(), 42, domain.ProductPatch{Description: &description}, true,
		)

		require.ErrorIs(t, err, updateErr)
	})
}

func TestListProducts(t *testing.T) {
	t.Run("customer gets only active products from cache", func(t *testing.T) {
		cached := []*domain.Product{validProduct()}
		products := &productManagerMock{}
		cache := &cacheManagerMock{
			buildKeyFn: func(
				filter domain.ProductFilter,
				_ domain.SortField,
				_ domain.SortOrder,
				_, _ int,
			) string {
				require.NotNil(t, filter.IsActive)
				require.True(t, *filter.IsActive)
				return "customer-key"
			},
			getListFn: func(_ context.Context, key string) ([]*domain.Product, int64, error) {
				require.Equal(t, "customer-key", key)
				return cached, 1, nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		got, total, err := srv.ListProducts(
			context.Background(),
			domain.ProductListRequest{Limit: 20},
			false,
		)

		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, cached, got)
	})

	t.Run("cache miss reads repository and fills cache", func(t *testing.T) {
		expected := []*domain.Product{validProduct()}
		cacheSet := make(chan struct{}, 1)
		products := &productManagerMock{
			listFn: func(
				_ context.Context,
				req domain.ProductListRequest,
			) ([]*domain.Product, int64, error) {
				require.Nil(t, req.Filter.IsActive)
				return expected, 1, nil
			},
		}
		cache := &cacheManagerMock{
			getListFn: func(context.Context, string) ([]*domain.Product, int64, error) {
				return nil, 0, customerrors.ErrCacheMiss
			},
			setListFn: func(
				_ context.Context,
				key string,
				products []*domain.Product,
				total int64,
			) error {
				require.Equal(t, "products:test", key)
				require.Equal(t, expected, products)
				require.Equal(t, int64(1), total)
				cacheSet <- struct{}{}
				return nil
			},
			buildKeyFn: func(
				domain.ProductFilter, domain.SortField, domain.SortOrder, int, int,
			) string {
				return "products:test"
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		got, total, err := srv.ListProducts(
			context.Background(),
			domain.ProductListRequest{Limit: 20},
			true,
		)

		require.NoError(t, err)
		require.Equal(t, expected, got)
		require.Equal(t, int64(1), total)
		waitSignal(t, cacheSet)
	})

	t.Run("repository error is preserved", func(t *testing.T) {
		listErr := errors.New("list failed")
		products := &productManagerMock{
			listFn: func(context.Context, domain.ProductListRequest) ([]*domain.Product, int64, error) {
				return nil, 0, listErr
			},
		}
		srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

		got, total, err := srv.ListProducts(context.Background(), domain.ProductListRequest{}, true)

		require.Nil(t, got)
		require.Zero(t, total)
		require.ErrorIs(t, err, listErr)
	})
}

func TestGetProduct(t *testing.T) {
	t.Run("active product cache hit", func(t *testing.T) {
		expected := validProduct()
		cache := &cacheManagerMock{
			getProductFn: func(context.Context, int64) (*domain.Product, error) {
				return expected, nil
			},
		}
		srv := newTestService(&productManagerMock{}, cache, &producerMock{})

		got, err := srv.GetProduct(context.Background(), 42, false)

		require.NoError(t, err)
		require.Same(t, expected, got)
	})

	t.Run("inactive cached product is hidden from customer", func(t *testing.T) {
		expected := validProduct()
		expected.IsActive = false
		cache := &cacheManagerMock{
			getProductFn: func(context.Context, int64) (*domain.Product, error) {
				return expected, nil
			},
		}
		srv := newTestService(&productManagerMock{}, cache, &producerMock{})

		got, err := srv.GetProduct(context.Background(), 42, false)

		require.Nil(t, got)
		require.ErrorIs(t, err, customerrors.ErrProductNotFound)
	})

	t.Run("admin can get inactive cached product", func(t *testing.T) {
		expected := validProduct()
		expected.IsActive = false
		cache := &cacheManagerMock{
			getProductFn: func(context.Context, int64) (*domain.Product, error) {
				return expected, nil
			},
		}
		srv := newTestService(&productManagerMock{}, cache, &producerMock{})

		got, err := srv.GetProduct(context.Background(), 42, true)

		require.NoError(t, err)
		require.Same(t, expected, got)
	})

	t.Run("cache miss reads repository and fills cache", func(t *testing.T) {
		expected := validProduct()
		cacheSet := make(chan struct{}, 1)
		products := &productManagerMock{
			getFn: func(_ context.Context, productID int64) (*domain.Product, error) {
				require.Equal(t, int64(42), productID)
				return expected, nil
			},
		}
		cache := &cacheManagerMock{
			setProductFn: func(
				_ context.Context,
				productID int64,
				product *domain.Product,
			) error {
				require.Equal(t, int64(42), productID)
				require.Same(t, expected, product)
				cacheSet <- struct{}{}
				return nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		got, err := srv.GetProduct(context.Background(), 42, false)

		require.NoError(t, err)
		require.Same(t, expected, got)
		waitSignal(t, cacheSet)
	})

	t.Run("inactive repository product is hidden and not cached", func(t *testing.T) {
		expected := validProduct()
		expected.IsActive = false
		var cacheSet atomic.Int32
		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return expected, nil
			},
		}
		cache := &cacheManagerMock{
			setProductFn: func(context.Context, int64, *domain.Product) error {
				cacheSet.Add(1)
				return nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		got, err := srv.GetProduct(context.Background(), 42, false)

		require.Nil(t, got)
		require.ErrorIs(t, err, customerrors.ErrProductNotFound)
		require.Zero(t, cacheSet.Load())
	})

	t.Run("repository errors are preserved", func(t *testing.T) {
		repositoryErr := errors.New("get failed")
		for _, tt := range []struct {
			name string
			err  error
		}{
			{name: "not found", err: customerrors.ErrProductNotFound},
			{name: "generic", err: repositoryErr},
		} {
			t.Run(tt.name, func(t *testing.T) {
				products := &productManagerMock{
					getFn: func(context.Context, int64) (*domain.Product, error) {
						return nil, tt.err
					},
				}
				srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

				got, err := srv.GetProduct(context.Background(), 42, false)

				require.Nil(t, got)
				require.ErrorIs(t, err, tt.err)
			})
		}
	})
}

func TestSoftDelete(t *testing.T) {
	t.Run("customer is forbidden", func(t *testing.T) {
		srv := newTestService(&productManagerMock{}, &cacheManagerMock{}, &producerMock{})

		err := srv.SoftDelete(context.Background(), 42, false)

		require.ErrorIs(t, err, customerrors.ErrForbidden)
	})

	t.Run("missing product", func(t *testing.T) {
		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return nil, customerrors.ErrProductNotFound
			},
		}
		srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

		err := srv.SoftDelete(context.Background(), 42, true)

		require.ErrorIs(t, err, customerrors.ErrProductNotFound)
	})

	t.Run("inactive product is rejected", func(t *testing.T) {
		product := validProduct()
		product.IsActive = false
		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return product, nil
			},
		}
		srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

		err := srv.SoftDelete(context.Background(), 42, true)

		require.Error(t, err)
		require.Contains(t, err.Error(), "already deleted")
	})

	t.Run("repository error is preserved", func(t *testing.T) {
		deleteErr := errors.New("delete failed")
		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return validProduct(), nil
			},
			softDeleteFn: func(context.Context, int64) error {
				return deleteErr
			},
		}
		srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

		err := srv.SoftDelete(context.Background(), 42, true)

		require.ErrorIs(t, err, deleteErr)
	})

	t.Run("success invalidates cache", func(t *testing.T) {
		invalidated := make(chan struct{}, 1)
		products := &productManagerMock{
			getFn: func(context.Context, int64) (*domain.Product, error) {
				return validProduct(), nil
			},
			softDeleteFn: func(_ context.Context, productID int64) error {
				require.Equal(t, int64(42), productID)
				return nil
			},
		}
		cache := &cacheManagerMock{
			invalidateAllFn: func(context.Context) error {
				invalidated <- struct{}{}
				return nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		err := srv.SoftDelete(context.Background(), 42, true)

		require.NoError(t, err)
		waitSignal(t, invalidated)
	})
}

func TestReserveStock(t *testing.T) {
	t.Run("invalid quantity", func(t *testing.T) {
		srv := newTestService(&productManagerMock{}, &cacheManagerMock{}, &producerMock{})

		err := srv.ReserveStock(context.Background(), "reservation-1", 42, 0)

		require.ErrorIs(t, err, customerrors.ErrInvalidProductData)
	})

	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "insufficient stock", err: customerrors.ErrInsufficientStock},
		{name: "inactive product", err: customerrors.ErrProductInactive},
		{name: "missing product", err: customerrors.ErrProductNotFound},
		{name: "reservation conflict", err: customerrors.ErrReservationConflict},
		{name: "repository failure", err: errors.New("reserve failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			products := &productManagerMock{
				reserveStockFn: func(context.Context, string, int64, int64) (int64, bool, error) {
					return 0, false, tt.err
				},
			}
			srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

			err := srv.ReserveStock(context.Background(), "reservation-1", 42, 2)

			require.ErrorIs(t, err, tt.err)
		})
	}

	t.Run("idempotent retry does not repeat side effects", func(t *testing.T) {
		var cacheInvalidations atomic.Int32
		products := &productManagerMock{
			reserveStockFn: func(context.Context, string, int64, int64) (int64, bool, error) {
				return 0, false, nil
			},
		}
		cache := &cacheManagerMock{
			invalidateAllFn: func(context.Context) error {
				cacheInvalidations.Add(1)
				return nil
			},
		}
		producer := &producerMock{}
		srv := newTestService(products, cache, producer)

		err := srv.ReserveStock(context.Background(), "reservation-1", 42, 2)

		require.NoError(t, err)
		require.Zero(t, cacheInvalidations.Load())
		require.Zero(t, producer.calls.Load())
	})

	t.Run("success invalidates cache", func(t *testing.T) {
		invalidated := make(chan struct{}, 1)
		products := &productManagerMock{
			reserveStockFn: func(
				_ context.Context,
				reservationID string,
				productID, quantity int64,
			) (int64, bool, error) {
				require.Equal(t, "reservation-1", reservationID)
				require.Equal(t, int64(42), productID)
				require.Equal(t, int64(2), quantity)
				return 18, true, nil
			},
		}
		cache := &cacheManagerMock{
			invalidateAllFn: func(context.Context) error {
				invalidated <- struct{}{}
				return nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		err := srv.ReserveStock(context.Background(), "reservation-1", 42, 2)

		require.NoError(t, err)
		waitSignal(t, invalidated)
	})
}

func TestReleaseStock(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{name: "missing product", err: customerrors.ErrProductNotFound},
		{name: "missing reservation", err: customerrors.ErrReservationNotFound},
		{name: "reservation conflict", err: customerrors.ErrReservationConflict},
		{name: "repository failure", err: errors.New("release failed")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			products := &productManagerMock{
				releaseStockFn: func(context.Context, string, int64) (int64, bool, error) {
					return 0, false, tt.err
				},
			}
			srv := newTestService(products, &cacheManagerMock{}, &producerMock{})

			err := srv.ReleaseStock(context.Background(), "reservation-1", 42)

			require.ErrorIs(t, err, tt.err)
		})
	}

	t.Run("idempotent retry does not repeat side effects", func(t *testing.T) {
		var cacheInvalidations atomic.Int32
		products := &productManagerMock{
			releaseStockFn: func(context.Context, string, int64) (int64, bool, error) {
				return 0, false, nil
			},
		}
		cache := &cacheManagerMock{
			invalidateAllFn: func(context.Context) error {
				cacheInvalidations.Add(1)
				return nil
			},
		}
		producer := &producerMock{}
		srv := newTestService(products, cache, producer)

		err := srv.ReleaseStock(context.Background(), "reservation-1", 42)

		require.NoError(t, err)
		require.Zero(t, cacheInvalidations.Load())
		require.Zero(t, producer.calls.Load())
	})

	t.Run("success invalidates cache", func(t *testing.T) {
		invalidated := make(chan struct{}, 1)
		products := &productManagerMock{
			releaseStockFn: func(
				_ context.Context,
				reservationID string,
				productID int64,
			) (int64, bool, error) {
				require.Equal(t, "reservation-1", reservationID)
				require.Equal(t, int64(42), productID)
				return 22, true, nil
			},
		}
		cache := &cacheManagerMock{
			invalidateAllFn: func(context.Context) error {
				invalidated <- struct{}{}
				return nil
			},
		}
		srv := newTestService(products, cache, &producerMock{})

		err := srv.ReleaseStock(context.Background(), "reservation-1", 42)

		require.NoError(t, err)
		waitSignal(t, invalidated)
	})
}

func pointer[T any](value T) *T {
	return &value
}
