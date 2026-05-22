package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
)

type ProductServiceInterface interface {
	CreateProduct(ctx context.Context, name, description string, price, stock int64, category string, images []string, isActive, isAdmin bool) (int64, error)
	GetProduct(ctx context.Context, productID int64, isAdmin bool) (*domain.Product, error)
	UpdateProductFields(ctx context.Context, productID int64, fields map[string]any, isAdmin bool) error
	SoftDelete(ctx context.Context, productID int64, isAdmin bool) error
	ListProducts(ctx context.Context, req domain.ProductListRequest, isAdmin bool) ([]*domain.Product, int64, error)
}

type ProductService struct {
	log            *slog.Logger
	productManager ProductManager
	cacheManager   CacheManager
	producer       Producer
}

type ProductManager interface {
	SaveProduct(
		ctx context.Context,
		name, description string,
		price, stock int64,
		category string,
		images []string,
		isActive bool,
	) (int64, error)
	UpdateProductFields(
		ctx context.Context,
		productID int64,
		fields map[string]any,
	) error
	ListProducts(ctx context.Context, req domain.ProductListRequest) ([]*domain.Product, int64, error)
	SoftDelete(ctx context.Context, productID int64) error
	GetProduct(ctx context.Context, productID int64) (*domain.Product, error)
	ReserveStockTX(ctx context.Context, productID int64, quantity int64) error
	ReleaseStockTX(ctx context.Context, productID int64, quantity int64) error
}

type CacheManager interface {
	SetListProductsCache(ctx context.Context, key string, products []*domain.Product, total int64) error
	GetListProductsCache(ctx context.Context, key string) ([]*domain.Product, int64, error)
	InvalidateProductsCache(ctx context.Context, key string) error
	InvalidateProductsCacheByPattern(ctx context.Context, pattern string) error
	SetProductCache(ctx context.Context, productID int64, product *domain.Product) error
	GetProductCache(ctx context.Context, productID int64) (*domain.Product, error)
	InvalidateProductCache(ctx context.Context, productID int64) error
	InvalidateAllProductCache(ctx context.Context) error
	BuildListCacheKey(filter domain.ProductFilter, sort domain.SortField, order domain.SortOrder, limit, offset int) string
}

type Producer interface {
	ProduceProductUpdated(productID int64, changes map[string]any) error
}

func New(log *slog.Logger, productManager ProductManager, cacheManager CacheManager, producer Producer) *ProductService {
	return &ProductService{
		log:            log,
		productManager: productManager,
		cacheManager:   cacheManager,
		producer:       producer,
	}
}

