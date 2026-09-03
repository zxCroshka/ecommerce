package domain

import "time"

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusFailed    OrderStatus = "failed"
)

type OrderItem struct {
	ProductID   int64
	ProductName string
	UnitPrice   int64
	Quantity    int64
	LineTotal   int64
}

type Order struct {
	ID             int64
	Status         OrderStatus
	TotalAmount    int64
	Currency       string
	IdempotencyKey string
	CartRevision   int64
	Items          []OrderItem
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type CreateOrderResult struct {
	Order   *Order
	Created bool
}

type OrderList struct {
	Orders []*Order
	Total  int64
	Limit  int32
	Offset int32
}
