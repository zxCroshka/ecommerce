package product

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const serviceTokenMetadataKey = "x-service-token"

type Client struct {
	api     productservicev1.ProductsClient
	conn    *grpc.ClientConn
	token   string
	timeout time.Duration
}

type Config struct {
	Address       string
	InternalToken string
	Timeout       time.Duration
}

func New(config Config) (*Client, error) {
	if strings.TrimSpace(config.Address) == "" || strings.TrimSpace(config.InternalToken) == "" || config.Timeout <= 0 {
		return nil, fmt.Errorf("invalid Product client config")
	}
	conn, err := grpc.NewClient(
		config.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create Product gRPC client: %w", err)
	}
	return &Client{
		api:     productservicev1.NewProductsClient(conn),
		conn:    conn,
		token:   strings.TrimSpace(config.InternalToken),
		timeout: config.Timeout,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *Client) GetProduct(ctx context.Context, productID int64) (*domain.Product, error) {
	callCtx, cancel := c.internalContext(ctx)
	defer cancel()
	response, err := c.api.GetProduct(callCtx, &productservicev1.GetProductRequest{ProductId: productID})
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound, codes.FailedPrecondition:
			return nil, fmt.Errorf("%w: product %d", domain.ErrProductUnavailable, productID)
		default:
			return nil, fmt.Errorf("%w: Product lookup: %v", domain.ErrDownstream, err)
		}
	}
	product := response.GetProduct()
	if product == nil {
		return nil, fmt.Errorf("%w: empty Product response", domain.ErrDownstream)
	}
	return &domain.Product{
		ID:       product.GetId(),
		Name:     product.GetName(),
		Price:    product.GetPrice(),
		IsActive: product.GetIsActive(),
	}, nil
}

func (c *Client) ReserveStock(ctx context.Context, reservationID string, productID, quantity int64) error {
	callCtx, cancel := c.internalContext(ctx)
	defer cancel()
	_, err := c.api.ReserveStock(callCtx, &productservicev1.ReserveStockRequest{
		ReservationId: reservationID,
		ProductId:     productID,
		Quantity:      quantity,
	})
	if err == nil {
		return nil
	}
	grpcStatus := status.Convert(err)
	if grpcStatus.Code() == codes.FailedPrecondition {
		switch grpcStatus.Message() {
		case "insufficient stock":
			return fmt.Errorf("%w: product %d", domain.ErrInsufficientStock, productID)
		case "product is inactive":
			return fmt.Errorf("%w: product %d", domain.ErrProductUnavailable, productID)
		default:
			return fmt.Errorf("%w: %s", domain.ErrInvalidOrder, grpcStatus.Message())
		}
	}
	if grpcStatus.Code() == codes.NotFound {
		return fmt.Errorf("%w: product %d", domain.ErrProductUnavailable, productID)
	}
	return fmt.Errorf("%w: reserve product %d: %v", domain.ErrDownstream, productID, err)
}

func (c *Client) ReleaseStock(ctx context.Context, reservationID string, productID int64) error {
	callCtx, cancel := c.internalContext(ctx)
	defer cancel()
	_, err := c.api.ReleaseStock(callCtx, &productservicev1.ReleaseStockRequest{
		ReservationId: reservationID,
		ProductId:     productID,
	})
	if err == nil || status.Code(err) == codes.NotFound {
		return nil
	}
	return fmt.Errorf("%w: release product %d: %v", domain.ErrDownstream, productID, err)
}

func (c *Client) internalContext(ctx context.Context) (context.Context, context.CancelFunc) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	return metadata.AppendToOutgoingContext(callCtx, serviceTokenMetadataKey, c.token), cancel
}
