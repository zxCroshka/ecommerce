package grpc

import (
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateGetProduct(req *productservicev1.GetProductRequest) error {
	if req.ProductId <= 0 {
		return status.Error(codes.InvalidArgument, "invalid productID")
	}
	return nil
}

func ValidateReserveStock(req *productservicev1.ReserveStockRequest) error {
	if req.ProductId <= 0 {
		return status.Error(codes.InvalidArgument, "invalid productID")
	}
	if req.Quantity <= 0 {
		return status.Error(codes.InvalidArgument, "invalid quantity")
	}
	return nil
}


func ValidateReleaseStock(req *productservicev1.ReleaseStockRequest) error {
	if req.ProductId <= 0 {
		return status.Error(codes.InvalidArgument, "invalid productID")
	}
	if req.Quantity <= 0 {
		return status.Error(codes.InvalidArgument, "invalid quantity")
	}
	return nil
}

