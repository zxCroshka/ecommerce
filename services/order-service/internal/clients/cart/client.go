package cart

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zxCroshka/ecommerce/services/order-service/internal/domain"
	cartservicev1 "github.com/zxCroshka/ecommerce/shared/cartservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const serviceTokenMetadataKey = "x-service-token"

type Client struct {
	api     cartservicev1.CartClient
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
		return nil, fmt.Errorf("invalid Cart client config")
	}
	conn, err := grpc.NewClient(
		config.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create Cart gRPC client: %w", err)
	}
	return &Client{
		api:     cartservicev1.NewCartClient(conn),
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

func (c *Client) Snapshot(ctx context.Context, userID int64) (*domain.CartSnapshot, error) {
	callCtx, cancel := c.internalContext(ctx)
	defer cancel()
	response, err := c.api.CheckoutCart(callCtx, &cartservicev1.CheckoutCartRequest{UserId: userID})
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			return nil, domain.ErrCartEmpty
		}
		return nil, fmt.Errorf("%w: Cart snapshot: %v", domain.ErrDownstream, err)
	}
	if response.GetRevision() <= 0 || len(response.GetItems()) == 0 {
		return nil, domain.ErrCartEmpty
	}
	items := make([]domain.CartItem, 0, len(response.GetItems()))
	for _, item := range response.GetItems() {
		if item == nil || item.GetProductId() <= 0 || item.GetQuantity() <= 0 {
			return nil, fmt.Errorf("%w: invalid Cart snapshot", domain.ErrDownstream)
		}
		items = append(items, domain.CartItem{ProductID: item.GetProductId(), Quantity: item.GetQuantity()})
	}
	return &domain.CartSnapshot{Items: items, Revision: response.GetRevision()}, nil
}

func (c *Client) ClearIfUnchanged(ctx context.Context, userID, revision int64) (bool, error) {
	callCtx, cancel := c.internalContext(ctx)
	defer cancel()
	response, err := c.api.ClearCartIfUnchanged(callCtx, &cartservicev1.ClearCartIfUnchangedRequest{
		UserId:   userID,
		Revision: revision,
	})
	if err != nil {
		return false, fmt.Errorf("%w: conditional Cart clear: %v", domain.ErrDownstream, err)
	}
	return response.GetCleared(), nil
}

func (c *Client) internalContext(ctx context.Context) (context.Context, context.CancelFunc) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	return metadata.AppendToOutgoingContext(callCtx, serviceTokenMetadataKey, c.token), cancel
}
