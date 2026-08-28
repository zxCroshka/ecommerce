package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func MapError(err error) APIError {
	switch {
	case errors.Is(err, customerrors.ErrInvalidArgument):
		return APIError{
			Status:  http.StatusBadRequest,
			Code:    "INVALID_ARGUMENT",
			Message: "invalid request",
		}

	case errors.Is(err, customerrors.ErrUnauthenticated):
		return APIError{
			Status:  http.StatusUnauthorized,
			Code:    "UNAUTHENTICATED",
			Message: "authentication required",
		}

	case errors.Is(err, customerrors.ErrPermissionDenied):
		return APIError{
			Status:  http.StatusForbidden,
			Code:    "PERMISSION_DENIED",
			Message: "permission denied",
		}

	case errors.Is(err, customerrors.ErrNotFound):
		return APIError{
			Status:  http.StatusNotFound,
			Code:    "NOT_FOUND",
			Message: "resource not found",
		}

	case errors.Is(err, customerrors.ErrAlreadyExists):
		return APIError{
			Status:  http.StatusConflict,
			Code:    "ALREADY_EXISTS",
			Message: "resource already exists",
		}

	case errors.Is(err, customerrors.ErrFailedPrecondition):
		return APIError{
			Status:  http.StatusUnprocessableEntity,
			Code:    "FAILED_PRECONDITION",
			Message: "operation cannot be completed",
		}

	case errors.Is(err, customerrors.ErrResourceExhausted):
		return APIError{
			Status:  http.StatusTooManyRequests,
			Code:    "RESOURCE_EXHAUSTED",
			Message: "too many requests",
		}

	case errors.Is(err, customerrors.ErrCanceled):
		return APIError{
			Status:  http.StatusRequestTimeout,
			Code:    "REQUEST_CANCELED",
			Message: "request canceled",
		}

	case errors.Is(err, customerrors.ErrDeadlineExceeded):
		return APIError{
			Status:  http.StatusGatewayTimeout,
			Code:    "DEADLINE_EXCEEDED",
			Message: "upstream service timed out",
		}

	case errors.Is(err, customerrors.ErrServiceUnavailable):
		return APIError{
			Status:  http.StatusServiceUnavailable,
			Code:    "SERVICE_UNAVAILABLE",
			Message: "service temporarily unavailable",
		}

	default:
		return APIError{
			Status:  http.StatusInternalServerError,
			Code:    "INTERNAL_ERROR",
			Message: "internal server error",
		}
	}
}

func WriteError(ctx *gin.Context, err error) {
	apiErr := MapError(err)

	ctx.AbortWithStatusJSON(apiErr.Status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    apiErr.Code,
			"message": apiErr.Message,
		},
	})
}
