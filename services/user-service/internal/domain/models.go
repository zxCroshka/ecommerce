package domain

import (
	"time"
)

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAdmin    Role = "admin"
)

type User struct {
	Id        int64
	Email     string
	PassHash  []byte
	Name      string
	Role      Role
	CreatedAt time.Time
}

// Identity contains authentication data that has already been verified by the
// transport layer. Services use it for authorization and session revocation
// without depending on a raw access token.
type Identity struct {
	UserID    int64
	Role      Role
	TokenID   string
	ExpiresAt time.Time
}

func New(id int64,
	email string,
	passHash []byte,
	name string,
	role Role,
	createdAt time.Time,
) *User {
	if len(email) == 0 {
		return nil
	}
	if len(passHash) == 0 {
		return nil
	}
	return &User{
		Id:        id,
		Email:     email,
		PassHash:  passHash,
		Name:      name,
		Role:      role,
		CreatedAt: createdAt,
	}
}
