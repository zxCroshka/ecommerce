package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type relayStore struct {
	mu               sync.Mutex
	records          []Record
	published        []uuid.UUID
	failed           []uuid.UUID
	claimLimits      []int
	markPublishedErr error
}

func (s *relayStore) Claim(_ context.Context, _ uuid.UUID, limit int, _ time.Duration) ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimLimits = append(s.claimLimits, limit)
	if len(s.records) == 0 {
		return nil, nil
	}
	count := min(limit, len(s.records))
	result := append([]Record(nil), s.records[:count]...)
	s.records = append([]Record(nil), s.records[count:]...)
	return result, nil
}

func (s *relayStore) MarkPublished(_ context.Context, eventID, _ uuid.UUID, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.markPublishedErr != nil {
		return s.markPublishedErr
	}
	s.published = append(s.published, eventID)
	return nil
}

func (s *relayStore) MarkFailed(_ context.Context, eventID, _ uuid.UUID, _ string, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, eventID)
	return nil
}

type relayPublisher struct {
	mu      sync.Mutex
	err     error
	values  [][]byte
	started chan struct{}
	block   chan struct{}
}

func (p *relayPublisher) Publish(ctx context.Context, _ string, _, value []byte) error {
	if p.started != nil {
		select {
		case p.started <- struct{}{}:
		default:
		}
	}
	if p.block != nil {
		select {
		case <-p.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	p.mu.Lock()
	p.values = append(p.values, append([]byte(nil), value...))
	p.mu.Unlock()
	return p.err
}

func testRelay(t *testing.T, store Store, publisher Publisher) *Relay {
	t.Helper()
	relay, err := NewRelay(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		store,
		publisher,
		RelayConfig{
			PollInterval:   5 * time.Millisecond,
			PublishTimeout: 50 * time.Millisecond,
			StoreTimeout:   50 * time.Millisecond,
			LockTimeout:    time.Second,
			RetryBaseDelay: time.Millisecond,
			RetryMaxDelay:  time.Second,
			BatchSize:      10,
		},
	)
	require.NoError(t, err)
	return relay
}

func testRecord(t *testing.T) Record {
	t.Helper()
	event, err := NewEvent(
		"user.registered", "user.registered", "user", "42", 1,
		struct {
			UserID int64 `json:"user_id"`
		}{UserID: 42},
		time.Now(),
	)
	require.NoError(t, err)
	return Record{Event: event, Attempts: 1}
}

func TestRelayPublishesAndMarksSuccess(t *testing.T) {
	record := testRecord(t)
	store := &relayStore{records: []Record{record}}
	publisher := &relayPublisher{}
	relay := testRelay(t, store, publisher)

	require.NoError(t, relay.processBatch(context.Background()))
	require.Equal(t, []uuid.UUID{record.EventID}, store.published)
	require.Empty(t, store.failed)
	require.Len(t, publisher.values, 1)
	require.Equal(t, []int{1, 1}, store.claimLimits)
}

func TestRelayKafkaFailureRemainsRetryable(t *testing.T) {
	record := testRecord(t)
	store := &relayStore{records: []Record{record}}
	publisher := &relayPublisher{err: errors.New("kafka unavailable")}
	relay := testRelay(t, store, publisher)

	require.NoError(t, relay.processBatch(context.Background()))
	require.Empty(t, store.published)
	require.Equal(t, []uuid.UUID{record.EventID}, store.failed)

	publisher.err = nil
	store.records = []Record{{Event: record.Event, Attempts: 2}}
	require.NoError(t, relay.processBatch(context.Background()))
	require.Equal(t, []uuid.UUID{record.EventID}, store.published)
	require.Len(t, publisher.values, 2)
}

func TestRelayDuplicateAfterPublishBeforeMark(t *testing.T) {
	record := testRecord(t)
	store := &relayStore{
		records:          []Record{record},
		markPublishedErr: errors.New("database unavailable after publish"),
	}
	publisher := &relayPublisher{}
	relay := testRelay(t, store, publisher)

	require.NoError(t, relay.processBatch(context.Background()))
	require.Len(t, publisher.values, 1)

	store.markPublishedErr = nil
	store.records = []Record{{Event: record.Event, Attempts: 2}}
	require.NoError(t, relay.processBatch(context.Background()))
	require.Len(t, publisher.values, 2)
	require.Equal(t, publisher.values[0], publisher.values[1])
	require.Equal(t, []uuid.UUID{record.EventID}, store.published)
}

func TestRelayGracefulShutdownWaitsForOwnedWorker(t *testing.T) {
	record := testRecord(t)
	store := &relayStore{records: []Record{record}}
	publisher := &relayPublisher{started: make(chan struct{}, 1), block: make(chan struct{})}
	relay := testRelay(t, store, publisher)

	require.NoError(t, relay.Start(context.Background()))
	select {
	case <-publisher.started:
	case <-time.After(time.Second):
		t.Fatal("relay did not start publishing")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, relay.Stop(shutdownCtx))
}
