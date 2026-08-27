package grpc

import (
	"context"
	"errors"

	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	fallbackDefaultListLimit int32 = 20
	fallbackMaxListLimit     int32 = 100
)

type ProductService interface {
	CreateProduct(
		ctx context.Context,
		name, description string,
		price, stock int64,
		category string,
		images []string,
		isActive, isAdmin bool,
	) (int64, error)
	GetProduct(ctx context.Context, productID int64, isAdmin bool) (*domain.Product, error)
	ListProducts(
		ctx context.Context,
		req domain.ProductListRequest,
		isAdmin bool,
	) ([]*domain.Product, int64, error)
	UpdateProductFields(
		ctx context.Context,
		productID int64,
		patch domain.ProductPatch,
		isAdmin bool,
	) error
	SoftDelete(ctx context.Context, productID int64, isAdmin bool) error
	ReserveStock(
		ctx context.Context,
		reservationID string,
		productID, quantity int64,
	) error
	ReleaseStock(ctx context.Context, reservationID string, productID int64) error
}

type ServerAPI struct {
	productservice   ProductService
	defaultListLimit int32
	maxListLimit     int32
	productservicev1.UnimplementedProductsServer
}

func RegisterServerAPI(
	gRPC *grpc.Server,
	productservice ProductService,
	defaultListLimit, maxListLimit int,
) {
	productservicev1.RegisterProductsServer(gRPC, &ServerAPI{
		productservice:   productservice,
		defaultListLimit: int32(defaultListLimit),
		maxListLimit:     int32(maxListLimit),
	})
}

func (s *ServerAPI) GetProduct(
	ctx context.Context,
	req *productservicev1.GetProductRequest,
) (*productservicev1.GetProductResponse, error) {
	if err := ValidateGetProduct(req); err != nil {
		return nil, err
	}

	product, err := s.productservice.GetProduct(ctx, req.GetProductId(), IsAdmin(ctx))
	if err != nil {
		return nil, mapServiceError(err)
	}
	if product == nil {
		return nil, status.Error(codes.Internal, "product service returned an empty product")
	}

	return &productservicev1.GetProductResponse{Product: product.ToProto()}, nil
}

func (s *ServerAPI) ListProducts(
	ctx context.Context,
	req *productservicev1.ListProductsRequest,
) (*productservicev1.ListProductsResponse, error) {
	defaultLimit, maxLimit := s.paginationLimits()
	if err := ValidateListProducts(req, maxLimit); err != nil {
		return nil, err
	}

	listRequest, responseLimit := productListRequestFromProto(req, defaultLimit)
	products, total, err := s.productservice.ListProducts(ctx, listRequest, IsAdmin(ctx))
	if err != nil {
		return nil, mapServiceError(err)
	}

	protoProducts := make([]*productservicev1.Product, 0, len(products))
	for _, product := range products {
		if product == nil {
			return nil, status.Error(codes.Internal, "product service returned an empty product")
		}
		protoProducts = append(protoProducts, product.ToProto())
	}

	return &productservicev1.ListProductsResponse{
		Products: protoProducts,
		Total:    total,
		Limit:    responseLimit,
		Offset:   req.GetOffset(),
	}, nil
}

func (s *ServerAPI) CreateProduct(
	ctx context.Context,
	req *productservicev1.CreateProductRequest,
) (*productservicev1.CreateProductResponse, error) {
	if err := ValidateCreateProduct(req); err != nil {
		return nil, err
	}

	isActive := true
	if req.IsActive != nil {
		isActive = req.GetIsActive()
	}

	productID, err := s.productservice.CreateProduct(
		ctx,
		req.GetName(),
		req.GetDescription(),
		req.GetPrice(),
		req.GetStock(),
		req.GetCategory(),
		req.GetImages(),
		isActive,
		IsAdmin(ctx),
	)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &productservicev1.CreateProductResponse{ProductId: productID}, nil
}

func (s *ServerAPI) UpdateProductFields(
	ctx context.Context,
	req *productservicev1.UpdateProductFieldsRequest,
) (*productservicev1.UpdateProductFieldsResponse, error) {
	if err := ValidateUpdateProductFields(req); err != nil {
		return nil, err
	}

	patch := domain.ProductPatch{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
		Category:    req.Category,
		IsActive:    req.IsActive,
	}
	if req.GetImages() != nil {
		patch.Images = req.GetImages().GetValues()
		patch.ImagesSet = true
	}

	if err := s.productservice.UpdateProductFields(
		ctx,
		req.GetProductId(),
		patch,
		IsAdmin(ctx),
	); err != nil {
		return nil, mapServiceError(err)
	}

	return &productservicev1.UpdateProductFieldsResponse{}, nil
}

