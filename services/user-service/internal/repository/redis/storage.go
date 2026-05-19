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
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Client{client: client}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) SaveRefreshToken(ctx context.Context, userID int64, tokenID string, ttl time.Duration) error {
	const op = "storage.redis.SaveRefreshToken"
	key := fmt.Sprintf("refresh:%d:%s", userID, tokenID)
	tokenInfo := map[string]interface{}{
		"user_id":  userID,
		"token_id": tokenID,
	}
	data,_ := json.Marshal(tokenInfo)

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
		return false, fmt.Errorf("%s: %w",op, err)
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
