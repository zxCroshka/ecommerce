package grpc

import (
	"context"
	"fmt"
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/customerrors"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Client struct {
	api  productservicev1.ProductsClient
	conn *grpc.ClientConn
}

type ClientConfig struct {
	Address    string
	RetryCount int
	Timeout    time.Duration
}

func New(cfg ClientConfig) (*Client, error) {
	const op = "grpc.New"

	retryopts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.Aborted, codes.DeadlineExceeded, codes.ResourceExhausted, codes.Unavailable),
		grpcretry.WithMax(uint(cfg.RetryCount)),
		grpcretry.WithPerRetryTimeout(cfg.Timeout),
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryopts...)),
	}
	cc, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &Client{api: productservicev1.NewProductsClient(cc), conn: cc}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) GetProduct(ctx context.Context, id int64) (*productservicev1.Product, error) {
	const op = "grpc.GetProduct"
	req := &productservicev1.GetProductRequest{ProductId: id}
	res, err := c.api.GetProduct(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
		}
		return nil, fmt.Errorf("%s: %w: %w", op, customerrors.ErrProductServiceUnavailable, err)
	}
	if res.GetProduct() == nil {
		return nil, fmt.Errorf("%s: %w", op, customerrors.ErrProductNotFound)
	}

	return res.GetProduct(), nil
}
