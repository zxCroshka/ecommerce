package cartservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/grpcerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	cartservicev1 "github.com/zxCroshka/ecommerce/shared/cartservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
)

type CartClient struct {
	api  cartservicev1.CartClient
	conn *grpc.ClientConn
}

type ClientConfig struct {
	Address    string
	RetryCount int
	Timeout    time.Duration
}

func New(cfg ClientConfig) (*CartClient, error) {
	const op = "grpc.CartClient.New"
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, fmt.Errorf("%s: address is required", op)
	}
	if cfg.RetryCount < 0 {
		return nil, fmt.Errorf("%s: retry count cannot be negative", op)
	}
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("%s: timeout must be positive", op)
	}

	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.Unavailable),
		grpcretry.WithMax(uint(cfg.RetryCount)),
		grpcretry.WithPerRetryTimeout(cfg.Timeout),
	}
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOpts...)),
	}

	conn, err := grpc.NewClient(cfg.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &CartClient{
		api:  cartservicev1.NewCartClient(conn),
		conn: conn,
	}, nil
}

func (c *CartClient) Close() error {
	const op = "grpc.CartClient.Close"

	if c == nil || c.conn == nil {
		return nil
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *CartClient) GetCart(ctx context.Context, userID int64) (*domain.Cart, error) {
	const op = "grpc.CartClient.GetCart"

	response, err := c.api.GetCart(ctx, &cartservicev1.GetCartRequest{UserId: userID})
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateGetCartResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return cartFromItems(response.GetItems()), nil
}

func (c *CartClient) AddProduct(
	ctx context.Context,
	userID, productID, quantity int64,
) (*domain.AddProductResult, error) {
	const op = "grpc.CartClient.AddProduct"

	response, err := c.api.AddProduct(
		ctx,
		&cartservicev1.AddProductRequest{
			UserId: userID,
			Product: &cartservicev1.CartItem{
				ProductId: productID,
				Quantity:  quantity,
			},
		},
		grpcretry.Disable(),
	)
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateAddProductResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &domain.AddProductResult{
		NewQuantity:     response.GetNewQuantity(),
		CurrentQuantity: response.GetCurrentQuantity(),
	}, nil
}

func (c *CartClient) RemoveProduct(ctx context.Context, userID, productID int64) error {
	const op = "grpc.CartClient.RemoveProduct"

	response, err := c.api.RemoveProduct(
		ctx,
		&cartservicev1.RemoveProductRequest{
			UserId:    userID,
			ProductId: productID,
		},
		grpcretry.Disable(),
	)
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateRemoveProductResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *CartClient) ChangeProductQuantity(
	ctx context.Context,
	userID, productID, quantity int64,
) error {
	const op = "grpc.CartClient.ChangeProductQuantity"

	response, err := c.api.ChangeProductQuantity(
		ctx,
		&cartservicev1.ChangeProductQuantityRequest{
			UserId: userID,
			Product: &cartservicev1.CartItem{
				ProductId: productID,
				Quantity:  quantity,
			},
		},
		grpcretry.Disable(),
	)
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateChangeProductQuantityResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *CartClient) CheckoutCart(ctx context.Context, userID int64) (*domain.Cart, error) {
	const op = "grpc.CartClient.CheckoutCart"

	response, err := c.api.CheckoutCart(
		ctx,
		&cartservicev1.CheckoutCartRequest{UserId: userID},
		grpcretry.Disable(),
	)
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateCheckoutCartResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return cartFromItems(response.GetItems()), nil
}

func cartFromItems(items []*cartservicev1.CartItem) *domain.Cart {
	result := make([]domain.CartItem, 0, len(items))
	for _, item := range items {
		result = append(result, domain.CartItem{
			ProductID: item.GetProductId(),
			Quantity:  item.GetQuantity(),
		})
	}
	return &domain.Cart{Items: result}
}

func mappingErrors(op string, err error) error {
	return grpcerrors.Map(op, err)
}
