package domain

import "time"

type Product struct {
	ID          int64
	Name        string
	Description string
	Price       int64
	Stock       int64
	Category    string
	Images      []string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProductSortField string

const (
	ProductSortDefault   ProductSortField = ""
	ProductSortByPrice   ProductSortField = "price"
	ProductSortByName    ProductSortField = "name"
	ProductSortCreatedAt ProductSortField = "created_at"
)

type ProductSortOrder string

const (
	ProductOrderDefault ProductSortOrder = ""
	ProductOrderAsc     ProductSortOrder = "asc"
	ProductOrderDesc    ProductSortOrder = "desc"
)

type ProductListRequest struct {
	Category *string
	IsActive *bool
	Sort     ProductSortField
	Order    ProductSortOrder
	Limit    int32
	Offset   int32
}

type ProductList struct {
	Products []Product
	Total    int64
	Limit    int32
	Offset   int32
}

type CreateProductInput struct {
	Name        string
	Description string
	Price       int64
	Stock       int64
	Category    string
	Images      []string
	IsActive    *bool
}

type ProductPatch struct {
	Name        *string
	Description *string
	Price       *int64
	Stock       *int64
	Category    *string
	Images      *[]string
	IsActive    *bool
}
