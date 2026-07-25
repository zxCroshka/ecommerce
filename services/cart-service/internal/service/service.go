package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/zxCroshka/ecommerce/services/cart-service/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/domain"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
)

type ProductServiceClient interface {
	GetProduct(ctx context.Context, id int64) (*productservicev1.Product, error)
}

type CartService struct {
	log                  *slog.Logger
	cartManager          CartManager
	productServiceClient ProductServiceClient
	cartTTL              time.Duration
	maxProductQuantity   int64
}

type CartManager interface {
	InsertCartProduct(ctx context.Context, userID, productID, quantity, maxQuantity int64, ttl time.Duration) (int64, int64, error)
	DeleteCartProduct(ctx context.Context, userID, productID int64, ttl time.Duration) (int64, error)
	GetCartProducts(ctx context.Context, userID int64) (*domain.Cart, error)
	ChangeProductQuantity(ctx context.Context, userID, productID, quantity int64, ttl time.Duration) error
	GetCartForCheckout(ctx context.Context, userID int64) (*domain.Cart, error)
}

func NewCartService(
	log *slog.Logger,
	cartManager CartManager,
	productServiceClient ProductServiceClient,
	cartTTL time.Duration,
	maxProductQuantity int64,
) *CartService {
	return &CartService{
		log:                  log,
		cartManager:          cartManager,
		productServiceClient: productServiceClient,
		cartTTL:              cartTTL,
		maxProductQuantity:   maxProductQuantity,
	}
}

func (s *CartService) AddProductToCart(
	ctx context.Context,
	userID, productID, quantity int64,
) (int64, int64, error) {
	const op = "service.AddProductToCart"
	log := s.log.With("op", op, "user_id", userID, "product_id", productID)

	if err := validateAddInput(userID, productID, quantity); err != nil {
		log.Warn("invalid add-to-cart request", "error", err)
		return 0, 0, fmt.Errorf("%s: %w", op, err)
	}

	if quantity == 0 {
		currentQuantity, err := s.cartManager.DeleteCartProduct(ctx, userID, productID, s.cartTTL)
		if err != nil {
			log.Error("failed to remove product from cart", "error", err)
			return 0, 0, fmt.Errorf("%s: %w", op, err)
		}
		log.Info("product removed from cart because quantity is zero")
		return 0, currentQuantity, nil
	}

	product, err := s.validateProductForCart(ctx, log, productID, quantity)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: %w", op, err)
	}
	maxQuantity := min(product.GetStock(), s.maxProductQuantity)

	newQuantity, currentQuantity, err := s.cartManager.InsertCartProduct(
		ctx, userID, productID, quantity, maxQuantity, s.cartTTL,
	)
	if err != nil {
		log.Error("failed to save product in cart", "error", err)
		return 0, 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("product added to cart", "old_quantity", currentQuantity, "new_quantity", newQuantity)
	return newQuantity, currentQuantity, nil
}

