package outbox

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEventMessageUsesStableEnvelope(t *testing.T) {
	type payload struct {
		UserID int64  `json:"user_id"`
		Email  string `json:"email"`
	}
	occurredAt := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	event, err := NewEvent(
		"user.registered",
		"user.registered",
		"user",
		"42",
		1,
		payload{UserID: 42, Email: "user@example.com"},
		occurredAt,
	)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, event.EventID)

	message, err := event.Message()
	require.NoError(t, err)

	var envelope Envelope
	require.NoError(t, json.Unmarshal(message, &envelope))
	require.Equal(t, event.EventID.String(), envelope.EventID)
	require.Equal(t, "user.registered", envelope.EventType)
	require.Equal(t, 1, envelope.Version)
	require.Equal(t, "42", envelope.AggregateID)
	require.Equal(t, occurredAt, envelope.OccurredAt)

	var decodedPayload payload
	require.NoError(t, json.Unmarshal(envelope.Payload, &decodedPayload))
	require.Equal(t, payload{UserID: 42, Email: "user@example.com"}, decodedPayload)
}
