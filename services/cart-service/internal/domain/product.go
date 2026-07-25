package domain

import (
	"fmt"
	"strconv"
)

type Quantity int64
type ProductID int64

type Cart struct {
	Items map[ProductID]Quantity
}

func NewCart(items map[string]string) (*Cart, error) {
	tmp := make(map[ProductID]Quantity)
	for k, v := range items {
		id, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid product id %q: %w", k, err)
		}
		quan, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid quantity %q for product %q: %w", v, k, err)
		}
		tmp[ProductID(id)] = Quantity(quan)
	}
	return &Cart{
		Items: tmp,
	}, nil
}
