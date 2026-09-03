package events

import (
	"fmt"
	"time"

	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

const (
	OrderCreatedType    = "order.created"
	OrderCreatedVersion = 1
)

type OrderItemSummary struct {
	ProductID int64 `json:"product_id"`
	UnitPrice int64 `json:"unit_price"`
	Quantity  int64 `json:"quantity"`
	LineTotal int64 `json:"line_total"`
}

type OrderCreatedPayload struct {
	OrderID     int64              `json:"order_id"`
	UserID      int64              `json:"user_id"`
	TotalAmount int64              `json:"total_amount"`
	Currency    string             `json:"currency"`
	Items       []OrderItemSummary `json:"items"`
}

func NewOrderCreated(topic string, order *domain.Order, occurredAt time.Time) (outbox.Event, error) {
	if order == nil || order.ID <= 0 || order.UserID <= 0 {
		return outbox.Event{}, fmt.Errorf("order.created payload requires a persisted order")
	}
	items := make([]OrderItemSummary, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, OrderItemSummary{
			ProductID: item.ProductID,
			UnitPrice: item.UnitPrice,
			Quantity:  item.Quantity,
			LineTotal: item.LineTotal,
		})
	}
	return outbox.NewEvent(
		topic,
		OrderCreatedType,
		"order",
		outbox.AggregateInt64(order.ID),
		OrderCreatedVersion,
		OrderCreatedPayload{
			OrderID:     order.ID,
			UserID:      order.UserID,
			TotalAmount: order.TotalAmount,
			Currency:    order.Currency,
			Items:       items,
		},
		occurredAt,
	)
}
