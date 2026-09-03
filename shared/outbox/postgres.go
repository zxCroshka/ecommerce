package outbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Record struct {
	Event
	Attempts int
}

type Store interface {
	Claim(ctx context.Context, workerID uuid.UUID, limit int, lockTimeout time.Duration) ([]Record, error)
	MarkPublished(ctx context.Context, eventID, workerID uuid.UUID, publishedAt time.Time) error
	MarkFailed(ctx context.Context, eventID, workerID uuid.UUID, lastError string, nextAttemptAt time.Time) error
}

type PostgresStore struct {
	pool      *pgxpool.Pool
	tableName string
}

func NewPostgresStore(pool *pgxpool.Pool, schema string) (*PostgresStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("outbox postgres pool is required")
	}
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return nil, fmt.Errorf("outbox schema is required")
	}

	return &PostgresStore{
		pool:      pool,
		tableName: pgx.Identifier{schema, "outbox_events"}.Sanitize(),
	}, nil
}

func (s *PostgresStore) Insert(ctx context.Context, tx pgx.Tx, event Event) error {
	if tx == nil {
		return fmt.Errorf("outbox transaction is required")
	}
	statement := fmt.Sprintf(`INSERT INTO %s (
		event_id, topic, event_type, aggregate_type, aggregate_id,
		version, payload, occurred_at, created_at, next_attempt_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`, s.tableName)

	if _, err := tx.Exec(
		ctx,
		statement,
		event.EventID,
		event.Topic,
		event.EventType,
		event.AggregateType,
		event.AggregateID,
		event.Version,
		event.Payload,
		event.OccurredAt,
	); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (s *PostgresStore) Claim(
	ctx context.Context,
	workerID uuid.UUID,
	limit int,
	lockTimeout time.Duration,
) ([]Record, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("outbox claim limit must be positive")
	}
	if lockTimeout <= 0 {
		return nil, fmt.Errorf("outbox lock timeout must be positive")
	}

	statement := fmt.Sprintf(`WITH candidates AS (
		SELECT event_id
		FROM %s
		WHERE published_at IS NULL
		  AND next_attempt_at <= NOW()
		  AND (locked_at IS NULL OR locked_at < NOW() - ($3 * INTERVAL '1 millisecond'))
		ORDER BY created_at, event_id
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	)
	UPDATE %s AS events
	SET locked_by = $2,
		locked_at = NOW(),
		attempts = events.attempts + 1
	FROM candidates
	WHERE events.event_id = candidates.event_id
	RETURNING events.event_id, events.topic, events.event_type,
		events.aggregate_type, events.aggregate_id, events.version,
		events.payload, events.occurred_at, events.attempts`, s.tableName, s.tableName)

	rows, err := s.pool.Query(ctx, statement, limit, workerID, lockTimeout.Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer rows.Close()

	records := make([]Record, 0, limit)
	for rows.Next() {
		var record Record
		if err := rows.Scan(
			&record.EventID,
			&record.Topic,
			&record.EventType,
			&record.AggregateType,
			&record.AggregateID,
			&record.Version,
			&record.Payload,
			&record.OccurredAt,
			&record.Attempts,
		); err != nil {
			return nil, fmt.Errorf("scan claimed outbox event: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed outbox events: %w", err)
	}
	return records, nil
}

func (s *PostgresStore) MarkPublished(
	ctx context.Context,
	eventID, workerID uuid.UUID,
	publishedAt time.Time,
) error {
	statement := fmt.Sprintf(`UPDATE %s
		SET published_at=$3, locked_by=NULL, locked_at=NULL, last_error=NULL
		WHERE event_id=$1 AND locked_by=$2 AND published_at IS NULL`, s.tableName)
	tag, err := s.pool.Exec(ctx, statement, eventID, workerID, publishedAt.UTC())
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("mark outbox event published: lease is no longer owned")
	}
	return nil
}

func (s *PostgresStore) MarkFailed(
	ctx context.Context,
	eventID, workerID uuid.UUID,
	lastError string,
	nextAttemptAt time.Time,
) error {
	statement := fmt.Sprintf(`UPDATE %s
		SET last_error=$3, next_attempt_at=$4, locked_by=NULL, locked_at=NULL
		WHERE event_id=$1 AND locked_by=$2 AND published_at IS NULL`, s.tableName)
	tag, err := s.pool.Exec(ctx, statement, eventID, workerID, lastError, nextAttemptAt.UTC())
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("mark outbox event failed: lease is no longer owned")
	}
	return nil
}
