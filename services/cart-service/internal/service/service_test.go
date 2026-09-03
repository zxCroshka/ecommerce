package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/domain"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
)

type fakeCartManager struct {
	insertCalled   bool
	deleteCalled   bool
	changeCalled   bool
	insertMax      int64
	insertQuantity int64
	checkoutErr    error
	snapshot       *domain.CartSnapshot
}

func (f *fakeCartManager) InsertCartProduct(
	_ context.Context,
	_, _, quantity, maxQuantity int64,
	_ time.Duration,
) (int64, int64, error) {
	f.insertCalled = true
	f.insertQuantity = quantity
	f.insertMax = maxQuantity
	return quantity, 1, nil
}

func (f *fakeCartManager) DeleteCartProduct(context.Context, int64, int64, time.Duration) (int64, error) {
	f.deleteCalled = true
	return 3, nil
}

func (f *fakeCartManager) GetCartProducts(context.Context, int64) (*domain.Cart, error) {
	return &domain.Cart{Items: map[domain.ProductID]domain.Quantity{}}, nil
}

func (f *fakeCartManager) ChangeProductQuantity(
	context.Context,
	int64,
	int64,
	int64,
	time.Duration,
) error {
	f.changeCalled = true
	return nil
}

func (f *fakeCartManager) GetCartSnapshot(context.Context, int64, time.Duration) (*domain.CartSnapshot, error) {
	return f.snapshot, f.checkoutErr
}

func (f *fakeCartManager) ClearCartIfUnchanged(context.Context, int64, int64, time.Duration) (bool, error) {
	return true, nil
}

type fakeProductClient struct {
	called  bool
	product *productservicev1.Product
	err     error
}

func (f *fakeProductClient) GetProduct(context.Context, int64) (*productservicev1.Product, error) {
	f.called = true
	return f.product, f.err
}

func newTestService(manager CartManager, productClient ProductServiceClient) *CartService {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewCartService(log, manager, productClient, 7*24*time.Hour, 99)
}

func userContext() context.Context {
	return auth.WithUserIdentity(context.Background(), auth.UserIdentity{UserID: 1, Role: "customer"})
}

func TestAddProductToCart_UsesConfiguredLimit(t *testing.T) {
	manager := &fakeCartManager{}
	productClient := &fakeProductClient{
		product: &productservicev1.Product{Id: 10, IsActive: true, Stock: 500},
	}
	srv := newTestService(manager, productClient)

	_, _, err := srv.AddProductToCart(userContext(), 10, 5)
	require.NoError(t, err)
	assert.True(t, manager.insertCalled)
	assert.EqualValues(t, 99, manager.insertMax)
}

func TestAddProductToCart_ZeroRemovesWithoutProductCall(t *testing.T) {
	manager := &fakeCartManager{}
	productClient := &fakeProductClient{}
	srv := newTestService(manager, productClient)

	_, _, err := srv.AddProductToCart(userContext(), 10, 0)
	require.NoError(t, err)
	assert.False(t, productClient.called)
	assert.False(t, manager.insertCalled)
	assert.True(t, manager.deleteCalled)
}

func TestChangeProductQuantity_NonPositiveRemovesWithoutProductCall(t *testing.T) {
	manager := &fakeCartManager{}
	productClient := &fakeProductClient{}
	srv := newTestService(manager, productClient)

	err := srv.ChangeProductQuantity(userContext(), 10, -1)
	require.NoError(t, err)
	assert.False(t, productClient.called)
	assert.True(t, manager.changeCalled)
}

func TestChangeProductQuantity_RejectsConfiguredLimit(t *testing.T) {
	manager := &fakeCartManager{}
	productClient := &fakeProductClient{
		product: &productservicev1.Product{Id: 10, IsActive: true, Stock: 500},
	}
	srv := newTestService(manager, productClient)

	err := srv.ChangeProductQuantity(userContext(), 10, 100)
	assert.ErrorIs(t, err, customerrors.ErrQuantityExceedsLimit)
	assert.False(t, manager.changeCalled)
}

func TestGetCartForCheckout_EmptyCart(t *testing.T) {
	manager := &fakeCartManager{checkoutErr: customerrors.ErrCartEmpty}
	srv := newTestService(manager, &fakeProductClient{})

	ctx := auth.WithServiceIdentity(context.Background())
	_, err := srv.GetCartForCheckout(ctx, 1)
	assert.True(t, errors.Is(err, customerrors.ErrCartEmpty))
}

func TestUserMethodsRequireIdentityFromContext(t *testing.T) {
	srv := newTestService(&fakeCartManager{}, &fakeProductClient{})

	_, err := srv.GetCartProducts(context.Background())
	assert.ErrorIs(t, err, customerrors.ErrInvalidUserID)
}
