package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
)

const cacheGenerationKey = "products:cache-generation"

var setIfGenerationScript = redis.NewScript(`
	local current = tonumber(redis.call("GET", KEYS[1]) or "0")
	local expected = tonumber(ARGV[1])
	if current ~= expected then
		return 0
	end
	redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[3])
	return 1
`)

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
		_ = client.Close()
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
	generation, err := c.CacheGeneration(ctx)
	if err != nil {
		return err
	}
	_, err = c.SetListProductsCacheIfGeneration(ctx, key, products, total, generation)
	return err
}

func (c *Client) SetListProductsCacheIfGeneration(
	ctx context.Context,
	key string,
	products []*domain.Product,
	total int64,
	generation int64,
) (bool, error) {
	const op = "storage.redis.SetListProductsCache"
	cacheData := NewCache(generation, products, total)

	data, err := json.Marshal(cacheData)
	if err != nil {
		return false, fmt.Errorf("%s: failed to marshal %w", op, customerrors.ErrMarshal)
	}
	set, err := setIfGenerationScript.Run(
		ctx,
		c.client,
		[]string{cacheGenerationKey, key},
		generation,
		data,
		c.productsListTTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, fmt.Errorf("%s: failed to set cache %w", op, customerrors.ErrSetCache)
	}
	return set == 1, nil
}

func (c *Client) GetListProductsCache(ctx context.Context, key string) ([]*domain.Product, int64, error) {
	const op = "storage.redis.GetListProductsCache"
	values, err := c.client.MGet(ctx, cacheGenerationKey, key).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrGetCache)
	}
	if len(values) != 2 || values[1] == nil {
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrCacheMiss)
	}
	generation, err := parseGeneration(values[0])
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrGetCache)
	}
	data, ok := values[1].(string)
	if !ok {
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrUnmarshal)
	}
	var cacheDate Cache
	if err := json.Unmarshal([]byte(data), &cacheDate); err != nil {
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrUnmarshal)
	}
	if cacheDate.Generation != generation {
		return nil, 0, fmt.Errorf("%s: %w", op, customerrors.ErrCacheMiss)
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
	generation, err := c.CacheGeneration(ctx)
	if err != nil {
		return err
	}
	_, err = c.SetProductCacheIfGeneration(ctx, productID, product, generation)
	return err
}

func (c *Client) SetProductCacheIfGeneration(
	ctx context.Context,
	productID int64,
	product *domain.Product,
	generation int64,
) (bool, error) {
	const op = "storage.redis.SetProductCache"

	key := fmt.Sprintf("product:%d", productID)
	data, err := json.Marshal(ProductCache{Generation: generation, Product: product})
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, customerrors.ErrMarshal)
	}
	set, err := setIfGenerationScript.Run(
		ctx,
		c.client,
		[]string{cacheGenerationKey, key},
		generation,
		data,
		c.productTTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, customerrors.ErrSetCache)
	}

	return set == 1, nil
}

func (c *Client) GetProductCache(ctx context.Context, productID int64) (*domain.Product, error) {
	const op = "storage.redis.GetProductCache"

	key := fmt.Sprintf("product:%d", productID)
	values, err := c.client.MGet(ctx, cacheGenerationKey, key).Result()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrGetCache)
	}
	if len(values) != 2 || values[1] == nil {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrCacheMiss)
	}
	generation, err := parseGeneration(values[0])
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrGetCache)
	}
	data, ok := values[1].(string)
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrUnmarshal)
	}

	var cached ProductCache
	if err := json.Unmarshal([]byte(data), &cached); err != nil || cached.Product == nil {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrUnmarshal)
	}
	if cached.Generation != generation {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrCacheMiss)
	}

	return cached.Product, nil
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
	if err := c.client.Incr(ctx, cacheGenerationKey).Err(); err != nil {
		return fmt.Errorf("%s: increment cache generation: %w", op, err)
	}
	return nil
}

func (c *Client) CacheGeneration(ctx context.Context) (int64, error) {
	value, err := c.client.Get(ctx, cacheGenerationKey).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get product cache generation: %w", err)
	}
	generation, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse product cache generation: %w", err)
	}
	return generation, nil
}

func parseGeneration(value any) (int64, error) {
	if value == nil {
		return 0, nil
	}
	text, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("unexpected generation type %T", value)
	}
	return strconv.ParseInt(text, 10, 64)
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
