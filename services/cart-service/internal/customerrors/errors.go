package customerrors

import "errors"

var (
	// Business and validation errors.
	ErrInvalidUserID        = errors.New("invalid user id")
	ErrInvalidProductID     = errors.New("invalid product id")
	ErrInvalidQuantity      = errors.New("quantity cannot be negative")
	ErrInvalidTTL           = errors.New("cart ttl must be greater than zero")
	ErrProductNotFound      = errors.New("product not found")
	ErrProductInactive      = errors.New("product is inactive")
	ErrProductOutOfStock    = errors.New("product is out of stock")
	ErrQuantityExceedsStock = errors.New("quantity exceeds available stock")
	ErrQuantityExceedsLimit = errors.New("quantity exceeds cart limit")
	ErrCartEmpty            = errors.New("cart is empty")
	ErrCartChanged          = errors.New("cart changed after snapshot")

	// Dependency and persistence errors.
	ErrProductServiceUnavailable = errors.New("product service unavailable")
	ErrRedisConnection           = errors.New("failed to connect redis client")
	ErrScriptExecute             = errors.New("failed to execute lua script")
	ErrUnexpectedResult          = errors.New("unexpected script result format")
	ErrUnexpectedQuantityType    = errors.New("unexpected quantity types in script result")
	ErrProductDelete             = errors.New("failed to delete product from cart")
	ErrGetProducts               = errors.New("failed to get cart products")
	ErrInvalidCartData           = errors.New("invalid cart data")
	ErrClearCart                 = errors.New("failed to clear cart")
	ErrCheckoutCart              = errors.New("failed to checkout cart")
	ErrConditionalClear          = errors.New("failed to conditionally clear cart")
	ErrCloseRedis                = errors.New("failed to close redis client")
)
