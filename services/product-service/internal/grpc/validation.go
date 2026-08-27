package grpc

import (
	"strings"

	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateGetProduct(req *productservicev1.GetProductRequest) error {
	if req == nil || req.GetProductId() <= 0 {
		return status.Error(codes.InvalidArgument, "product_id must be positive")
	}
	return nil
}

func ValidateListProducts(req *productservicev1.ListProductsRequest, maxLimit int32) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetLimit() < 0 || req.GetLimit() > maxLimit {
		return status.Errorf(codes.InvalidArgument, "limit must be between 0 and %d", maxLimit)
	}
	if req.GetOffset() < 0 {
		return status.Error(codes.InvalidArgument, "offset cannot be negative")
	}
	if req.Category != nil && strings.TrimSpace(req.GetCategory()) == "" {
		return status.Error(codes.InvalidArgument, "category cannot be empty")
	}
	switch req.GetSort() {
	case productservicev1.ProductSortField_PRODUCT_SORT_FIELD_UNSPECIFIED,
		productservicev1.ProductSortField_PRODUCT_SORT_FIELD_PRICE,
		productservicev1.ProductSortField_PRODUCT_SORT_FIELD_NAME,
		productservicev1.ProductSortField_PRODUCT_SORT_FIELD_CREATED_AT:
	default:
		return status.Error(codes.InvalidArgument, "invalid sort field")
	}
	switch req.GetOrder() {
	case productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_UNSPECIFIED,
		productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_ASC,
		productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_DESC:
	default:
		return status.Error(codes.InvalidArgument, "invalid sort order")
	}
	return nil
}

func ValidateCreateProduct(req *productservicev1.CreateProductRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	return nil
}

func ValidateUpdateProductFields(req *productservicev1.UpdateProductFieldsRequest) error {
	if req == nil || req.GetProductId() <= 0 {
		return status.Error(codes.InvalidArgument, "product_id must be positive")
	}
	if req.Name == nil &&
		req.Description == nil &&
		req.Price == nil &&
		req.Stock == nil &&
		req.Category == nil &&
		req.Images == nil &&
		req.IsActive == nil {
		return status.Error(codes.InvalidArgument, "at least one field must be provided")
	}
	return nil
}

func ValidateSoftDelete(req *productservicev1.SoftDeleteRequest) error {
	if req == nil || req.GetProductId() <= 0 {
		return status.Error(codes.InvalidArgument, "product_id must be positive")
	}
	return nil
}

func ValidateReserveStock(req *productservicev1.ReserveStockRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if strings.TrimSpace(req.GetReservationId()) == "" {
		return status.Error(codes.InvalidArgument, "reservation_id is required")
	}
	if req.GetProductId() <= 0 {
		return status.Error(codes.InvalidArgument, "product_id must be positive")
	}
	if req.GetQuantity() <= 0 {
		return status.Error(codes.InvalidArgument, "quantity must be positive")
	}
	return nil
}

func ValidateReleaseStock(req *productservicev1.ReleaseStockRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetProductId() <= 0 {
		return status.Error(codes.InvalidArgument, "product_id must be positive")
	}
	if strings.TrimSpace(req.GetReservationId()) == "" {
		return status.Error(codes.InvalidArgument, "reservation_id is required")
	}
	return nil
}
