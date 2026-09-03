package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

type RelayConfig struct {
	PollInterval   time.Duration
	PublishTimeout time.Duration
	StoreTimeout   time.Duration
	LockTimeout    time.Duration
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	BatchSize      int
}

func (c RelayConfig) validate() error {
	if c.PollInterval <= 0 || c.PublishTimeout <= 0 || c.StoreTimeout <= 0 ||
		c.LockTimeout <= 0 || c.RetryBaseDelay <= 0 || c.RetryMaxDelay <= 0 {
		return fmt.Errorf("outbox relay durations must be positive")
	}
	if c.RetryBaseDelay > c.RetryMaxDelay {
		return fmt.Errorf("outbox retry base delay cannot exceed max delay")
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("outbox relay batch size must be positive")
	}
	return nil
}

type Relay struct {
	log       *slog.Logger
	store     Store
	publisher Publisher
	config    RelayConfig
	workerID  uuid.UUID

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

func NewRelay(log *slog.Logger, store Store, publisher Publisher, config RelayConfig) (*Relay, error) {
	if store == nil {
		return nil, fmt.Errorf("outbox store is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("outbox publisher is required")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &Relay{
		log:       log,
		store:     store,
		publisher: publisher,
		config:    config,
		workerID:  uuid.New(),
	}, nil
}

func (r *Relay) Start(parent context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return fmt.Errorf("outbox relay is already running")
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.done = make(chan struct{})
	r.running = true
	go r.run(ctx)
	return nil
}

func (r *Relay) Stop(ctx context.Context) error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	cancel := r.cancel
	done := r.done
	r.mu.Unlock()

	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop outbox relay: %w", ctx.Err())
	}
}

func (r *Relay) run(ctx context.Context) {
	defer func() {
		r.mu.Lock()
		r.running = false
		close(r.done)
		r.mu.Unlock()
	}()

	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	for {
		if err := r.processBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.log.Error("outbox relay iteration failed", "error", err, "worker_id", r.workerID)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Relay) processBatch(ctx context.Context) error {
	for range r.config.BatchSize {
		// Claim immediately before publishing. A large batch claimed up front could
		// leave its tail leased longer than LockTimeout while earlier Kafka calls
		// are in flight, allowing another instance to reclaim it unnecessarily.
		storeCtx, cancel := context.WithTimeout(ctx, r.config.StoreTimeout)
		records, err := r.store.Claim(storeCtx, r.workerID, 1, r.config.LockTimeout)
		cancel()
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return nil
		}
		if len(records) != 1 {
			return fmt.Errorf("outbox store returned %d records for a single-record claim", len(records))
		}

		record := records[0]
		if err := r.publishRecord(ctx, record); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			r.log.Warn(
				"outbox event delivery failed and remains retryable",
				"error", err,
				"event_id", record.EventID,
				"event_type", record.EventType,
				"attempt", record.Attempts,
				"worker_id", r.workerID,
			)
		}
	}
	return nil
}

func (r *Relay) publishRecord(ctx context.Context, record Record) error {
	message, err := record.Event.Message()
	if err == nil {
		publishCtx, cancel := context.WithTimeout(ctx, r.config.PublishTimeout)
		err = r.publisher.Publish(
			publishCtx,
			record.Topic,
			[]byte(record.AggregateID),
			message,
		)
		cancel()
	}

	storeParent := context.WithoutCancel(ctx)
	storeCtx, cancel := context.WithTimeout(storeParent, r.config.StoreTimeout)
	defer cancel()
	if err == nil {
		if markErr := r.store.MarkPublished(storeCtx, record.EventID, r.workerID, time.Now().UTC()); markErr != nil {
			// Kafka may already contain the event. Leaving the row retryable is what
			// makes the delivery contract explicitly at-least-once.
			return markErr
		}
		return nil
	}

	nextAttempt := time.Now().UTC().Add(r.retryDelay(record.Attempts))
	if markErr := r.store.MarkFailed(
		storeCtx,
		record.EventID,
		r.workerID,
		err.Error(),
		nextAttempt,
	); markErr != nil {
		return errors.Join(err, markErr)
	}
	return err
}

func (r *Relay) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := r.config.RetryBaseDelay
	for range attempt - 1 {
		if delay >= r.config.RetryMaxDelay/2 {
			return r.config.RetryMaxDelay
		}
		delay *= 2
	}
	if delay > r.config.RetryMaxDelay {
		return r.config.RetryMaxDelay
	}
	return delay
}

func AggregateInt64(id int64) string {
	return strconv.FormatInt(id, 10)
}