func (s *CartService) RemoveProductFromCart(ctx context.Context, userID, productID int64) error {
	const op = "service.RemoveProductFromCart"
	log := s.log.With("op", op, "user_id", userID, "product_id", productID)

	if userID <= 0 {
		log.Warn("invalid remove-from-cart request", "error", customerrors.ErrInvalidUserID)
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidUserID)
	}
	if productID <= 0 {
		log.Warn("invalid remove-from-cart request", "error", customerrors.ErrInvalidProductID)
		return fmt.Errorf("%s: %w", op, customerrors.ErrInvalidProductID)
	}
	if _, err := s.cartManager.DeleteCartProduct(ctx, userID, productID, s.cartTTL); err != nil {
		log.Error("failed to remove product from cart", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	log.Info("product removed from cart")
	return nil
}

func (s *CartService) GetCartProducts(ctx context.Context, userID int64) (*domain.Cart, error) {
	const op = "service.GetCartProducts"
	log := s.log.With("op", op, "user_id", userID)

	if userID <= 0 {
		log.Warn("invalid get-cart request", "error", customerrors.ErrInvalidUserID)
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrInvalidUserID)
	}
	cart, err := s.cartManager.GetCartProducts(ctx, userID)
	if err != nil {
		log.Error("failed to get cart", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	if cart == nil {
		cart = &domain.Cart{Items: make(map[domain.ProductID]domain.Quantity)}
	}

	log.Debug("cart loaded", "items_count", len(cart.Items))
	return cart, nil
}

func (s *CartService) ChangeProductQuantity(
	ctx context.Context,
	userID, productID, quantity int64,
) error {
	const op = "service.ChangeProductQuantity"
	log := s.log.With("op", op, "user_id", userID, "product_id", productID)

	if err := validateIDs(userID, productID); err != nil {
		log.Warn("invalid change-quantity request", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if quantity > 0 {
		if _, err := s.validateProductForCart(ctx, log, productID, quantity); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		if quantity > s.maxProductQuantity {
			return fmt.Errorf("%s: %w", op, customerrors.ErrQuantityExceedsLimit)
		}
	}

	if err := s.cartManager.ChangeProductQuantity(ctx, userID, productID, quantity, s.cartTTL); err != nil {
		log.Error("failed to change product quantity", "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	if quantity <= 0 {
		log.Info("product removed from cart because quantity is not positive")
	} else {
		log.Info("product quantity changed", "new_quantity", quantity)
	}
	return nil
}

func (s *CartService) GetCartForCheckout(ctx context.Context, userID int64) (*domain.Cart, error) {
	const op = "service.GetCartForCheckout"
	log := s.log.With("op", op, "user_id", userID)

	if userID <= 0 {
		log.Warn("invalid checkout request", "error", customerrors.ErrInvalidUserID)
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrInvalidUserID)
	}
	cart, err := s.cartManager.GetCartForCheckout(ctx, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrCartEmpty) {
			log.Warn("checkout requested for empty cart")
			return nil, fmt.Errorf("%s: %w", op, customerrors.ErrCartEmpty)
		}
		log.Error("failed to checkout cart", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("cart returned for checkout and cleared", "items_count", len(cart.Items))
	return cart, nil
}

func (s *CartService) validateProductForCart(
	ctx context.Context,
	log *slog.Logger,
	productID, quantity int64,
) (*productservicev1.Product, error) {
	product, err := s.productServiceClient.GetProduct(ctx, productID)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found")
			return nil, customerrors.ErrProductNotFound
		}
		log.Error("failed to get product", "error", err)
		if errors.Is(err, customerrors.ErrProductServiceUnavailable) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %w", customerrors.ErrProductServiceUnavailable, err)
	}
	if product == nil {
		log.Error("product service returned an empty product")
		return nil, customerrors.ErrProductNotFound
	}
	if !product.GetIsActive() {
		log.Warn("attempt to use inactive product")
		return nil, customerrors.ErrProductInactive
	}
	if product.GetStock() <= 0 {
		log.Warn("product is out of stock")
		return nil, customerrors.ErrProductOutOfStock
	}
	if quantity > product.GetStock() {
		log.Warn("requested quantity exceeds stock", "quantity", quantity, "stock", product.GetStock())
		return nil, customerrors.ErrQuantityExceedsStock
	}
	return product, nil
}

func validateAddInput(userID, productID, quantity int64) error {
	switch {
	case userID <= 0:
		return customerrors.ErrInvalidUserID
	case productID <= 0:
		return customerrors.ErrInvalidProductID
	case quantity < 0:
		return customerrors.ErrInvalidQuantity
	default:
		return nil
	}
}

func validateIDs(userID, productID int64) error {
	switch {
	case userID <= 0:
		return customerrors.ErrInvalidUserID
	case productID <= 0:
		return customerrors.ErrInvalidProductID
	default:
		return nil
	}
}
