package product

import (
	"time"

	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type CreateProductRequest struct {
	Name        string   `json:"name" binding:"required,min=3,max=255"`
	Description string   `json:"description"`
	Price       *int64   `json:"price" binding:"required,gte=0"`
	Stock       *int64   `json:"stock" binding:"required,gte=0"`
	Category    string   `json:"category" binding:"required,min=2,max=100"`
	Images      []string `json:"images" binding:"omitempty,max=10,dive,url"`
	IsActive    *bool    `json:"is_active"`
}

type UpdateProductRequest struct {
	Name        *string   `json:"name" binding:"omitempty,min=3,max=255"`
	Description *string   `json:"description"`
	Price       *int64    `json:"price" binding:"omitempty,gte=0"`
	Stock       *int64    `json:"stock" binding:"omitempty,gte=0"`
	Category    *string   `json:"category" binding:"omitempty,min=2,max=100"`
	Images      *[]string `json:"images" binding:"omitempty,max=10,dive,url"`
	IsActive    *bool     `json:"is_active"`
}

func (r UpdateProductRequest) Empty() bool {
	return r.Name == nil && r.Description == nil && r.Price == nil && r.Stock == nil &&
		r.Category == nil && r.Images == nil && r.IsActive == nil
}

type ListProductsQuery struct {
	Category *string `form:"category" binding:"omitempty,min=2,max=100"`
	IsActive *bool   `form:"is_active"`
	Sort     string  `form:"sort" binding:"omitempty,oneof=price name created_at"`
	Order    string  `form:"order" binding:"omitempty,oneof=asc desc"`
	Page     int32   `form:"page" binding:"omitempty,min=1"`
	PageSize int32   `form:"page_size" binding:"omitempty,min=1,max=100"`
}

type ProductResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int64     `json:"price"`
	Stock       int64     `json:"stock"`
	Category    string    `json:"category"`
	Images      []string  `json:"images"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func productResponse(product domain.Product) ProductResponse {
	return ProductResponse{
		ID:          product.ID,
		Name:        product.Name,
		Description: product.Description,
		Price:       product.Price,
		Stock:       product.Stock,
		Category:    product.Category,
		Images:      product.Images,
		IsActive:    product.IsActive,
		CreatedAt:   product.CreatedAt,
		UpdatedAt:   product.UpdatedAt,
	}
}
