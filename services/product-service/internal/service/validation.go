package service

import (
	"fmt"
	"strings"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
)

func invalidData(message string) error {
	return fmt.Errorf("%w: %s", customerrors.ErrInvalidProductData, message)
}

func (s *ProductService) validateProductName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return invalidData("name is required")
	}
	if len(name) < 3 {
		return invalidData("name must be at least 3 characters")
	}
	if len(name) > 255 {
		return invalidData("name must not exceed 255 characters")
	}
	return nil
}

func (s *ProductService) validatePrice(price int64) error {
	if price < 0 {
		return invalidData("price cannot be negative")
	}
	return nil
}

func (s *ProductService) validateStock(stock int64) error {
	if stock < 0 {
		return invalidData("stock cannot be negative")
	}
	return nil
}

func (s *ProductService) validateCategory(category string) error {
	category = strings.TrimSpace(category)
	if category == "" {
		return invalidData("category is required")
	}
	if len(category) < 2 {
		return invalidData("category must be at least 2 characters")
	}
	if len(category) > 100 {
		return invalidData("category must not exceed 100 characters")
	}
	return nil
}

func (s *ProductService) validateImages(images []string) error {
	if len(images) > 10 {
		return invalidData("maximum 10 images allowed")
	}
	for i, img := range images {
		if img == "" {
			return invalidData(fmt.Sprintf("image URL at index %d is empty", i))
		}
		if !strings.HasPrefix(img, "http://") && !strings.HasPrefix(img, "https://") {
			return invalidData("image URL must start with http:// or https://")
		}
	}
	return nil
}

func (s *ProductService) validateQuantity(quantity int64) error {
	if quantity <= 0 {
		return invalidData("quantity must be positive")
	}
	return nil
}
