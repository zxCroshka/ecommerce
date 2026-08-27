package domain

import (
	"time"

	"github.com/jackc/pgx/v5"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Product struct {
	Id          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Price       int64     `json:"price" db:"price"`
	Stock       int64     `json:"stock" db:"stock"`
	Category    string    `json:"category" db:"category"`
	Images      []string  `json:"images" db:"images"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

func (p *Product) ToProto() *productservicev1.Product {
	return &productservicev1.Product{
		Id:          p.Id,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		Category:    p.Category,
		Images:      p.Images,
		IsActive:    p.IsActive,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
}

func (p *Product) FromRow(row pgx.Row) error {
	return row.Scan(&p.Id, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category, &p.Images, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
}

func ProductFactory() *Product {
	return &Product{}
}

type ProductFilter struct {
	Category *string
	IsActive *bool
}

type SortField string

const (
	SortByPrice     SortField = "price"
	SortByName      SortField = "name"
	SortByCreatedAt SortField = "created_at"
)

type ProductPatch struct {
	Name        *string
	Description *string
	Price       *int64
	Stock       *int64
	Category    *string
	Images      []string
	ImagesSet   bool
	IsActive    *bool
}

func (p ProductPatch) Empty() bool {
	return p.Name == nil && p.Description == nil && p.Price == nil && p.Stock == nil &&
		p.Category == nil && !p.ImagesSet && p.IsActive == nil
}

type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

type ProductListRequest struct {
	Filter ProductFilter
	Sort   SortField
	Order  SortOrder
	Limit  int
	Offset int
}
