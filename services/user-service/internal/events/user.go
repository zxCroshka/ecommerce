package events

import (
	"time"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/shared/outbox"
)

const (
	UserRegisteredType    = "user.registered"
	UserRegisteredVersion = 1
)

type UserRegisteredPayload struct {
	UserID int64       `json:"user_id"`
	Email  string      `json:"email"`
	Name   string      `json:"name"`
	Role   domain.Role `json:"role"`
}

func NewUserRegistered(
	topic string,
	userID int64,
	email, name string,
	role domain.Role,
	occurredAt time.Time,
) (outbox.Event, error) {
	return outbox.NewEvent(
		topic,
		UserRegisteredType,
		"user",
		outbox.AggregateInt64(userID),
		UserRegisteredVersion,
		UserRegisteredPayload{
			UserID: userID,
			Email:  email,
			Name:   name,
			Role:   role,
		},
		occurredAt,
	)
}
