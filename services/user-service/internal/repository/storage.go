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
	"github.com/zxCroshka/ecommerce/services/user-service/internal/events"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/db"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/users"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

type Storage struct {
	pool   *pgxpool.Pool
	users  *users.Storage
	outbox *outbox.PostgresStore
	topic  string
}

func NewForTests(ctx context.Context, pool *pgxpool.Pool) (*Storage, error) {
	outboxStore, err := outbox.NewPostgresStore(pool, "userservice")
	if err != nil {
		return nil, err
	}
	return &Storage{
		pool:   pool,
		users:  users.New(pool),
		outbox: outboxStore,
		topic:  events.UserRegisteredType,
	}, nil
}

func New(ctx context.Context, storageURL, userRegisteredTopic string) (*Storage, error) {
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
		pool.Close()
		slog.Error("Failed to ping database", "error", err)
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	outboxStore, err := outbox.NewPostgresStore(pool, "userservice")
	if err != nil {
		pool.Close()
		return nil, err
	}
	slog.Info("Database connection established successfully")
	return &Storage{
		pool:   pool,
		users:  users.New(pool),
		outbox: outboxStore,
		topic:  userRegisteredTopic,
	}, nil
}

func (s *Storage) RegisterUserTX(
	ctx context.Context,
	email string,
	passHash []byte,
	name string,
	role domain.Role,
) (int64, error) {
	var userID int64
	err := db.Transaction(ctx, s.pool, func(tx pgx.Tx) error {
		Users := s.users.WithTX(tx)
		createdAt := time.Now().UTC()
		id, err := Users.SaveUser(ctx, email, passHash, name, role, createdAt)
		if err != nil {
			return err
		}
		userID = id
		event, err := events.NewUserRegistered(s.topic, userID, email, name, role, createdAt)
		if err != nil {
			return err
		}
		return s.outbox.Insert(ctx, tx, event)
	})
	if err != nil {
		if errors.Is(err, customerrors.ErrDuplicateEmail) {
			slog.Info("user already exists", "email", email)
		} else {
			slog.Error("failed to register user", "email", email, "error", err)
		}
		return 0, err
	}
	slog.Info("user registered successfully", "user_id", userID, "email", email)
	return userID, err
}

func (s *Storage) OutboxStore() outbox.Store {
	return s.outbox
}

func (s *Storage) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Storage) User(ctx context.Context, email string) (domain.User, error) {
	return s.users.User(ctx, email)
}

func (s *Storage) UserByID(ctx context.Context, userID int64) (domain.User, error) {
	return s.users.UserByID(ctx, userID)
}

func (s *Storage) Role(ctx context.Context, userID int64) (domain.Role, error) {
	return s.users.Role(ctx, userID)
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
