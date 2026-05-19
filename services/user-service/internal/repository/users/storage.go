package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/db"
)

type Storage struct {
	db db.DBTX
}

func New(db db.DBTX) *Storage {
	return &Storage{db: db}
}

func (s *Storage) WithTX(tx pgx.Tx) *Storage {
	return &Storage{db: tx}
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte, name string, isAdmin bool, created_at time.Time) (int64,error) {
	const op = "storage.postgres.SaveUser"
	stmt := `INSERT INTO userservice.users(email,password_hash,name,is_admin,created_at)
			VALUES($1,$2,$3,$4,$5)
			RETURNING id`
	var userID int64
	err := s.db.QueryRow(ctx, stmt, email, passHash, name, isAdmin, created_at).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				return 0,fmt.Errorf("%s: %w", op, customerrors.ErrUserExists)
			}
		}
		return 0,fmt.Errorf("%s: %w", op, err)
	}
	return userID,nil
}

func (s *Storage) User(ctx context.Context, email string) (domain.User, error) {
	const op = "storage.postgres.User"
	stmt := `SELECT * FROM userservice.users WHERE email=$1`
	row := s.db.QueryRow(ctx, stmt, email)
	var user domain.User
	err := row.Scan(&user.Id, &user.Email, &user.PassHash, &user.Name, &user.IsAdmin, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%s: %w",op, customerrors.ErrUserNotFound)
		}
		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}
func (s *Storage) UserByID(ctx context.Context, userID int64) (domain.User, error) {
	const op = "storage.postgres.UserByID"
	stmt := `SELECT * FROM userservice.users WHERE id=$1`
	row := s.db.QueryRow(ctx, stmt, userID)
	var user domain.User
	err := row.Scan(&user.Id, &user.Email, &user.PassHash, &user.Name, &user.IsAdmin, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%s: %w",op, customerrors.ErrUserNotFound)
		}
		return domain.User{}, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}
func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "storage.postgres.IsAdmin"
	stmt := `SELECT is_admin FROM userservice.users WHERE id=$1`
	row := s.db.QueryRow(ctx, stmt, userID)
	var isAdmin bool
	if err := row.Scan(&isAdmin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("%s: %w", op, customerrors.ErrUserNotFound)
		}
		return false, fmt.Errorf("%s: %w", op, err)
	}
	return isAdmin, nil
}

func (s *Storage) UpdateName(ctx context.Context, userID int64, newName string) error {
	const op = "storage.postgres.UpdateName"
	stmt := `UPDATE userservice.users SET name=$1 WHERE id=$2`
	if _, err := s.db.Exec(ctx, stmt, newName, userID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *Storage) UpdateEmail(ctx context.Context, userID int64, newEmail string) error {
	const op = "storage.postgres.UpdateName"
	stmt := `UPDATE userservice.users SET email=$1 WHERE id=$2`
	if _, err := s.db.Exec(ctx, stmt, newEmail, userID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (s *Storage) UpdatePassword(ctx context.Context, userID int64, newPassHash []byte) error {
	const op = "storage.postgres.UpdateName"
	stmt := `UPDATE userservice.users SET password_hash=$1 WHERE id=$2`
	if _, err := s.db.Exec(ctx, stmt, newPassHash, userID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}
