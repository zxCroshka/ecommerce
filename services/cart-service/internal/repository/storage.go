package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/domain"
)

type Client struct {
	client *redis.Client
}

type Config struct {
	Host     string
	Port     int
	Password string
	DB       int
}

var (
	addToCartScript = redis.NewScript(`
		local cartKey = KEYS[1]
		local productId = ARGV[1]
		local quantity = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])
		local maxQuantity = tonumber(ARGV[4])
		local currentQuantity = redis.call("HGET",cartKey, productId)
		currentQuantity = currentQuantity and tonumber(currentQuantity) or 0
		local newQuantity = quantity + currentQuantity
		if newQuantity > maxQuantity then
			return {-1,currentQuantity}
		end


		if newQuantity <= 0 then
			redis.call("HDEL", cartKey, productId)
		else
			redis.call("HSET", cartKey, productId, newQuantity)
		end
		
		redis.call("EXPIRE", cartKey, ttl)
		return {newQuantity,currentQuantity}

	`)
	changeProductQuantityScript = redis.NewScript(`
		local cartKey = KEYS[1]
		local productId = ARGV[1]
		local quantity = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])
		local currentQuantity = redis.call("HGET", cartKey, productId)
		currentQuantity = currentQuantity and tonumber(currentQuantity) or 0

		if not quantity then
			return redis.error_reply("quantity must be a number")
		end
		if not ttl or ttl <= 0 then
			return redis.error_reply("ttl must be positive")
		end

		if quantity <= 0 then
			redis.call("HDEL", cartKey, productId)
		else
			redis.call("HSET", cartKey, productId, quantity)
		end
		redis.call("EXPIRE", cartKey, ttl)
		return {quantity,currentQuantity}
	
	`)
	checkoutCartScript = redis.NewScript(`
		local cartKey = KEYS[1]
		local items = redis.call("HGETALL", cartKey)
		if #items == 0 then
			return {}
		end
		redis.call("DEL", cartKey)
		return items
	`)
)

func NewClient(cfg Config) (*Client, error) {
	const op = "redis.NewClient"
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
		return nil, fmt.Errorf("%s: %w: %w", op, customerrors.ErrRedisConnection, err)
	}

	return &Client{client: client}, nil
}

func (c *Client) InsertCartProduct(ctx context.Context, userId int64, productId int64, quantity, maxQuantity int64, ttl time.Duration) (int64, int64, error) {
	const op = "repository.InsertCartProduct"
	cartKey := fmt.Sprintf("cart:%d", userId)
	result, err := addToCartScript.Run(ctx, c.client, []string{cartKey}, productId, quantity, int(ttl.Seconds()), maxQuantity).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w: %w", op, customerrors.ErrScriptExecute, err)
	}

	resArr, ok := result.([]interface{})
	if !ok || len(resArr) != 2 {
		return 0, 0, fmt.Errorf("%s: %w", op, customerrors.ErrUnexpectedResult)
	}

	newQuantity, ok1 := resArr[0].(int64)
	oldQuantity, ok2 := resArr[1].(int64)
	if !ok1 || !ok2 {
		return 0, 0, fmt.Errorf("%s: %w", op, customerrors.ErrUnexpectedQuantityType)
	}
	if newQuantity == -1 {
		return 0, oldQuantity, fmt.Errorf("%s: %w", op, customerrors.ErrQuantityExceedsLimit)
	}
	return newQuantity, oldQuantity, nil
}

func (c *Client) DeleteCartProduct(ctx context.Context, userId int64, productId int64, ttl time.Duration) (int64, error) {
	const op = "repository.DeleteCartProduct"
	oldQuantity, err := c.runChangeProductQuantity(ctx, userId, productId, 0, ttl)
	if err != nil {
		return 0, fmt.Errorf("%s: %w: %w", op, customerrors.ErrProductDelete, err)
	}
	return oldQuantity, nil
}

func (c *Client) GetCartProducts(ctx context.Context, userId int64) (*domain.Cart, error) {
	const op = "repository.GetCartProducts"
	cartKey := fmt.Sprintf("cart:%d", userId)
	result, err := c.client.HGetAll(ctx, cartKey).Result()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", op, customerrors.ErrGetProducts, err)
	}
	cart, err := domain.NewCart(result)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", op, customerrors.ErrInvalidCartData, err)
	}
	return cart, nil
}

func (c *Client) ClearCart(ctx context.Context, userId int64) error {
	const op = "repository.ClearCart"
	cartKey := fmt.Sprintf("cart:%d", userId)
	if err := c.client.Del(ctx, cartKey).Err(); err != nil {
		return fmt.Errorf("%s: %w: %w", op, customerrors.ErrClearCart, err)
	}
	return nil
}

func (c *Client) GetCartForCheckout(ctx context.Context, userID int64) (*domain.Cart, error) {
	const op = "repository.GetCartForCheckout"
	cartKey := fmt.Sprintf("cart:%d", userID)

	result, err := checkoutCartScript.Run(ctx, c.client, []string{cartKey}).Result()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", op, customerrors.ErrCheckoutCart, err)
	}
	values, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrUnexpectedResult)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrCartEmpty)
	}
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrUnexpectedResult)
	}

	items := make(map[string]string, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		items[fmt.Sprint(values[i])] = fmt.Sprint(values[i+1])
	}
	cart, err := domain.NewCart(items)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", op, customerrors.ErrInvalidCartData, err)
	}
	return cart, nil
}

func (c *Client) ChangeProductQuantity(ctx context.Context, userID int64, productID, newQuantity int64, ttl time.Duration) error {
	const op = "repository.ChangeProductQuantity"
	if _, err := c.runChangeProductQuantity(ctx, userID, productID, newQuantity, ttl); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *Client) runChangeProductQuantity(
	ctx context.Context,
	userID, productID, newQuantity int64,
	ttl time.Duration,
) (int64, error) {
	cartKey := fmt.Sprintf("cart:%d", userID)
	result, err := changeProductQuantityScript.Run(
		ctx, c.client, []string{cartKey}, productID, newQuantity, int(ttl.Seconds()),
	).Result()
	if err != nil {
		return 0, fmt.Errorf("%w: %w", customerrors.ErrScriptExecute, err)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return 0, customerrors.ErrUnexpectedResult
	}
	oldQuantity, ok := values[1].(int64)
	if !ok {
		return 0, customerrors.ErrUnexpectedQuantityType
	}
	return oldQuantity, nil
}

func (c *Client) Close() error {
	const op = "repository.Close"
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("%s: %w: %w", op, customerrors.ErrCloseRedis, err)
	}
	return nil
}
