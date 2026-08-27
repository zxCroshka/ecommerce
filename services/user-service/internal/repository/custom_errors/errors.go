package customerrors

import "errors"

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrRefreshTokenNotFound = errors.New("refresh token not found or revoked")
	ErrInvalidToken         = errors.New("invalid token")
	ErrTokenBlacklisted     = errors.New("token in black list")
	ErrDuplicateEmail       = errors.New("this email is already used")
	ErrSamePassword         = errors.New("same password as old")
)