func (s *ProductService) CreateProduct(
	ctx context.Context,
	name, description string,
	price, stock int64,
	category string,
	images []string,
	isActive bool,
	isAdmin bool,
) (int64, error) {
	const op = "service.CreateProduct"
	log := s.log.With(slog.String("op", op))

	if !isAdmin {
		log.Warn("unauthorized attempt to create product")
		return 0, fmt.Errorf("%s: %w", op, customerrors.ErrForbidden)
	}

	if err := s.validateProductName(name); err != nil {
		log.Warn("validation failed", "error", err)
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	if err := s.validatePrice(price); err != nil {
		log.Warn("validation failed", "error", err)
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	if err := s.validateStock(stock); err != nil {
		log.Warn("validation failed", "error", err)
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	if err := s.validateCategory(category); err != nil {
		log.Warn("validation failed", "error", err)
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	if err := s.validateImages(images); err != nil {
		log.Warn("validation failed", "error", err)
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	productID, err := s.productManager.SaveProduct(ctx, name, description, price, stock, category, images, isActive)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductExists) {
			log.Warn("product already exists")
			return 0, fmt.Errorf("%s: %w", op, err)
		}
		log.Error("failed to save product", "error", err)
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	go func() {
		if err := s.cacheManager.InvalidateAllProductCache(context.Background()); err != nil {
			s.log.Error("failed to invalidate cache after create", "error", err)
		}
	}()

	log.Info("product created successfully", "product_id", productID)
	return productID, nil
}

func (s *ProductService) UpdateProductFields(
	ctx context.Context,
	productID int64,
	fields map[string]any,
	isAdmin bool,
) error {
	const op = "service.UpdateProductFields"
	log := s.log.With(slog.String("op", op), slog.Int64("product_id", productID))

	if !isAdmin {
		log.Warn("unauthorized attempt to update product")
		return fmt.Errorf("%s: %w", op, customerrors.ErrForbidden)
	}

	if len(fields) == 0 {
		return fmt.Errorf("%s: no fields to update", op)
	}

	oldProduct, err := s.productManager.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Error("failed to get product", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if name, ok := fields["name"].(string); ok {
		if err := s.validateProductName(name); err != nil {
			log.Warn("validation failed", "error", err)
			return fmt.Errorf("%s: %w", op, err)
		}
	}
	if price, ok := fields["price"].(int64); ok {
		if err := s.validatePrice(price); err != nil {
			log.Warn("validation failed", "error", err)
			return fmt.Errorf("%s: %w", op, err)
		}
	}
	if stock, ok := fields["stock"].(int64); ok {
		if err := s.validateStock(stock); err != nil {
			log.Warn("validation failed", "error", err)
			return fmt.Errorf("%s: %w", op, err)
		}
	}
	if category, ok := fields["category"].(string); ok {
		if err := s.validateCategory(category); err != nil {
			log.Warn("validation failed", "error", err)
			return fmt.Errorf("%s: %w", op, err)
		}
	}
	if images, ok := fields["images"].([]string); ok {
		if err := s.validateImages(images); err != nil {
			log.Warn("validation failed", "error", err)
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	fields["updated_at"] = time.Now()

	if err := s.productManager.UpdateProductFields(ctx, productID, fields); err != nil {
		log.Error("failed to update product fields", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	changes := make(map[string]any)
	if newPrice, ok := fields["price"]; ok {
		changes["old_price"] = oldProduct.Price
		changes["new_price"] = newPrice
	}
	if newStock, ok := fields["stock"]; ok {
		changes["old_stock"] = oldProduct.Stock
		changes["new_stock"] = newStock
	}
	if isActive, ok := fields["is_active"]; ok {
		changes["is_active"] = isActive
	}

	go func() {
		if err := s.cacheManager.InvalidateAllProductCache(context.Background()); err != nil {
			s.log.Error("failed to invalidate cache after update", "error", err)
		}
	}()

	if len(changes) > 0 {
		go func() {
			if err := s.producer.ProduceProductUpdated(productID, changes); err != nil {
				s.log.Error("failed to send kafka event", "error", err)
			}
		}()
	}

	log.Info("product updated successfully", "product_id", productID)
	return nil
}

func (s *ProductService) ListProducts(ctx context.Context, req domain.ProductListRequest, isAdmin bool) ([]*domain.Product, int64, error) {
	const op = "service.ListProducts"
	log := s.log.With(slog.String("op", op))

	if !isAdmin {
		active := true
		req.Filter.IsActive = &active
	}

	cacheKey := s.cacheManager.BuildListCacheKey(req.Filter, req.Sort, req.Order, req.Limit, req.Offset)

	cachedProducts, cachedTotal, err := s.cacheManager.GetListProductsCache(ctx, cacheKey)
	if err == nil {
		log.Debug("cache hit", "key", cacheKey)
		return cachedProducts, cachedTotal, nil
	}

	log.Debug("cache miss", "key", cacheKey, "error", err)

	products, total, err := s.productManager.ListProducts(ctx, req)
	if err != nil {
		log.Error("failed to list products", "error", err)
		return nil, 0, fmt.Errorf("%s: %w", op, err)
	}

	go func() {
		if err := s.cacheManager.SetListProductsCache(context.Background(), cacheKey, products, total); err != nil {
			s.log.Error("failed to set list cache", "error", err)
		}
	}()

	return products, total, nil
}

func (s *ProductService) GetProduct(ctx context.Context, productID int64, isAdmin bool) (*domain.Product, error) {
	const op = "service.GetProduct"
	log := s.log.With(slog.String("op", op), slog.Int64("product_id", productID))

	cachedProduct, err := s.cacheManager.GetProductCache(ctx, productID)
	if err == nil {
		if !isAdmin && !cachedProduct.IsActive {
			log.Warn("inactive product requested by customer")
			return nil, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Debug("cache hit")
		return cachedProduct, nil
	}

	log.Debug("cache miss", "error", err)

	product, err := s.productManager.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return nil, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Error("failed to get product", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if !isAdmin && !product.IsActive {
		log.Warn("inactive product requested by customer")
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
	}

	go func() {
		if err := s.cacheManager.SetProductCache(context.Background(), productID, product); err != nil {
			s.log.Error("failed to set product cache", "error", err)
		}
	}()

	return product, nil
}

func (s *ProductService) SoftDelete(ctx context.Context, productID int64, isAdmin bool) error {
	const op = "service.SoftDelete"
	log := s.log.With(slog.String("op", op), slog.Int64("product_id", productID))

	if !isAdmin {
		log.Warn("unauthorized attempt to delete product")
		return fmt.Errorf("%s: %w", op, customerrors.ErrForbidden)
	}

	product, err := s.productManager.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Error("failed to get product", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if !product.IsActive {
		log.Warn("product already inactive")
		return fmt.Errorf("%s: product already deleted", op)
	}

	if err := s.productManager.SoftDelete(ctx, productID); err != nil {
		log.Error("failed to soft delete product", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	go func() {
		if err := s.cacheManager.InvalidateAllProductCache(context.Background()); err != nil {
			s.log.Error("failed to invalidate cache after delete", "error", err)
		}
	}()

	go func() {
		changes := map[string]any{
			"is_active":  false,
			"deleted_at": time.Now().Unix(),
		}
		if err := s.producer.ProduceProductUpdated(productID, changes); err != nil {
			s.log.Error("failed to send kafka event", "error", err)
		}
	}()

	log.Info("product soft deleted successfully")
	return nil
}

func (s *ProductService) ReserveStock(ctx context.Context, productID int64, quantity int64) error {
	const op = "service.ReserveStock"
	log := s.log.With(slog.String("op", op), slog.Int64("product_id", productID), slog.Int64("quantity", quantity))

	if err := s.validateQuantity(quantity); err != nil {
		log.Warn("invalid quantity", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	oldProduct, err := s.productManager.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Error("failed to get product", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if !oldProduct.IsActive {
		log.Warn("inactive product cannot be reserved")
		return fmt.Errorf("%s: product is inactive", op)
	}

	if err := s.productManager.ReserveStockTX(ctx, productID, quantity); err != nil {
		if errors.Is(err, customerrors.ErrInsufficientStock) {
			log.Warn("insufficient stock", "available", oldProduct.Stock, "requested", quantity)
			return fmt.Errorf("%s: %w", op, customerrors.ErrInsufficientStock)
		}
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Error("failed to reserve stock", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	go func() {
		if err := s.cacheManager.InvalidateProductCache(context.Background(), productID); err != nil {
			s.log.Error("failed to invalidate product cache", "error", err)
		}
	}()

	go func() {
		changes := map[string]any{
			"stock":             oldProduct.Stock - quantity,
			"reserved_quantity": quantity,
			"operation":         "reserve",
		}
		if err := s.producer.ProduceProductUpdated(productID, changes); err != nil {
			s.log.Error("failed to send kafka event for stock reserve", "error", err)
		}
	}()

	log.Info("stock reserved successfully", "old_stock", oldProduct.Stock, "new_stock", oldProduct.Stock-quantity)
	return nil
}

func (s *ProductService) ReleaseStock(ctx context.Context, productID int64, quantity int64) error {
	const op = "service.ReleaseStock"
	log := s.log.With(slog.String("op", op), slog.Int64("product_id", productID), slog.Int64("quantity", quantity))

	if err := s.validateQuantity(quantity); err != nil {
		log.Warn("invalid quantity", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	oldProduct, err := s.productManager.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Error("failed to get product", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := s.productManager.ReleaseStockTX(ctx, productID, quantity); err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		log.Error("failed to release stock", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	go func() {
		if err := s.cacheManager.InvalidateProductCache(context.Background(), productID); err != nil {
			s.log.Error("failed to invalidate product cache", "error", err)
		}
	}()

	go func() {
		changes := map[string]any{
			"stock":             oldProduct.Stock + quantity,
			"released_quantity": quantity,
			"operation":         "release",
		}
		if err := s.producer.ProduceProductUpdated(productID, changes); err != nil {
			s.log.Error("failed to send kafka event for stock release", "error", err)
		}
	}()
	log.Info("stock released successfully", "old_stock", oldProduct.Stock, "new_stock", oldProduct.Stock+quantity)
	return nil
}
