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
		local revisionKey = KEYS[2]
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


		if newQuantity == currentQuantity then
			return {newQuantity,currentQuantity}
		end

		if newQuantity <= 0 then
			redis.call("HDEL", cartKey, productId)
		else
			redis.call("HSET", cartKey, productId, newQuantity)
		end
		
		redis.call("INCR", revisionKey)
		redis.call("EXPIRE", cartKey, ttl)
		redis.call("EXPIRE", revisionKey, ttl)
		return {newQuantity,currentQuantity}

	`)
	changeProductQuantityScript = redis.NewScript(`
		local cartKey = KEYS[1]
		local revisionKey = KEYS[2]
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

		if quantity == currentQuantity then
			return {quantity,currentQuantity}
		end

		if quantity <= 0 then
			redis.call("HDEL", cartKey, productId)
		else
			redis.call("HSET", cartKey, productId, quantity)
		end
		redis.call("INCR", revisionKey)
		redis.call("EXPIRE", cartKey, ttl)
		redis.call("EXPIRE", revisionKey, ttl)
		return {quantity,currentQuantity}
	
	`)
	checkoutCartScript = redis.NewScript(`
		local cartKey = KEYS[1]
		local revisionKey = KEYS[2]
		local ttl = tonumber(ARGV[1])
		local items = redis.call("HGETALL", cartKey)
		if #items == 0 then
			return {}
		end
		local revision = redis.call("GET", revisionKey)
		if not revision then
			revision = 1
			redis.call("SET", revisionKey, revision, "EX", ttl)
		end
		local result = {tonumber(revision)}
		for _, value in ipairs(items) do
			table.insert(result, value)
		end
		return result
	`)
	clearCartIfUnchangedScript = redis.NewScript(`
		local cartKey = KEYS[1]
		local revisionKey = KEYS[2]
		local expectedRevision = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		local currentRevision = tonumber(redis.call("GET", revisionKey) or "0")
		if currentRevision ~= expectedRevision then
			return 0
		end
		if redis.call("HLEN", cartKey) == 0 then
			return 0
		end
		redis.call("DEL", cartKey)
		redis.call("INCR", revisionKey)
		redis.call("EXPIRE", revisionKey, ttl)
		return 1
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
	revisionKey := fmt.Sprintf("cart:%d:revision", userId)
	result, err := addToCartScript.Run(ctx, c.client, []string{cartKey, revisionKey}, productId, quantity, int(ttl.Seconds()), maxQuantity).Result()
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

func (c *Client) GetCartSnapshot(ctx context.Context, userID int64, ttl time.Duration) (*domain.CartSnapshot, error) {
	const op = "repository.GetCartForCheckout"
	cartKey := fmt.Sprintf("cart:%d", userID)
	revisionKey := fmt.Sprintf("cart:%d:revision", userID)

	result, err := checkoutCartScript.Run(
		ctx, c.client, []string{cartKey, revisionKey}, int(ttl.Seconds()),
	).Result()
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
	if len(values) < 3 || (len(values)-1)%2 != 0 {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrUnexpectedResult)
	}
	revision, ok := values[0].(int64)
	if !ok || revision <= 0 {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrUnexpectedResult)
	}

	items := make(map[string]string, (len(values)-1)/2)
	for i := 1; i < len(values); i += 2 {
		items[fmt.Sprint(values[i])] = fmt.Sprint(values[i+1])
	}
	cart, err := domain.NewCart(items)
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %w", op, customerrors.ErrInvalidCartData, err)
	}
	return &domain.CartSnapshot{Items: cart.Items, Revision: revision}, nil
}

func (c *Client) ClearCartIfUnchanged(
	ctx context.Context,
	userID, revision int64,
	ttl time.Duration,
) (bool, error) {
	const op = "repository.ClearCartIfUnchanged"
	cartKey := fmt.Sprintf("cart:%d", userID)
	revisionKey := fmt.Sprintf("cart:%d:revision", userID)
	cleared, err := clearCartIfUnchangedScript.Run(
		ctx,
		c.client,
		[]string{cartKey, revisionKey},
		revision,
		int(ttl.Seconds()),
	).Int()
	if err != nil {
		return false, fmt.Errorf("%s: %w: %w", op, customerrors.ErrConditionalClear, err)
	}
	return cleared == 1, nil
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
	revisionKey := fmt.Sprintf("cart:%d:revision", userID)
	result, err := changeProductQuantityScript.Run(
		ctx, c.client, []string{cartKey, revisionKey}, productID, newQuantity, int(ttl.Seconds()),
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
