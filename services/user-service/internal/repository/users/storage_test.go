package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/testhelper"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/users"
)

func TestStorage_SaveUser(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage := users.New(tdb.Pool)

	t.Run("successfully create user", func(t *testing.T) {
		email := "test@example.com"
		passHash := []byte("hashed_password")
		name := "Test User"
		role := domain.RoleCustomer
		createdAt := time.Now()

		id, err := storage.SaveUser(ctx, email, passHash, name, role, createdAt)

		require.NoError(t, err)
		// ✅ Исправлено: используем EqualValues
		assert.EqualValues(t, 1, id)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, role, user.Role)
		assert.Equal(t, passHash, user.PassHash)
	})

	t.Run("duplicate email returns error", func(t *testing.T) {
		email := "duplicate@example.com"
		passHash := []byte("hash1")

		_, err := storage.SaveUser(ctx, email, passHash, "User1", domain.RoleCustomer, time.Now())
		require.NoError(t, err)

		_, err = storage.SaveUser(ctx, email, []byte("hash2"), "User2", domain.RoleCustomer, time.Now())

		require.Error(t, err)
		assert.ErrorIs(t, err, customerrors.ErrDuplicateEmail)
	})

	t.Run("create admin user", func(t *testing.T) {
		email := "admin@example.com"

		id, err := storage.SaveUser(ctx, email, []byte("hash"), "Admin", domain.RoleAdmin, time.Now())

		require.NoError(t, err)
		assert.Greater(t, id, int64(0))

		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, user.Role)
	})
}

func TestStorage_User(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage := users.New(tdb.Pool)

	t.Run("get existing user", func(t *testing.T) {
		email := "find@example.com"
		passHash := []byte("hash")
		name := "Find User"
		role := domain.RoleCustomer
		createdAt := time.Now()

		_, err := storage.SaveUser(ctx, email, passHash, name, role, createdAt)
		require.NoError(t, err)

		user, err := storage.User(ctx, email)

		require.NoError(t, err)
		assert.Equal(t, email, user.Email)
		assert.Equal(t, name, user.Name)
		assert.Equal(t, role, user.Role)
		assert.Equal(t, passHash, user.PassHash)
		assert.NotZero(t, user.CreatedAt)
	})

	t.Run("get non-existent user", func(t *testing.T) {
		_, err := storage.User(ctx, "nonexistent@example.com")

		require.Error(t, err)
		assert.ErrorIs(t, err, customerrors.ErrUserNotFound)
	})
}

func TestStorage_Role(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage := users.New(tdb.Pool)

	t.Run("regular user is not admin", func(t *testing.T) {
		email := "regular@example.com"
		_, err := storage.SaveUser(ctx, email, []byte("hash"), "Regular", domain.RoleCustomer, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)

		role, err := storage.Role(ctx, user.Id)

		require.NoError(t, err)
		assert.Equal(t, domain.RoleCustomer, role)
	})

	t.Run("admin user is admin", func(t *testing.T) {
		email := "admin@example.com"
		_, err := storage.SaveUser(ctx, email, []byte("hash"), "Admin", domain.RoleAdmin, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)

		role, err := storage.Role(ctx, user.Id)

		require.NoError(t, err)
		assert.Equal(t, domain.RoleAdmin, role)
	})
}

func TestStorage_UpdateName(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage := users.New(tdb.Pool)

	t.Run("update user name", func(t *testing.T) {
		email := "updatename@example.com"
		_, err := storage.SaveUser(ctx, email, []byte("hash"), "Old Name", domain.RoleCustomer, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)

		err = storage.UpdateName(ctx, user.Id, "New Name")

		require.NoError(t, err)

		updatedUser, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, "New Name", updatedUser.Name)
	})
}

func TestStorage_UpdateEmail(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage := users.New(tdb.Pool)

	t.Run("update user email", func(t *testing.T) {
		oldEmail := "old@example.com"
		newEmail := "new@example.com"
		_, err := storage.SaveUser(ctx, oldEmail, []byte("hash"), "User", domain.RoleCustomer, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, oldEmail)
		require.NoError(t, err)

		err = storage.UpdateEmail(ctx, user.Id, newEmail)

		require.NoError(t, err)

		_, err = storage.User(ctx, oldEmail)
		assert.ErrorIs(t, err, customerrors.ErrUserNotFound)

		updatedUser, err := storage.User(ctx, newEmail)
		require.NoError(t, err)
		assert.Equal(t, newEmail, updatedUser.Email)
	})
}

func TestStorage_UpdatePassword(t *testing.T) {
	tdb := testhelper.SetupTestPostgres(t)
	ctx := context.Background()
	defer func() { _ = tdb.TruncateTables(ctx) }()

	storage := users.New(tdb.Pool)

	t.Run("update user password", func(t *testing.T) {
		email := "updatepass@example.com"
		oldHash := []byte("old_hash")
		_, err := storage.SaveUser(ctx, email, oldHash, "User", domain.RoleCustomer, time.Now())
		require.NoError(t, err)

		user, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, oldHash, user.PassHash)

		newHash := []byte("new_hash")
		err = storage.UpdatePassword(ctx, user.Id, newHash)

		require.NoError(t, err)

		updatedUser, err := storage.User(ctx, email)
		require.NoError(t, err)
		assert.Equal(t, newHash, updatedUser.PassHash)
	})
}
