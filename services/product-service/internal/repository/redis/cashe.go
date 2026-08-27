package redis

import "github.com/zxCroshka/ecommerce/services/product-service/internal/domain"

type Cache struct {
	Products []*domain.Product `json:"products"`
	Total    int64             `json:"total"`
}

func NewCache(products []*domain.Product, total int64) *Cache {
	return &Cache{Products: products, Total: total}
}
