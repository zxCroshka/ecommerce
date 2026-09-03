package redis

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
)

func TestGenerationFencePreventsStaleFillAfterInvalidation(t *testing.T) {
	server := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	client := &Client{
		client: redisClient, productTTL: time.Minute, productsListTTL: time.Minute,
	}
	ctx := context.Background()
	oldProduct := &domain.Product{Id: 42, Name: "old", IsActive: true}

	generationBeforeRead, err := client.CacheGeneration(ctx)
	require.NoError(t, err)
	require.NoError(t, client.InvalidateAllProductCache(ctx))

	set, err := client.SetProductCacheIfGeneration(ctx, 42, oldProduct, generationBeforeRead)
	require.NoError(t, err)
	require.False(t, set, "a fill that started before invalidation must be fenced out")
	_, err = client.GetProductCache(ctx, 42)
	require.ErrorIs(t, err, customerrors.ErrCacheMiss)
}

func TestGenerationCacheRoundTrip(t *testing.T) {
	server := miniredis.RunT(t)
	redisClient := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	client := &Client{
		client: redisClient, productTTL: time.Minute, productsListTTL: time.Minute,
	}
	ctx := context.Background()
	product := &domain.Product{Id: 42, Name: "current", IsActive: true}

	generation, err := client.CacheGeneration(ctx)
	require.NoError(t, err)
	set, err := client.SetProductCacheIfGeneration(ctx, 42, product, generation)
	require.NoError(t, err)
	require.True(t, set)

	loaded, err := client.GetProductCache(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, product.Name, loaded.Name)
}
