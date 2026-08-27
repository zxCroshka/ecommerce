package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/domain"
)

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
		server.Close()
	})
	return &Client{client: client}, server
}

func TestInsertCartProduct_UsesLimitAndRefreshesTTL(t *testing.T) {
	client, server := newTestClient(t)
	ctx := context.Background()
	ttl := 7 * 24 * time.Hour

	newQuantity, oldQuantity, err := client.InsertCartProduct(ctx, 1, 10, 50, 99, ttl)
	require.NoError(t, err)
	assert.EqualValues(t, 50, newQuantity)
	assert.EqualValues(t, 0, oldQuantity)

	newQuantity, oldQuantity, err = client.InsertCartProduct(ctx, 1, 10, 49, 99, ttl)
	require.NoError(t, err)
	assert.EqualValues(t, 99, newQuantity)
	assert.EqualValues(t, 50, oldQuantity)

	_, _, err = client.InsertCartProduct(ctx, 1, 10, 1, 99, ttl)
	assert.ErrorIs(t, err, customerrors.ErrQuantityExceedsLimit)
	assert.Equal(t, ttl, server.TTL("cart:1"))
}

func TestChangeAndDeleteCartProduct(t *testing.T) {
	client, server := newTestClient(t)
	ctx := context.Background()
	ttl := 7 * 24 * time.Hour

	_, _, err := client.InsertCartProduct(ctx, 1, 10, 1, 99, ttl)
	require.NoError(t, err)
	_, _, err = client.InsertCartProduct(ctx, 1, 20, 1, 99, ttl)
	require.NoError(t, err)

	require.NoError(t, client.ChangeProductQuantity(ctx, 1, 10, 7, ttl))
	assert.Equal(t, "7", server.HGet("cart:1", "10"))

	server.FastForward(time.Hour)
	oldQuantity, err := client.DeleteCartProduct(ctx, 1, 10, ttl)
	require.NoError(t, err)
	assert.EqualValues(t, 7, oldQuantity)
	assert.Empty(t, server.HGet("cart:1", "10"))
	assert.Equal(t, "1", server.HGet("cart:1", "20"))
	assert.Equal(t, ttl, server.TTL("cart:1"))

	require.NoError(t, client.ChangeProductQuantity(ctx, 1, 20, 0, ttl))
	assert.False(t, server.Exists("cart:1"))
}

func TestGetCartForCheckout_ReturnsAndDeletesCart(t *testing.T) {
	client, server := newTestClient(t)
	ctx := context.Background()

	_, _, err := client.InsertCartProduct(ctx, 5, 100, 2, 99, time.Hour)
	require.NoError(t, err)
	_, _, err = client.InsertCartProduct(ctx, 5, 200, 3, 99, time.Hour)
	require.NoError(t, err)

	cart, err := client.GetCartForCheckout(ctx, 5)
	require.NoError(t, err)
	assert.Equal(t, map[domain.ProductID]domain.Quantity{100: 2, 200: 3}, cart.Items)
	assert.False(t, server.Exists("cart:5"))

	_, err = client.GetCartForCheckout(ctx, 5)
	assert.True(t, errors.Is(err, customerrors.ErrCartEmpty))
}
