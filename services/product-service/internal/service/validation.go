package service

import (
	"errors"
	"fmt"
	"strings"
)

func (s *ProductService) validateProductName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) < 3 {
		return errors.New("name must be at least 3 characters")
	}
	if len(name) > 255 {
		return errors.New("name must not exceed 255 characters")
	}
	return nil
}

func (s *ProductService) validatePrice(price int64) error {
	if price < 0 {
		return errors.New("price cannot be negative")
	}
	return nil
}

func (s *ProductService) validateStock(stock int64) error {
	if stock < 0 {
		return errors.New("stock cannot be negative")
	}
	return nil
}

func (s *ProductService) validateCategory(category string) error {
	category = strings.TrimSpace(category)
	if category == "" {
		return errors.New("category is required")
	}
	if len(category) < 2 {
		return errors.New("category must be at least 2 characters")
	}
	if len(category) > 100 {
		return errors.New("category must not exceed 100 characters")
	}
	return nil
}

func (s *ProductService) validateImages(images []string) error {
	if len(images) > 10 {
		return errors.New("maximum 10 images allowed")
	}
	for i, img := range images {
		if img == "" {
			return fmt.Errorf("image URL at index %d is empty", i)
		}
		if !strings.HasPrefix(img, "http://") && !strings.HasPrefix(img, "https://") {
			return fmt.Errorf("image URL must start with http:// or https://")
		}
	}
	return nil
}

func (s *ProductService) validateQuantity(quantity int64) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	return nil
}
