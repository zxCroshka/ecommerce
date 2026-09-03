package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

func TestNewProductUpdatedBuildsTypedVersionedEvent(t *testing.T) {
	product := &domain.Product{
		Id:          7,
		Name:        "Keyboard",
		Description: "Mechanical",
		Price:       12_500,
		Stock:       8,
		Category:    "peripherals",
		Images:      []string{"keyboard.jpg"},
		IsActive:    true,
	}
	event, err := NewProductUpdated(
		"product.updated",
		OperationStockReserved,
		product,
		"order-17",
		2,
		time.Now(),
	)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, event.EventID)
	require.Equal(t, ProductUpdatedType, event.EventType)
	require.Equal(t, ProductUpdatedVersion, event.Version)
	require.Equal(t, "7", event.AggregateID)

	message, err := event.Message()
	require.NoError(t, err)
	var envelope outbox.Envelope
	require.NoError(t, json.Unmarshal(message, &envelope))

	var payload ProductUpdatedPayload
	require.NoError(t, json.Unmarshal(envelope.Payload, &payload))
	require.Equal(t, int64(7), payload.ProductID)
	require.Equal(t, OperationStockReserved, payload.Operation)
	require.Equal(t, "order-17", payload.ReservationID)
	require.Equal(t, int64(2), payload.Quantity)
	require.Equal(t, int64(8), payload.Stock)
}

func TestNewProductUpdatedRejectsInvalidContractInput(t *testing.T) {
	_, err := NewProductUpdated("product.updated", OperationUpdated, nil, "", 0, time.Now())
	require.Error(t, err)

	_, err = NewProductUpdated(
		"product.updated",
		"unknown",
		&domain.Product{Id: 1},
		"",
		0,
		time.Now(),
	)
	require.Error(t, err)
}
