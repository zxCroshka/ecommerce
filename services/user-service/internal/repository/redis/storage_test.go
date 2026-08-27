package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	redisClient := &Client{client: client}
	return redisClient, mr
}

func TestSaveRefreshToken(t *testing.T) {
	redisClient, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("successfully save refresh token", func(t *testing.T) {
		userID := int64(123)
		tokenID := "token-123"
		ttl := 15 * time.Minute

		err := redisClient.SaveRefreshToken(ctx, userID, tokenID, ttl)

		require.NoError(t, err)

		// Verify token exists
		key := "refresh:123:token-123"
		exists, err := redisClient.client.Exists(ctx, key).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, exists)

		// Verify TTL
		ttlRemaining, err := redisClient.client.TTL(ctx, key).Result()
		require.NoError(t, err)
		assert.Greater(t, ttlRemaining, time.Duration(0))
	})
}

func TestValidateRefreshToken(t *testing.T) {
	redisClient, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("existing token returns true", func(t *testing.T) {
		userID := int64(123)
		tokenID := "valid-token"

		err := redisClient.SaveRefreshToken(ctx, userID, tokenID, 15*time.Minute)
		require.NoError(t, err)

		exists, err := redisClient.ValidateRefreshToken(ctx, userID, tokenID)

		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("non-existing token returns false", func(t *testing.T) {
		exists, err := redisClient.ValidateRefreshToken(ctx, int64(999), "non-existing")

		require.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestDeleteRefreshToken(t *testing.T) {
	redisClient, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("successfully delete existing token", func(t *testing.T) {
		userID := int64(123)
		tokenID := "delete-token"

		err := redisClient.SaveRefreshToken(ctx, userID, tokenID, 15*time.Minute)
		require.NoError(t, err)

		err = redisClient.DeleteRefreshToken(ctx, userID, tokenID)
		require.NoError(t, err)

		exists, err := redisClient.ValidateRefreshToken(ctx, userID, tokenID)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("delete non-existing token does not error", func(t *testing.T) {
		err := redisClient.DeleteRefreshToken(ctx, int64(999), "non-existing")

		// Redis DEL on non-existing key returns 0, not an error
		assert.NoError(t, err)
	})
}

func TestRotateRefreshToken(t *testing.T) {
	redisClient, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	const userID int64 = 123
	const oldTokenID = "old-token"
	const newTokenID = "new-token"
	ttl := 7 * 24 * time.Hour

	require.NoError(t, redisClient.SaveRefreshToken(ctx, userID, oldTokenID, ttl))

	rotated, err := redisClient.RotateRefreshToken(ctx, userID, oldTokenID, newTokenID, ttl)
	require.NoError(t, err)
	assert.True(t, rotated)

	oldExists, err := redisClient.ValidateRefreshToken(ctx, userID, oldTokenID)
	require.NoError(t, err)
	assert.False(t, oldExists)

	newExists, err := redisClient.ValidateRefreshToken(ctx, userID, newTokenID)
	require.NoError(t, err)
	assert.True(t, newExists)

	remainingTTL, err := redisClient.client.TTL(ctx, "refresh:123:new-token").Result()
	require.NoError(t, err)
	assert.Greater(t, remainingTTL, time.Duration(0))
}

func TestRotateRefreshToken_CanConsumeOldTokenOnlyOnce(t *testing.T) {
	redisClient, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	const userID int64 = 123
	require.NoError(t, redisClient.SaveRefreshToken(ctx, userID, "old-token", time.Hour))

	firstRotation, err := redisClient.RotateRefreshToken(ctx, userID, "old-token", "new-token-1", time.Hour)
	require.NoError(t, err)
	assert.True(t, firstRotation)

	secondRotation, err := redisClient.RotateRefreshToken(ctx, userID, "old-token", "new-token-2", time.Hour)
	require.NoError(t, err)
	assert.False(t, secondRotation)

	secondTokenExists, err := redisClient.ValidateRefreshToken(ctx, userID, "new-token-2")
	require.NoError(t, err)
	assert.False(t, secondTokenExists)
}

func TestAddToBlacklist(t *testing.T) {
	redisClient, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("successfully add token to blacklist", func(t *testing.T) {
		tokenID := "blacklist-token"
		ttl := 15 * time.Minute

		err := redisClient.AddToBlacklist(ctx, tokenID, ttl)

		require.NoError(t, err)

		key := "blacklist:blacklist-token"
		exists, err := redisClient.client.Exists(ctx, key).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 1, exists)
	})

	t.Run("adding same token twice is fine", func(t *testing.T) {
		tokenID := "duplicate-token"
		ttl := 10 * time.Minute

		err := redisClient.AddToBlacklist(ctx, tokenID, ttl)
		require.NoError(t, err)

		err = redisClient.AddToBlacklist(ctx, tokenID, ttl)
		// Should not error, just overwrite
		assert.NoError(t, err)
	})
}

func TestIsBlacklisted(t *testing.T) {
	redisClient, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	t.Run("blacklisted token returns true", func(t *testing.T) {
		tokenID := "check-blacklist"

		err := redisClient.AddToBlacklist(ctx, tokenID, 15*time.Minute)
		require.NoError(t, err)

		isBlacklisted, err := redisClient.IsBlacklisted(ctx, tokenID)

		require.NoError(t, err)
		assert.True(t, isBlacklisted)
	})

	t.Run("non-blacklisted token returns false", func(t *testing.T) {
		isBlacklisted, err := redisClient.IsBlacklisted(ctx, "not-blacklisted")

		require.NoError(t, err)
		assert.False(t, isBlacklisted)
	})
}

// Benchmark tests
func BenchmarkSaveRefreshToken(b *testing.B) {
	mr := miniredis.RunT(b)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisClient := &Client{client: client}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = redisClient.SaveRefreshToken(ctx, int64(i), "token", 15*time.Minute)
	}
}

func BenchmarkValidateRefreshToken(b *testing.B) {
	mr := miniredis.RunT(b)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	redisClient := &Client{client: client}
	ctx := context.Background()

	// Setup
	_ = redisClient.SaveRefreshToken(ctx, 123, "bench-token", 15*time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = redisClient.ValidateRefreshToken(ctx, 123, "bench-token")
	}
}
