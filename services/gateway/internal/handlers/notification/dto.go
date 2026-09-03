package notification

import (
	"time"

	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type ListQuery struct {
	Page     int32 `form:"page" binding:"omitempty,min=1"`
	PageSize int32 `form:"page_size" binding:"omitempty,min=1,max=100"`
}

type Response struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}

func responseFromDomain(value *domain.Notification) Response {
	return Response{
		ID: value.ID, Type: value.Type, Title: value.Title, Body: value.Body,
		CreatedAt: value.CreatedAt, ReadAt: value.ReadAt,
	}
}
