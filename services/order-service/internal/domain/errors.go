package domain

import "errors"

var (
	ErrOrderNotFound       = errors.New("order not found")
	ErrUnauthenticated     = errors.New("authentication required")
	ErrCartEmpty           = errors.New("cart is empty")
	ErrInvalidOrder        = errors.New("invalid order")
	ErrInvalidIdempotency  = errors.New("invalid idempotency key")
	ErrInvalidTransition   = errors.New("invalid order status transition")
	ErrWorkflowLeaseLost   = errors.New("order workflow lease lost")
	ErrOrderFailed         = errors.New("order creation failed")
	ErrProductUnavailable  = errors.New("product is unavailable")
	ErrInsufficientStock   = errors.New("insufficient stock")
	ErrDownstream          = errors.New("downstream service error")
	ErrAmountOverflow      = errors.New("order amount overflow")
	ErrCompensationPending = errors.New("stock compensation is pending")
)
