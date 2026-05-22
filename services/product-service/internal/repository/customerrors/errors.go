package customerrors

import "errors"

var (
	ErrProductNotFound    = errors.New("product not found")
	ErrProductExists      = errors.New("product already exists")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrInvalidProductData = errors.New("invalid product data")
	ErrCacheMiss          = errors.New("cache miss")
    ErrMarshal = errors.New("failed to marshal")
    ErrUnmarshal = errors.New("failed to unmarshal")
    ErrSetCache = errors.New("failed to set cache")
    ErrGetCache = errors.New("failed to get from cache")
	ErrForbidden = errors.New("acess denied")
)
