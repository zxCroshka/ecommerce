package domain

import (
	"time"
)

type User struct {
	Id        int64
	Email     string
	PassHash  []byte
	Name      string
	IsAdmin   bool
	CreatedAt time.Time
}

func New(id int64,
	email string,
	passHash []byte,
	name string,
	isAdmin bool,
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
		IsAdmin:   isAdmin,
		CreatedAt: createdAt,
	}
}
