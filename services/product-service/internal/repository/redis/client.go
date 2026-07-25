package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
)

type Client struct {
	client          *redis.Client
	productTTL      time.Duration
	productsListTTL time.Duration
}

type Config struct {
	Host            string
	Port            int
	Password        string
	DB              int
	ProductTTL      time.Duration
	ProductsListTTL time.Duration
}

func NewClient(cfg Config) (*Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	if cfg.ProductTTL <= 0 {
		cfg.ProductTTL = 5 * time.Minute
	}
	if cfg.ProductsListTTL <= 0 {
		cfg.ProductsListTTL = 5 * time.Minute
	}
	return &Client{client: client, productTTL: cfg.ProductTTL, productsListTTL: cfg.ProductsListTTL}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) SetListProductsCache(ctx context.Context, key string, products []*domain.Product, total int64) error {
	const op = "storage.redis.SetListProductsCache"
	cacheData := NewCache(products, total)

	data, err := json.Marshal(cacheData)
	if err != nil {
		return fmt.Errorf("%s: failed to marshal %w", op, customerrors.ErrMarshal)
	}
	if err := c.client.Set(ctx, key, data, c.productsListTTL).Err(); err != nil {
		return fmt.Errorf("%s: failed to set cache %w", op, customerrors.ErrSetCache)
	}
	return nil
}

func (c *Client) GetListProductsCache(ctx context.Context, key string) ([]*domain.Product, int64, error) {
	const op = "storage.redis.GetListProductsCache"
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrCacheMiss)
		}
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrGetCache)
	}
	var cacheDate Cache
	if err := json.Unmarshal(data, &cacheDate); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrUnmarshal)
	}
	return cacheDate.Products, cacheDate.Total, nil
}

func (c *Client) InvalidateProductsCache(ctx context.Context, key string) error {

	const op = "storage.redis.InvalidateProductsCache"

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%s: failed to delete cache: %w", op, err)
	}

	return nil
}

func (c *Client) InvalidateProductsCacheByPattern(ctx context.Context, pattern string) error {
	const op = "storage.redis.InvalidateProductsCacheByPattern"

	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("%s: failed to get keys: %w", op, err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("%s: failed to delete keys: %w", op, err)
		}
	}

	return nil
}

func (c *Client) SetProductCache(ctx context.Context, productID int64, product *domain.Product) error {
	const op = "storage.redis.SetProductCache"

	key := fmt.Sprintf("product:%d", productID)
	data, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("%s: %w", op, customerrors.ErrMarshal)
	}
	if err := c.client.Set(ctx, key, data, c.productTTL).Err(); err != nil {
		return fmt.Errorf("%s: %w", op, customerrors.ErrSetCache)
	}

	return nil
}

func (c *Client) GetProductCache(ctx context.Context, productID int64) (*domain.Product, error) {
	const op = "storage.redis.GetProductCache"

	key := fmt.Sprintf("product:%d", productID)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("%s: %w", op, customerrors.ErrCacheMiss)
		}
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrGetCache)
	}

	var product domain.Product
	if err := json.Unmarshal(data, &product); err != nil {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrUnmarshal)
	}

	return &product, nil
}

func (c *Client) InvalidateProductCache(ctx context.Context, productID int64) error {
	const op = "storage.redis.InvalidateProductCache"

	key := fmt.Sprintf("product:%d", productID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%s: failed to delete cache: %w", op, err)
	}

	return nil
}

func (c *Client) InvalidateAllProductCache(ctx context.Context) error {
	const op = "storage.redis.InvalidateAllProductCache"

	pattern := "products:*"
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("%s: failed to get keys: %w", op, err)
	}

	productKeys, err := c.scanKeys(ctx, "product:*")
	if err != nil {
		return fmt.Errorf("%s: failed to get product keys: %w", op, err)
	}
	keys = append(keys, productKeys...)

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("%s: failed to delete keys: %w", op, err)
		}
	}

	return nil
}

func (c *Client) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var cursor uint64
	var result []string
	for {
		keys, next, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		result = append(result, keys...)
		cursor = next
		if cursor == 0 {
			return result, nil
		}
	}
}

func (c *Client) BuildListCacheKey(filter domain.ProductFilter, sort domain.SortField, order domain.SortOrder, limit, offset int) string {
	key := "products:list"

	if filter.Category != nil {
		key += fmt.Sprintf(":cat_%s", *filter.Category)
	}

	if filter.IsActive != nil {
		key += fmt.Sprintf(":active_%t", *filter.IsActive)
	}

	key += fmt.Sprintf(":%s:%s:%d:%d", sort, order, limit, offset)

	return key
}
