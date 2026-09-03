package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/events"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

type Storage struct {
	pool   *pgxpool.Pool
	outbox *outbox.PostgresStore
	topic  string
}

func New(ctx context.Context, databaseURL, orderCreatedTopic string) (*Storage, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Order PostgreSQL config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create Order PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping Order PostgreSQL: %w", err)
	}
	storage, err := NewForTests(pool, orderCreatedTopic)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return storage, nil
}

func NewForTests(pool *pgxpool.Pool, orderCreatedTopic string) (*Storage, error) {
	if pool == nil {
		return nil, fmt.Errorf("Order PostgreSQL pool is required")
	}
	if strings.TrimSpace(orderCreatedTopic) == "" {
		return nil, fmt.Errorf("order.created topic is required")
	}
	outboxStore, err := outbox.NewPostgresStore(pool, "orderservice")
	if err != nil {
		return nil, err
	}
	return &Storage{pool: pool, outbox: outboxStore, topic: strings.TrimSpace(orderCreatedTopic)}, nil
}

func (s *Storage) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Storage) OutboxStore() outbox.Store {
	return s.outbox
}

func (s *Storage) CreatePending(
	ctx context.Context,
	input domain.NewOrder,
	owner uuid.UUID,
) (*domain.Order, bool, error) {
	var result *domain.Order
	created := false
	err := inTransaction(ctx, s.pool, func(tx pgx.Tx) error {
		order, err := insertPendingOrder(ctx, tx, input, owner)
		if errors.Is(err, pgx.ErrNoRows) {
			order, err = getOrderByIdempotency(ctx, tx, input.UserID, input.IdempotencyKey)
			if err != nil {
				return err
			}
			items, err := getOrderItems(ctx, tx, order.ID)
			if err != nil {
				return err
			}
			order.Items = items
			result = order
			return nil
		}
		if err != nil {
			return err
		}

		for _, item := range input.Items {
			_, err := tx.Exec(ctx, `INSERT INTO orderservice.order_items (
				order_id, product_id, product_name, unit_price, quantity, line_total
			) VALUES ($1, $2, $3, $4, $5, $6)`,
				order.ID,
				item.ProductID,
				item.ProductName,
				item.UnitPrice,
				item.Quantity,
				item.LineTotal,
			)
			if err != nil {
				return fmt.Errorf("insert order item: %w", err)
			}
		}
		order.Items = append([]domain.Item(nil), input.Items...)
		result = order
		created = true
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("create pending order: %w", err)
	}
	return result, created, nil
}

func insertPendingOrder(
	ctx context.Context,
	tx pgx.Tx,
	input domain.NewOrder,
	owner uuid.UUID,
) (*domain.Order, error) {
	row := tx.QueryRow(ctx, `INSERT INTO orderservice.orders (
		user_id, status, total_amount, currency, idempotency_key,
		cart_revision, processing_by, processing_at
	) VALUES ($1, 'pending', $2, $3, $4, $5, $6, NOW())
	ON CONFLICT (user_id, idempotency_key) DO NOTHING
	RETURNING id, user_id, status, total_amount, currency, idempotency_key,
		cart_revision, failure_reason, processing_by, processing_at, created_at, updated_at`,
		input.UserID,
		input.TotalAmount,
		input.Currency,
		input.IdempotencyKey,
		input.CartRevision,
		owner,
	)
	return scanOrder(row)
}

func (s *Storage) GetByIdempotency(
	ctx context.Context,
	userID int64,
	idempotencyKey string,
) (*domain.Order, error) {
	order, err := getOrderByIdempotency(ctx, s.pool, userID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	order.Items, err = getOrderItems(ctx, s.pool, order.ID)
	return order, err
}

func (s *Storage) GetByIDForUser(ctx context.Context, orderID, userID int64) (*domain.Order, error) {
	order, err := getOrder(ctx, s.pool, orderID, userID)
	if err != nil {
		return nil, err
	}
	order.Items, err = getOrderItems(ctx, s.pool, order.ID)
	return order, err
}

func (s *Storage) GetByID(ctx context.Context, orderID int64) (*domain.Order, error) {
	order, err := getOrder(ctx, s.pool, orderID, 0)
	if err != nil {
		return nil, err
	}
	order.Items, err = getOrderItems(ctx, s.pool, order.ID)
	return order, err
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func getOrderByIdempotency(
	ctx context.Context,
	db queryRower,
	userID int64,
	idempotencyKey string,
) (*domain.Order, error) {
	row := db.QueryRow(ctx, `SELECT id, user_id, status, total_amount, currency,
		idempotency_key, cart_revision, failure_reason, processing_by,
		processing_at, created_at, updated_at
		FROM orderservice.orders
		WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey)
	return scanOrderNotFound(row)
}

func getOrder(ctx context.Context, db queryRower, orderID, userID int64) (*domain.Order, error) {
	statement := `SELECT id, user_id, status, total_amount, currency,
		idempotency_key, cart_revision, failure_reason, processing_by,
		processing_at, created_at, updated_at
		FROM orderservice.orders WHERE id=$1`
	args := []any{orderID}
	if userID > 0 {
		statement += ` AND user_id=$2`
		args = append(args, userID)
	}
	return scanOrderNotFound(db.QueryRow(ctx, statement, args...))
}

func scanOrderNotFound(row pgx.Row) (*domain.Order, error) {
	order, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan order: %w", err)
	}
	return order, nil
}

func scanOrder(row pgx.Row) (*domain.Order, error) {
	var order domain.Order
	var processingBy uuid.NullUUID
	err := row.Scan(
		&order.ID,
		&order.UserID,
		&order.Status,
		&order.TotalAmount,
		&order.Currency,
		&order.IdempotencyKey,
		&order.CartRevision,
		&order.FailureReason,
		&processingBy,
		&order.ProcessingAt,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if processingBy.Valid {
		order.ProcessingBy = processingBy.UUID.String()
	}
	return &order, err
}

func getOrderItems(ctx context.Context, db queryer, orderID int64) ([]domain.Item, error) {
	rows, err := db.Query(ctx, `SELECT product_id, product_name, unit_price, quantity, line_total
		FROM orderservice.order_items WHERE order_id=$1 ORDER BY product_id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("query order items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Item, 0)
	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(
			&item.ProductID,
			&item.ProductName,
			&item.UnitPrice,
			&item.Quantity,
			&item.LineTotal,
		); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}
	return items, nil
}

func (s *Storage) ListForUser(
	ctx context.Context,
	userID int64,
	limit, offset int,
) ([]*domain.Order, int64, error) {
	var total int64
	if err := s.pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM orderservice.orders WHERE user_id=$1`,
		userID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count user orders: %w", err)
	}

	rows, err := s.pool.Query(ctx, `SELECT id, user_id, status, total_amount, currency,
		idempotency_key, cart_revision, failure_reason, processing_by,
		processing_at, created_at, updated_at
		FROM orderservice.orders
		WHERE user_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list user orders: %w", err)
	}
	orders := make([]*domain.Order, 0, limit)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			rows.Close()
			return nil, 0, fmt.Errorf("scan listed order: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, fmt.Errorf("iterate user orders: %w", err)
	}
	rows.Close()

	// Release the result-set connection before loading item collections. This
	// keeps ListForUser safe even when the pool is deliberately configured with
	// a single connection.
	for _, order := range orders {
		items, err := getOrderItems(ctx, s.pool, order.ID)
		if err != nil {
			return nil, 0, err
		}
		order.Items = items
	}
	return orders, total, nil
}

func (s *Storage) TryClaimPending(
	ctx context.Context,
	orderID int64,
	owner uuid.UUID,
	staleAfter time.Duration,
) (bool, error) {
	result, err := s.pool.Exec(ctx, `UPDATE orderservice.orders
		SET processing_by=$2, processing_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND status='pending'
		AND (
			processing_by IS NULL OR processing_by=$2 OR
			processing_at < NOW() - ($3 * INTERVAL '1 millisecond')
		)`, orderID, owner, staleAfter.Milliseconds())
	if err != nil {
		return false, fmt.Errorf("claim pending order: %w", err)
	}
	return result.RowsAffected() == 1, nil
}

func (s *Storage) RefreshPendingLease(ctx context.Context, orderID int64, owner uuid.UUID) error {
	result, err := s.pool.Exec(ctx, `UPDATE orderservice.orders
		SET processing_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND status='pending' AND processing_by=$2`, orderID, owner)
	if err != nil {
		return fmt.Errorf("refresh pending order lease: %w", err)
	}
	if result.RowsAffected() != 1 {
		return domain.ErrWorkflowLeaseLost
	}
	return nil
}

func (s *Storage) ReleasePendingLease(
	ctx context.Context,
	orderID int64,
	owner uuid.UUID,
	reason string,
) error {
	result, err := s.pool.Exec(ctx, `UPDATE orderservice.orders
		SET processing_by=NULL, processing_at=NULL, failure_reason=$3, updated_at=NOW()
		WHERE id=$1 AND status='pending' AND processing_by=$2`, orderID, owner, reason)
	if err != nil {
		return fmt.Errorf("release pending order lease: %w", err)
	}
	if result.RowsAffected() != 1 {
		return domain.ErrWorkflowLeaseLost
	}
	return nil
}

func (s *Storage) MarkFailed(
	ctx context.Context,
	orderID int64,
	owner uuid.UUID,
	reason string,
) error {
	result, err := s.pool.Exec(ctx, `UPDATE orderservice.orders
		SET status='failed', processing_by=NULL, processing_at=NULL,
			failure_reason=$3, updated_at=NOW()
		WHERE id=$1 AND status='pending' AND processing_by=$2`, orderID, owner, reason)
	if err != nil {
		return fmt.Errorf("mark order failed: %w", err)
	}
	if result.RowsAffected() != 1 {
		return domain.ErrInvalidTransition
	}
	return nil
}

func (s *Storage) ConfirmWithOutbox(
	ctx context.Context,
	order *domain.Order,
	owner uuid.UUID,
) (bool, error) {
	transitioned := false
	err := inTransaction(ctx, s.pool, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `UPDATE orderservice.orders
			SET status='confirmed', processing_by=NULL, processing_at=NULL,
				failure_reason='', updated_at=NOW()
			WHERE id=$1 AND status='pending' AND processing_by=$2`, order.ID, owner)
		if err != nil {
			return fmt.Errorf("confirm order: %w", err)
		}
		if result.RowsAffected() != 1 {
			var status domain.Status
			if err := tx.QueryRow(ctx, `SELECT status FROM orderservice.orders WHERE id=$1`, order.ID).Scan(&status); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrOrderNotFound
				}
				return err
			}
			if status == domain.StatusConfirmed {
				return nil
			}
			return domain.ErrInvalidTransition
		}

		event, err := events.NewOrderCreated(s.topic, order, time.Now().UTC())
		if err != nil {
			return err
		}
		if err := s.outbox.Insert(ctx, tx, event); err != nil {
			return err
		}
		transitioned = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("confirm order with outbox: %w", err)
	}
	return transitioned, nil
}

func (s *Storage) ListRecoverablePending(
	ctx context.Context,
	olderThan, staleBefore time.Time,
	limit int,
) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM orderservice.orders
		WHERE status='pending' AND updated_at <= $1
		AND (processing_by IS NULL OR processing_at < $2)
		ORDER BY updated_at, id
		LIMIT $3`, olderThan, staleBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("list recoverable pending orders: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan recoverable order id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
