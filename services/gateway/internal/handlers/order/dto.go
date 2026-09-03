package order

import (
	"time"

	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type ListQuery struct {
	Page     int32 `form:"page" binding:"omitempty,min=1"`
	PageSize int32 `form:"page_size" binding:"omitempty,min=1,max=100"`
}

type ItemResponse struct {
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	UnitPrice   int64  `json:"unit_price"`
	Quantity    int64  `json:"quantity"`
	LineTotal   int64  `json:"line_total"`
}

type OrderResponse struct {
	ID             int64              `json:"id"`
	Status         domain.OrderStatus `json:"status"`
	TotalAmount    int64              `json:"total_amount"`
	Currency       string             `json:"currency"`
	IdempotencyKey string             `json:"idempotency_key"`
	CartRevision   int64              `json:"cart_revision"`
	Items          []ItemResponse     `json:"items"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

func orderResponse(order *domain.Order) OrderResponse {
	items := make([]ItemResponse, 0, len(order.Items))
	for _, item := range order.Items {
		items = append(items, ItemResponse{
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			UnitPrice:   item.UnitPrice,
			Quantity:    item.Quantity,
			LineTotal:   item.LineTotal,
		})
	}
	return OrderResponse{
		ID:             order.ID,
		Status:         order.Status,
		TotalAmount:    order.TotalAmount,
		Currency:       order.Currency,
		IdempotencyKey: order.IdempotencyKey,
		CartRevision:   order.CartRevision,
		Items:          items,
		CreatedAt:      order.CreatedAt,
		UpdatedAt:      order.UpdatedAt,
	}
}
