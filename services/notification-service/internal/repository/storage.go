package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/ecommerce/services/notification-service/internal/domain"
)

type Storage struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Storage, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Notification PostgreSQL config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create Notification PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping Notification PostgreSQL: %w", err)
	}
	return NewForTests(pool), nil
}

func NewForTests(pool *pgxpool.Pool) *Storage {
	return &Storage{pool: pool}
}

func (s *Storage) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Storage) Save(ctx context.Context, input domain.NewNotification) (bool, error) {
	if input.EventID == uuid.Nil || input.UserID <= 0 || strings.TrimSpace(input.Type) == "" ||
		strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Body) == "" {
		return false, domain.ErrInvalidNotification
	}
	result, err := s.pool.Exec(ctx, `INSERT INTO notificationservice.notifications (
		event_id, user_id, type, title, body
	) VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (event_id) DO NOTHING`,
		input.EventID,
		input.UserID,
		strings.TrimSpace(input.Type),
		strings.TrimSpace(input.Title),
		strings.TrimSpace(input.Body),
	)
	if err != nil {
		return false, fmt.Errorf("save notification: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *Storage) ListForUser(
	ctx context.Context,
	userID int64,
	limit, offset int,
) ([]*domain.Notification, int64, error) {
	var total int64
	if err := s.pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM notificationservice.notifications WHERE user_id=$1`,
		userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT id, event_id, user_id, type, title, body, created_at, read_at
		FROM notificationservice.notifications
		WHERE user_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	notifications := make([]*domain.Notification, 0, limit)
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notifications: %w", err)
	}
	return notifications, total, nil
}

func (s *Storage) MarkAsRead(ctx context.Context, notificationID, userID int64) (*domain.Notification, error) {
	row := s.pool.QueryRow(ctx, `UPDATE notificationservice.notifications
		SET read_at=COALESCE(read_at, NOW())
		WHERE id=$1 AND user_id=$2
		RETURNING id, event_id, user_id, type, title, body, created_at, read_at`,
		notificationID,
		userID,
	)
	notification, err := scanNotification(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotificationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mark notification read: %w", err)
	}
	return notification, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanNotification(row rowScanner) (*domain.Notification, error) {
	var notification domain.Notification
	if err := row.Scan(
		&notification.ID,
		&notification.EventID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Body,
		&notification.CreatedAt,
		&notification.ReadAt,
	); err != nil {
		return nil, err
	}
	return &notification, nil
}
