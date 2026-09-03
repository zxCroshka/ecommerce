package events

import (
	"fmt"
	"time"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

const (
	ProductUpdatedType    = "product.updated"
	ProductUpdatedVersion = 1

	OperationUpdated       = "updated"
	OperationSoftDeleted   = "soft_deleted"
	OperationStockReserved = "stock_reserved"
	OperationStockReleased = "stock_released"
)

// ProductUpdatedPayload is a stable public event contract. The product state is
// a post-transaction snapshot; reservation fields are populated for stock flows.
type ProductUpdatedPayload struct {
	ProductID     int64    `json:"product_id"`
	Operation     string   `json:"operation"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Price         int64    `json:"price"`
	Stock         int64    `json:"stock"`
	Category      string   `json:"category"`
	Images        []string `json:"images"`
	IsActive      bool     `json:"is_active"`
	ReservationID string   `json:"reservation_id,omitempty"`
	Quantity      int64    `json:"quantity,omitempty"`
}

func NewProductUpdated(
	topic, operation string,
	product *domain.Product,
	reservationID string,
	quantity int64,
	occurredAt time.Time,
) (outbox.Event, error) {
	if product == nil {
		return outbox.Event{}, fmt.Errorf("product.updated payload requires a product")
	}
	switch operation {
	case OperationUpdated, OperationSoftDeleted, OperationStockReserved, OperationStockReleased:
	default:
		return outbox.Event{}, fmt.Errorf("unsupported product.updated operation %q", operation)
	}
	return outbox.NewEvent(
		topic,
		ProductUpdatedType,
		"product",
		outbox.AggregateInt64(product.Id),
		ProductUpdatedVersion,
		ProductUpdatedPayload{
			ProductID:     product.Id,
			Operation:     operation,
			Name:          product.Name,
			Description:   product.Description,
			Price:         product.Price,
			Stock:         product.Stock,
			Category:      product.Category,
			Images:        append([]string(nil), product.Images...),
			IsActive:      product.IsActive,
			ReservationID: reservationID,
			Quantity:      quantity,
		},
		occurredAt,
	)
}
