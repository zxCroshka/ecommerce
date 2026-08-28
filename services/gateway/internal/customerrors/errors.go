package customerrors

import "errors"

var (
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrUnauthenticated    = errors.New("authentication required")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrFailedPrecondition = errors.New("failed precondition")
	ErrResourceExhausted  = errors.New("resource exhausted")
	ErrCanceled           = errors.New("request canceled")
	ErrDeadlineExceeded   = errors.New("request deadline exceeded")
	ErrServiceUnavailable = errors.New("service unavailable")
	ErrInternal           = errors.New("internal error")
)
