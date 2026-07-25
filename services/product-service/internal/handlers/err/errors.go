package errs

import "fmt"

type AppError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewBadRequestError(message string) *AppError {
	return &AppError{
		Status:  400,
		Code:    "BAD_REQUEST",
		Message: message,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Status:  401,
		Code:    "UNAUTHORIZED",
		Message: message,
	}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{
		Status:  403,
		Code:    "FORBIDDEN",
		Message: message,
	}
}

func NewInternalServerError(message string) *AppError {
	return &AppError{
		Status:  500,
		Code:    "INTERNAL_SERVER_ERROR",
		Message: message,
	}
}

func NewNotFoundError(message string) *AppError {
	return &AppError{
		Status:  404,
		Code:    "NOT_FOUND",
		Message: message,
	}
}

func NewConflictError(message string) *AppError {
	return &AppError{Status: 409, Code: "CONFLICT", Message: message}
}
