package grpcerrors

import (
	"context"
	"errors"
	"fmt"

	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Map(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w: %v", op, customerrors.ErrCanceled, err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w: %v", op, customerrors.ErrDeadlineExceeded, err)
	}

	grpcStatus, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%s: %w: %v", op, customerrors.ErrInternal, err)
	}

	mappedError := customerrors.ErrInternal
	switch grpcStatus.Code() {
	case codes.InvalidArgument, codes.OutOfRange:
		mappedError = customerrors.ErrInvalidArgument
	case codes.Unauthenticated:
		mappedError = customerrors.ErrUnauthenticated
	case codes.PermissionDenied:
		mappedError = customerrors.ErrPermissionDenied
	case codes.NotFound:
		mappedError = customerrors.ErrNotFound
	case codes.AlreadyExists:
		mappedError = customerrors.ErrAlreadyExists
	case codes.FailedPrecondition, codes.Aborted:
		mappedError = customerrors.ErrFailedPrecondition
	case codes.ResourceExhausted:
		mappedError = customerrors.ErrResourceExhausted
	case codes.Canceled:
		mappedError = customerrors.ErrCanceled
	case codes.DeadlineExceeded:
		mappedError = customerrors.ErrDeadlineExceeded
	case codes.Unavailable:
		mappedError = customerrors.ErrServiceUnavailable
	}

	return fmt.Errorf("%s: %w: %s", op, mappedError, grpcStatus.Message())
}
