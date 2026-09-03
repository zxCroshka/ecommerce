package redis

import "github.com/zxCroshka/ecommerce/services/product-service/internal/domain"

type Cache struct {
	Generation int64             `json:"generation"`
	Products   []*domain.Product `json:"products"`
	Total      int64             `json:"total"`
}

type ProductCache struct {
	Generation int64           `json:"generation"`
	Product    *domain.Product `json:"product"`
}

func NewCache(generation int64, products []*domain.Product, total int64) *Cache {
	return &Cache{Generation: generation, Products: products, Total: total}
}
