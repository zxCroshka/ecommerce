package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/testhelper"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/users"
)

func TestStorage_SaveUser(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func(){_ = tdb.TruncateTables(ctx)}()

	storage := users.New(tdb.Pool)

	t.Run("successfully create user", func(t *testing.T) {
		// Arrange
		email := "test@example.com"
		passHash := []byte("hashed_password")
		name := "Test User"
		isAdmin := false
		createdAt := time.Now()

		// Act
		id, err := storage.SaveUser(ctx, email, passHash, name, isAdmin, createdAt)

		// Assert
		require.NoError(t, err)
		assert.Greater(t, id, 0)

		// Verify user was saved
		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, isAdmin, user.IsAdmin)
		assert.Equal(t, passHash, user.PassHash)
	})

	t.Run("duplicate email returns error", func(t *testing.T) {
		// Arrange
		email := "duplicate@example.com"
		passHash := []byte("hash1")

		// First user
		_, err := storage.SaveUser(ctx, email, passHash, "User1", false, time.Now())
		require.NoError(t, err)

		// Act - try to create duplicate
		_, err = storage.SaveUser(ctx, email, []byte("hash2"), "User2", false, time.Now())

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, customerrors.ErrUserExists)
	})

	t.Run("create admin user", func(t *testing.T) {
		// Arrange
		email := "admin@example.com"

		// Act
		id, err := storage.SaveUser(ctx, email, []byte("hash"), "Admin", true, time.Now())

		// Assert
		require.NoError(t, err)
		assert.Greater(t, id, 0)

		// Verify admin flag
		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.True(t, user.IsAdmin)
	})
}

func TestStorage_User(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func(){_=tdb.TruncateTables(ctx)}()

	storage := users.New(tdb.Pool)

	t.Run("get existing user", func(t *testing.T) {
		// Arrange - create user first
		email := "find@example.com"
		passHash := []byte("hash")
		name := "Find User"
		isAdmin := false
		createdAt := time.Now()

		_, err := storage.SaveUser(ctx, email, passHash, name, isAdmin, createdAt)
		require.NoError(t, err)

		// Act
		user, err := storage.User(ctx, email)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, isAdmin, user.IsAdmin)
		assert.Equal(t, passHash, user.PassHash)
		assert.NotZero(t, user.CreatedAt)
	})

	t.Run("get non-existent user", func(t *testing.T) {
		// Act
		_, err := storage.User(ctx, "nonexistent@example.com")

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, customerrors.ErrUserNotFound)
	})
}

func TestStorage_IsAdmin(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func(){_=tdb.TruncateTables(ctx)}()

	storage := users.New(tdb.Pool)

	t.Run("regular user is not admin", func(t *testing.T) {
		// Arrange
		email := "regular@example.com"
		_, err := storage.SaveUser(ctx, email, []byte("hash"), "Regular", false, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)

		// Act
		isAdmin, err := storage.IsAdmin(ctx, user.Id)

		// Assert
		require.NoError(t, err)
		assert.False(t, isAdmin)
	})

	t.Run("admin user is admin", func(t *testing.T) {
		// Arrange
		email := "admin@example.com"
		_, err := storage.SaveUser(ctx, email, []byte("hash"), "Admin", true, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)

		// Act
		isAdmin, err := storage.IsAdmin(ctx, user.Id)

		// Assert
		require.NoError(t, err)
		assert.True(t, isAdmin)
	})
}

func TestStorage_UpdateName(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func(){_=tdb.TruncateTables(ctx)}()

	storage := users.New(tdb.Pool)

	t.Run("update user name", func(t *testing.T) {
		// Arrange
		email := "updatename@example.com"
		_, err := storage.SaveUser(ctx, email, []byte("hash"), "Old Name", false, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)

		// Act
		err = storage.UpdateName(ctx, user.Id, "New Name")

		// Assert
		require.NoError(t, err)

		// Verify update
		updatedUser, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, "New Name", updatedUser.Name)
	})
}

func TestStorage_UpdateEmail(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func(){_=tdb.TruncateTables(ctx)}()

	storage := users.New(tdb.Pool)

	t.Run("update user email", func(t *testing.T) {
		// Arrange
		oldEmail := "old@example.com"
		newEmail := "new@example.com"
		_, err := storage.SaveUser(ctx, oldEmail, []byte("hash"), "User", false, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, oldEmail)
		require.NoError(t, err)

		// Act
		err = storage.UpdateEmail(ctx, user.Id, newEmail)

		// Assert
		require.NoError(t, err)

		// Old email should not exist
		_, err = storage.User(ctx, oldEmail)
		assert.ErrorIs(t, err, customerrors.ErrUserNotFound)

		// New email should exist
		updatedUser, err := storage.User(ctx, newEmail)
		require.NoError(t, err)
		assert.Equal(t, newEmail, updatedUser.Email)
	})
}

func TestStorage_UpdatePassword(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func(){_=tdb.TruncateTables(ctx)}()

	storage := users.New(tdb.Pool)

	t.Run("update user password", func(t *testing.T) {
		// Arrange
		email := "updatepass@example.com"
		oldHash := []byte("old_hash")
		_, err := storage.SaveUser(ctx, email, oldHash, "User", false, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, oldHash, user.PassHash)

		// Act
		newHash := []byte("new_hash")
		err = storage.UpdatePassword(ctx, user.Id, newHash)

		// Assert
		require.NoError(t, err)

		// Verify update
		updatedUser, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, newHash, updatedUser.PassHash)
	})
}