func (s *ServerAPI) SoftDelete(
	ctx context.Context,
	req *productservicev1.SoftDeleteRequest,
) (*productservicev1.SoftDeleteResponse, error) {
	if err := ValidateSoftDelete(req); err != nil {
		return nil, err
	}

	if err := s.productservice.SoftDelete(ctx, req.GetProductId(), IsAdmin(ctx)); err != nil {
		return nil, mapServiceError(err)
	}

	return &productservicev1.SoftDeleteResponse{}, nil
}

func (s *ServerAPI) ReserveStock(
	ctx context.Context,
	req *productservicev1.ReserveStockRequest,
) (*productservicev1.ReserveStockResponse, error) {
	if err := ValidateReserveStock(req); err != nil {
		return nil, err
	}

	if err := s.productservice.ReserveStock(
		ctx,
		req.GetReservationId(),
		req.GetProductId(),
		req.GetQuantity(),
	); err != nil {
		return nil, mapServiceError(err)
	}

	return &productservicev1.ReserveStockResponse{}, nil
}

func (s *ServerAPI) ReleaseStock(
	ctx context.Context,
	req *productservicev1.ReleaseStockRequest,
) (*productservicev1.ReleaseStockResponse, error) {
	if err := ValidateReleaseStock(req); err != nil {
		return nil, err
	}

	if err := s.productservice.ReleaseStock(
		ctx,
		req.GetReservationId(),
		req.GetProductId(),
	); err != nil {
		return nil, mapServiceError(err)
	}

	return &productservicev1.ReleaseStockResponse{}, nil
}

func productListRequestFromProto(
	req *productservicev1.ListProductsRequest,
	defaultLimit int32,
) (domain.ProductListRequest, int32) {
	limit := req.GetLimit()
	if limit == 0 {
		limit = defaultLimit
	}

	sortField := domain.SortByCreatedAt
	switch req.GetSort() {
	case productservicev1.ProductSortField_PRODUCT_SORT_FIELD_PRICE:
		sortField = domain.SortByPrice
	case productservicev1.ProductSortField_PRODUCT_SORT_FIELD_NAME:
		sortField = domain.SortByName
	case productservicev1.ProductSortField_PRODUCT_SORT_FIELD_CREATED_AT:
		sortField = domain.SortByCreatedAt
	}

	sortOrder := domain.SortDesc
	switch req.GetOrder() {
	case productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_ASC:
		sortOrder = domain.SortAsc
	case productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_DESC:
		sortOrder = domain.SortDesc
	}

	return domain.ProductListRequest{
		Filter: domain.ProductFilter{
			Category: req.Category,
			IsActive: req.IsActive,
		},
		Sort:   sortField,
		Order:  sortOrder,
		Limit:  int(limit),
		Offset: int(req.GetOffset()),
	}, limit
}

func (s *ServerAPI) paginationLimits() (int32, int32) {
	defaultLimit := s.defaultListLimit
	if defaultLimit <= 0 {
		defaultLimit = fallbackDefaultListLimit
	}
	maxLimit := s.maxListLimit
	if maxLimit <= 0 {
		maxLimit = fallbackMaxListLimit
	}
	return defaultLimit, maxLimit
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, customerrors.ErrInvalidProductData):
		return status.Error(codes.InvalidArgument, "invalid product data")
	case errors.Is(err, customerrors.ErrForbidden):
		return status.Error(codes.PermissionDenied, "access denied")
	case errors.Is(err, customerrors.ErrProductNotFound):
		return status.Error(codes.NotFound, "product not found")
	case errors.Is(err, customerrors.ErrProductExists):
		return status.Error(codes.AlreadyExists, "product already exists")
	case errors.Is(err, customerrors.ErrInsufficientStock):
		return status.Error(codes.FailedPrecondition, "insufficient stock")
	case errors.Is(err, customerrors.ErrProductInactive):
		return status.Error(codes.FailedPrecondition, "product is inactive")
	case errors.Is(err, customerrors.ErrReservationNotFound):
		return status.Error(codes.NotFound, "reservation not found")
	case errors.Is(err, customerrors.ErrReservationConflict):
		return status.Error(codes.FailedPrecondition, "reservation parameters conflict")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
