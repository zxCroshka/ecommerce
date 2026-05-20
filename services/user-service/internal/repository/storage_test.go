package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/testhelper"
)

func TestStorage_RegisterUserTX(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage, err := repository.NewForTests(ctx, tdb.Pool)
	require.NoError(t, err)

	t.Run("successfully register user", func(t *testing.T) {
		// Arrange
		email := "register@example.com"
		passHash := []byte("hashed_password")
		name := "Register User"
		isAdmin := false

		// Act
		userID, err := storage.RegisterUserTX(ctx, email, passHash, name, isAdmin)

		// Assert
		require.NoError(t, err)
		// ✅ Исправлено: проверяем что ID > 0, а не конкретное значение
		assert.Greater(t, userID, int64(0))

		// Verify user was created
		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, isAdmin, user.IsAdmin)
		assert.Equal(t, passHash, user.PassHash)
		assert.NotZero(t, user.CreatedAt)
	})

	t.Run("register with duplicate email fails", func(t *testing.T) {
		// Arrange
		email := "duplicate_tx@example.com"
		passHash := []byte("hash1")

		// First registration
		_, err := storage.RegisterUserTX(ctx, email, passHash, "User1", false)
		require.NoError(t, err)

		// Act - try to register again
		_, err = storage.RegisterUserTX(ctx, email, []byte("hash2"), "User2", false)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, customerrors.ErrUserExists)

		// Verify data wasn't changed (transaction rolled back)
		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, "User1", user.Name)
		assert.Equal(t, passHash, user.PassHash)
	})

	t.Run("register admin user", func(t *testing.T) {
		// Arrange
		email := "admin_register@example.com"

		// Act
		userID, err := storage.RegisterUserTX(ctx, email, []byte("hash"), "Admin User", true)

		// Assert
		require.NoError(t, err)
		// ✅ Исправлено: проверяем что ID > 0
		assert.Greater(t, userID, int64(0))

		// Verify admin flag
		isAdmin, err := storage.IsAdmin(ctx, userID)
		require.NoError(t, err)
		assert.True(t, isAdmin)
	})

	t.Run("concurrent registrations with same email", func(t *testing.T) {
		// Arrange
		email := "concurrent@example.com"

		// Act - try to register concurrently
		done := make(chan error, 2)
		go func() {
			_, err := storage.RegisterUserTX(ctx, email, []byte("hash1"), "User1", false)
			done <- err
		}()
		go func() {
			_, err := storage.RegisterUserTX(ctx, email, []byte("hash2"), "User2", false)
			done <- err
		}()

		// Assert - only one should succeed
		err1 := <-done
		err2 := <-done

		if err1 == nil {
			assert.Error(t, err2)
			assert.ErrorIs(t, err2, customerrors.ErrUserExists)
		} else {
			assert.ErrorIs(t, err1, customerrors.ErrUserExists)
			assert.NoError(t, err2)
		}

		// Verify only one user exists
		_, err := storage.User(ctx, email)
		require.NoError(t, err)
	})
}

func TestStorage_CRUDOperations(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage, err := repository.NewForTests(ctx, tdb.Pool)
	require.NoError(t, err)

	// Create a user first
	email := "crud@example.com"
	passHash := []byte("hash")
	name := "CRUD User"

	userID, err := storage.RegisterUserTX(ctx, email, passHash, name, false)
	require.NoError(t, err)
	// ✅ Исправлено: просто проверяем что ID > 0
	assert.Greater(t, userID, int64(0))

	t.Run("get user by email", func(t *testing.T) {
		// Act
		user, err := storage.User(ctx, email)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, userID, user.Id)
		assert.Equal(t, email, user.Email)
	})

	t.Run("update user name", func(t *testing.T) {
		// Act
		err := storage.UpdateName(ctx, userID, "Updated Name")

		// Assert
		require.NoError(t, err)

		// Verify
		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", user.Name)
	})

	t.Run("update user email", func(t *testing.T) {
		newEmail := "new_crud@example.com"

		// Act
		err := storage.UpdateEmail(ctx, userID, newEmail)

		// Assert
		require.NoError(t, err)

		// Old email should not exist
		_, err = storage.User(ctx, email)
		assert.ErrorIs(t, err, customerrors.ErrUserNotFound)

		// New email should exist
		user, err := storage.User(ctx, newEmail)
		require.NoError(t, err)
		assert.Equal(t, userID, user.Id)
	})

	t.Run("update user password", func(t *testing.T) {
		newHash := []byte("new_hash")

		// Act
		err := storage.UpdatePassword(ctx, userID, newHash)

		// Assert
		require.NoError(t, err)

		// Verify
		user, err := storage.User(ctx, "new_crud@example.com")
		require.NoError(t, err)
		assert.Equal(t, newHash, user.PassHash)
	})
}
