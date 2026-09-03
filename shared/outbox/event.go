package outbox

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Event is the durable representation stored in a service-owned outbox table.
// Payload contains only the typed domain payload; Envelope is built by the relay.
type Event struct {
	EventID       uuid.UUID
	Topic         string
	EventType     string
	AggregateType string
	AggregateID   string
	Version       int
	Payload       json.RawMessage
	OccurredAt    time.Time
}

type Envelope struct {
	EventID     string          `json:"event_id"`
	EventType   string          `json:"event_type"`
	Version     int             `json:"version"`
	OccurredAt  time.Time       `json:"occurred_at"`
	AggregateID string          `json:"aggregate_id"`
	Payload     json.RawMessage `json:"payload"`
}

func NewEvent(
	topic string,
	eventType string,
	aggregateType string,
	aggregateID string,
	version int,
	payload any,
	occurredAt time.Time,
) (Event, error) {
	if strings.TrimSpace(topic) == "" {
		return Event{}, fmt.Errorf("outbox topic is required")
	}
	if strings.TrimSpace(eventType) == "" {
		return Event{}, fmt.Errorf("outbox event type is required")
	}
	if strings.TrimSpace(aggregateType) == "" {
		return Event{}, fmt.Errorf("outbox aggregate type is required")
	}
	if strings.TrimSpace(aggregateID) == "" {
		return Event{}, fmt.Errorf("outbox aggregate id is required")
	}
	if version <= 0 {
		return Event{}, fmt.Errorf("outbox event version must be positive")
	}
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal outbox payload: %w", err)
	}

	return Event{
		EventID:       uuid.New(),
		Topic:         strings.TrimSpace(topic),
		EventType:     strings.TrimSpace(eventType),
		AggregateType: strings.TrimSpace(aggregateType),
		AggregateID:   strings.TrimSpace(aggregateID),
		Version:       version,
		Payload:       encodedPayload,
		OccurredAt:    occurredAt.UTC(),
	}, nil
}

func (e Event) Message() ([]byte, error) {
	if e.EventID == uuid.Nil {
		return nil, fmt.Errorf("outbox event id is required")
	}
	envelope := Envelope{
		EventID:     e.EventID.String(),
		EventType:   e.EventType,
		Version:     e.Version,
		OccurredAt:  e.OccurredAt.UTC(),
		AggregateID: e.AggregateID,
		Payload:     e.Payload,
	}
	message, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal outbox envelope: %w", err)
	}
	return message, nil
}
