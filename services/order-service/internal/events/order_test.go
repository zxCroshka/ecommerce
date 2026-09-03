package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

func TestOrderCreatedUsesTypedStableEnvelope(t *testing.T) {
	occurredAt := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	event, err := NewOrderCreated("order.created", &domain.Order{
		ID:          17,
		UserID:      42,
		TotalAmount: 600,
		Currency:    "USD",
		Items: []domain.Item{{
			ProductID: 1,
			UnitPrice: 125,
			Quantity:  2,
			LineTotal: 250,
		}},
	}, occurredAt)
	require.NoError(t, err)
	require.Equal(t, OrderCreatedType, event.EventType)
	require.Equal(t, OrderCreatedVersion, event.Version)
	require.Equal(t, "17", event.AggregateID)

	message, err := event.Message()
	require.NoError(t, err)
	var envelope outbox.Envelope
	require.NoError(t, json.Unmarshal(message, &envelope))
	require.Equal(t, OrderCreatedType, envelope.EventType)
	require.Equal(t, occurredAt, envelope.OccurredAt)
	require.NotEmpty(t, envelope.EventID)

	var payload OrderCreatedPayload
	require.NoError(t, json.Unmarshal(envelope.Payload, &payload))
	require.Equal(t, int64(17), payload.OrderID)
	require.Equal(t, int64(42), payload.UserID)
	require.Equal(t, int64(600), payload.TotalAmount)
	require.Equal(t, "USD", payload.Currency)
	require.Equal(t, []OrderItemSummary{{ProductID: 1, UnitPrice: 125, Quantity: 2, LineTotal: 250}}, payload.Items)
}
