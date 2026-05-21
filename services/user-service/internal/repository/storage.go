package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/db"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/users"
)

type Config struct {
	Postgres struct {
		Host     string
		Port     uint16
		User     string
		Password string
		Database string
		Sslmode  string
	}
}

func NewConfig(Postgres struct {
	Host     string
	Port     uint16
	User     string
	Password string
	Database string
	Sslmode  string
}) Config {
	return Config{
		Postgres: Postgres,
	}
}
func GetPostgresURL(cfg Config) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.Database,
		cfg.Postgres.Sslmode,
	)
}

type Storage struct {
	pool  *pgxpool.Pool
	users *users.Storage
}

func NewForTests(ctx context.Context, pool *pgxpool.Pool) (*Storage, error) {
	return &Storage{
		pool:  pool,
		users: users.New(pool),
	}, nil
}

func New(ctx context.Context, storageURL string) (*Storage, error) {
	cfg, err := pgxpool.ParseConfig(storageURL)
	if err != nil {
		slog.Error(fmt.Sprintf("error parsing connection config: %v", err))
		return nil, err

	}
	slog.Info("Database config",
		"host", cfg.ConnConfig.Host,
		"port", cfg.ConnConfig.Port,
		"user", cfg.ConnConfig.User,
		"database", cfg.ConnConfig.Database)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		slog.Error(fmt.Sprintf("error creating connection pool: %v", err))
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		slog.Error("Failed to ping database", "error", err)
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	slog.Info("Database connection established successfully")
	return &Storage{
		pool:  pool,
		users: users.New(pool),
	}, nil
}

func (s *Storage) RegisterUserTX(
	ctx context.Context,
	email string,
	passHash []byte,
	name string,
	isAdmin bool,
) (int64, error) {
	var userID int64
	err := db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		Users := s.users.WithTX(tx)
		createdAt := time.Now()
		id, err := Users.SaveUser(ctx, email, passHash, name, isAdmin, createdAt)
		if err != nil {
			return err
		}
		userID = id
		return nil
	})
	if err != nil {
		if errors.Is(err, customerrors.ErrUserExists) {
			slog.Info("user already exists", "email", email)
		} else {
			slog.Error("failed to register user", "email", email, "error", err)
		}
		return 0, err
	}
	slog.Info("user registered successfully", "user_id", userID, "email", email)
	return userID, err
}

func (s *Storage) User(ctx context.Context, email string) (domain.User, error) {
	return s.users.User(ctx, email)
}

func (s *Storage) UserByID(ctx context.Context, userID int64) (domain.User, error) {
	return s.users.UserByID(ctx, userID)
}

func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	return s.users.IsAdmin(ctx, userID)
}

func (s *Storage) UpdateName(ctx context.Context, userID int64, newName string) error {
	return s.users.UpdateName(ctx, userID, newName)
}

func (s *Storage) UpdateEmail(ctx context.Context, userID int64, newEmail string) error {
	return s.users.UpdateEmail(ctx, userID, newEmail)
}

func (s *Storage) UpdatePassword(ctx context.Context, userId int64, newPassHash []byte) error {
	return s.users.UpdatePassword(ctx, userId, newPassHash)
}
