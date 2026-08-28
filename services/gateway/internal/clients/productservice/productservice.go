package productservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/grpcerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authorizationMetadataKey = "authorization"
	serviceTokenMetadataKey  = "x-service-token"
)

type ProductClient struct {
	api           productservicev1.ProductsClient
	conn          *grpc.ClientConn
	internalToken string
}

type ClientConfig struct {
	Address       string
	InternalToken string
	RetryCount    int
	Timeout       time.Duration
}

func New(cfg ClientConfig) (*ProductClient, error) {
	const op = "grpc.ProductClient.New"
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, fmt.Errorf("%s: address is required", op)
	}
	if strings.TrimSpace(cfg.InternalToken) == "" {
		return nil, fmt.Errorf("%s: internal token is required", op)
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

	return &ProductClient{
		api:           productservicev1.NewProductsClient(conn),
		conn:          conn,
		internalToken: strings.TrimSpace(cfg.InternalToken),
	}, nil
}

func (c *ProductClient) Close() error {
	const op = "grpc.ProductClient.Close"

	if c == nil || c.conn == nil {
		return nil
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *ProductClient) GetProduct(
	ctx context.Context,
	productID int64,
	accessToken string,
) (*domain.Product, error) {
	const op = "grpc.ProductClient.GetProduct"

	response, err := c.api.GetProduct(
		withOptionalBearerToken(ctx, accessToken),
		&productservicev1.GetProductRequest{ProductId: productID},
	)
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateGetProductResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return productFromProto(response.GetProduct()), nil
}

func (c *ProductClient) ListProducts(
	ctx context.Context,
	request domain.ProductListRequest,
	accessToken string,
) (*domain.ProductList, error) {
	const op = "grpc.ProductClient.ListProducts"

	protoRequest, err := listRequestToProto(request)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	response, err := c.api.ListProducts(withOptionalBearerToken(ctx, accessToken), protoRequest)
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateListProductsResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	products := make([]domain.Product, 0, len(response.GetProducts()))
	for _, product := range response.GetProducts() {
		products = append(products, *productFromProto(product))
	}

	return &domain.ProductList{
		Products: products,
		Total:    response.GetTotal(),
		Limit:    response.GetLimit(),
		Offset:   response.GetOffset(),
	}, nil
}

func (c *ProductClient) CreateProduct(
	ctx context.Context,
	accessToken string,
	input domain.CreateProductInput,
) (int64, error) {
	const op = "grpc.ProductClient.CreateProduct"

	response, err := c.api.CreateProduct(
		withBearerToken(ctx, accessToken),
		&productservicev1.CreateProductRequest{
			Name:        input.Name,
			Description: input.Description,
			Price:       input.Price,
			Stock:       input.Stock,
			Category:    input.Category,
			Images:      input.Images,
			IsActive:    input.IsActive,
		},
		grpcretry.Disable(),
	)
	if err != nil {
		return 0, mappingErrors(op, err)
	}
	if err := ValidateCreateProductResponse(response); err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return response.GetProductId(), nil
}

func (c *ProductClient) UpdateProduct(
	ctx context.Context,
	accessToken string,
	productID int64,
	patch domain.ProductPatch,
) error {
	const op = "grpc.ProductClient.UpdateProduct"

	request := &productservicev1.UpdateProductFieldsRequest{
		ProductId:   productID,
		Name:        patch.Name,
		Description: patch.Description,
		Price:       patch.Price,
		Stock:       patch.Stock,
		Category:    patch.Category,
		IsActive:    patch.IsActive,
	}
	if patch.Images != nil {
		request.Images = &productservicev1.ProductImages{Values: *patch.Images}
	}

	response, err := c.api.UpdateProductFields(
		withBearerToken(ctx, accessToken),
		request,
		grpcretry.Disable(),
	)
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateUpdateProductResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *ProductClient) SoftDelete(ctx context.Context, accessToken string, productID int64) error {
	const op = "grpc.ProductClient.SoftDelete"

	response, err := c.api.SoftDelete(
		withBearerToken(ctx, accessToken),
		&productservicev1.SoftDeleteRequest{ProductId: productID},
		grpcretry.Disable(),
	)
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateSoftDeleteResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *ProductClient) ReserveStock(
	ctx context.Context,
	reservationID string,
	productID, quantity int64,
) error {
	const op = "grpc.ProductClient.ReserveStock"

	response, err := c.api.ReserveStock(
		c.withServiceToken(ctx),
		&productservicev1.ReserveStockRequest{
			ProductId:     productID,
			Quantity:      quantity,
			ReservationId: reservationID,
		},
	)
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateReserveStockResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *ProductClient) ReleaseStock(ctx context.Context, reservationID string, productID int64) error {
	const op = "grpc.ProductClient.ReleaseStock"

	response, err := c.api.ReleaseStock(
		c.withServiceToken(ctx),
		&productservicev1.ReleaseStockRequest{
			ProductId:     productID,
			ReservationId: reservationID,
		},
	)
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateReleaseStockResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func withOptionalBearerToken(ctx context.Context, token string) context.Context {
	if strings.TrimSpace(token) == "" {
		return ctx
	}
	return withBearerToken(ctx, token)
}

func withBearerToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(
		ctx,
		authorizationMetadataKey,
		"Bearer "+strings.TrimSpace(token),
	)
}

func (c *ProductClient) withServiceToken(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, serviceTokenMetadataKey, c.internalToken)
}

func listRequestToProto(request domain.ProductListRequest) (*productservicev1.ListProductsRequest, error) {
	sortField := productservicev1.ProductSortField_PRODUCT_SORT_FIELD_UNSPECIFIED
	switch request.Sort {
	case domain.ProductSortDefault:
	case domain.ProductSortByPrice:
		sortField = productservicev1.ProductSortField_PRODUCT_SORT_FIELD_PRICE
	case domain.ProductSortByName:
		sortField = productservicev1.ProductSortField_PRODUCT_SORT_FIELD_NAME
	case domain.ProductSortCreatedAt:
		sortField = productservicev1.ProductSortField_PRODUCT_SORT_FIELD_CREATED_AT
	default:
		return nil, mappingErrors("grpc.ProductClient.ListRequest", status.Error(codes.InvalidArgument, "invalid sort field"))
	}

	sortOrder := productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_UNSPECIFIED
	switch request.Order {
	case domain.ProductOrderDefault:
	case domain.ProductOrderAsc:
		sortOrder = productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_ASC
	case domain.ProductOrderDesc:
		sortOrder = productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_DESC
	default:
		return nil, mappingErrors("grpc.ProductClient.ListRequest", status.Error(codes.InvalidArgument, "invalid sort order"))
	}

	return &productservicev1.ListProductsRequest{
		Category: request.Category,
		IsActive: request.IsActive,
		Sort:     sortField,
		Order:    sortOrder,
		Limit:    request.Limit,
		Offset:   request.Offset,
	}, nil
}

func productFromProto(product *productservicev1.Product) *domain.Product {
	return &domain.Product{
		ID:          product.GetId(),
		Name:        product.GetName(),
		Description: product.GetDescription(),
		Price:       product.GetPrice(),
		Stock:       product.GetStock(),
		Category:    product.GetCategory(),
		Images:      append([]string(nil), product.GetImages()...),
		IsActive:    product.GetIsActive(),
		CreatedAt:   product.GetCreatedAt().AsTime(),
		UpdatedAt:   product.GetUpdatedAt().AsTime(),
	}
}

func mappingErrors(op string, err error) error {
	return grpcerrors.Map(op, err)
}
