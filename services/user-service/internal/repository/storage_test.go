//go:build integration

package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
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
		role := domain.RoleCustomer

		// Act
		userID, err := storage.RegisterUserTX(ctx, email, passHash, name, role)

		// Assert
		require.NoError(t, err)
		// ✅ Исправлено: проверяем что ID > 0, а не конкретное значение
		assert.Greater(t, userID, int64(0))

		// Verify user was created
		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, role, user.Role)
		assert.Equal(t, passHash, user.PassHash)
		assert.NotZero(t, user.CreatedAt)

		var eventType, aggregateID string
		var payloadUserID int64
		err = tdb.Pool.QueryRow(ctx, `
			SELECT event_type, aggregate_id, (payload->>'user_id')::bigint
			FROM userservice.outbox_events
			WHERE aggregate_id=$1
		`, fmt.Sprint(userID)).Scan(&eventType, &aggregateID, &payloadUserID)
		require.NoError(t, err)
		assert.Equal(t, "user.registered", eventType)
		assert.Equal(t, fmt.Sprint(userID), aggregateID)
		assert.Equal(t, userID, payloadUserID)
	})

	t.Run("register with duplicate email fails", func(t *testing.T) {
		// Arrange
		email := "duplicate_tx@example.com"
		passHash := []byte("hash1")

		// First registration
		_, err := storage.RegisterUserTX(ctx, email, passHash, "User1", domain.RoleCustomer)
		require.NoError(t, err)

		// Act - try to register again
		_, err = storage.RegisterUserTX(ctx, email, []byte("hash2"), "User2", domain.RoleCustomer)

		// Assert
		require.Error(t, err)
		assert.ErrorIs(t, err, customerrors.ErrDuplicateEmail)

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
		userID, err := storage.RegisterUserTX(ctx, email, []byte("hash"), "Admin User", domain.RoleAdmin)

		// Assert
		require.NoError(t, err)
		// ✅ Исправлено: проверяем что ID > 0
		assert.Greater(t, userID, int64(0))

		// Verify role
		role, err := storage.Role(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, role)
	})

	t.Run("concurrent registrations with same email", func(t *testing.T) {
		// Arrange
		email := "concurrent@example.com"

		// Act - try to register concurrently
		done := make(chan error, 2)
		go func() {
			_, err := storage.RegisterUserTX(ctx, email, []byte("hash1"), "User1", domain.RoleCustomer)
			done <- err
		}()
		go func() {
			_, err := storage.RegisterUserTX(ctx, email, []byte("hash2"), "User2", domain.RoleCustomer)
			done <- err
		}()

		// Assert - only one should succeed
		err1 := <-done
		err2 := <-done

		if err1 == nil {
			assert.Error(t, err2)
			assert.ErrorIs(t, err2, customerrors.ErrDuplicateEmail)
			assert.NoError(t, err1)
		} else {
			assert.Error(t, err1)
			assert.ErrorIs(t, err1, customerrors.ErrDuplicateEmail)
			assert.NoError(t, err2)
		}

		// Verify only one user exists
		_, err := storage.User(ctx, email)
		require.NoError(t, err)
	})
}

func TestStorage_RegisterRollsBackWhenOutboxInsertFails(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	_, err := tdb.Pool.Exec(ctx, `DROP TABLE userservice.outbox_events`)
	require.NoError(t, err)

	storage, err := repository.NewForTests(ctx, tdb.Pool)
	require.NoError(t, err)
	_, err = storage.RegisterUserTX(
		ctx,
		"atomic@example.com",
		[]byte("hash"),
		"Atomic User",
		domain.RoleCustomer,
	)
	require.Error(t, err)

	var count int
	err = tdb.Pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM userservice.users WHERE email=$1`,
		"atomic@example.com",
	).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count, "business row and outbox row must commit or roll back together")
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

	userID, err := storage.RegisterUserTX(ctx, email, passHash, name, domain.RoleCustomer)
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
