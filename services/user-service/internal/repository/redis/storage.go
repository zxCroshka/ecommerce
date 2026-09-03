package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
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

	return &Client{client: client}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

var rotateRefreshTokenScript = redis.NewScript(`
	local oldKey = KEYS[1]
	local newKey = KEYS[2]
	local newValue = ARGV[1]
	local ttl = tonumber(ARGV[2])

	if redis.call("EXISTS", oldKey) == 0 then
		return 0
	end

	redis.call("DEL", oldKey)
	redis.call("SET", newKey, newValue, "EX", ttl)

	return 1
`)

func (c *Client) SaveRefreshToken(ctx context.Context, userID int64, tokenID string, ttl time.Duration) error {
	const op = "storage.redis.SaveRefreshToken"
	key := fmt.Sprintf("refresh:%d:%s", userID, tokenID)
	tokenInfo := map[string]interface{}{
		"user_id":  userID,
		"token_id": tokenID,
	}
	data, err := json.Marshal(tokenInfo)
	if err != nil {
		return fmt.Errorf("%s: marshal token info: %w", op, err)
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *Client) ValidateRefreshToken(ctx context.Context, userID int64, tokenID string) (bool, error) {
	const op = "storage.redis.ValidateRefreshToken"
	key := fmt.Sprintf("refresh:%d:%s", userID, tokenID)

	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	return exists == 1, nil

}

func (c *Client) DeleteRefreshToken(ctx context.Context, userID int64, tokenID string) error {
	const op = "storage.redis.DeleteRefreshToken"
	key := fmt.Sprintf("refresh:%d:%s", userID, tokenID)

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *Client) RotateRefreshToken(ctx context.Context, userID int64, oldTokenID string, newTokenID string, ttl time.Duration) (bool, error) {
	const op = "storage.redis.RotateRefreshToken"
	oldKey := fmt.Sprintf("refresh:%d:%s", userID, oldTokenID)
	newKey := fmt.Sprintf("refresh:%d:%s", userID, newTokenID)
	tokenInfo := map[string]interface{}{
		"user_id":  userID,
		"token_id": newTokenID,
	}
	value, err := json.Marshal(tokenInfo)
	if err != nil {
		return false, fmt.Errorf("%s: marshal token info: %w", op, err)
	}
	res, err := rotateRefreshTokenScript.Run(ctx, c.client, []string{oldKey, newKey}, value, int64(ttl.Seconds())).Int()
	if err != nil {
		return false, fmt.Errorf("%s: execute script: %w", op, err)
	}
	return res == 1, nil
}

func (c *Client) AddToBlacklist(ctx context.Context, tokenID string, ttl time.Duration) error {
	key := fmt.Sprintf("blacklist:%s", tokenID)

	err := c.client.Set(ctx, key, "blocked", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to add token to blacklist: %w", err)
	}

	return nil
}

func (c *Client) IsBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	key := fmt.Sprintf("blacklist:%s", tokenID)

	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}

	return exists == 1, nil
}
