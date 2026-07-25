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

type ProductService interface {
	ReleaseStock(ctx context.Context, productID int64, quantity int64) error
	ReserveStock(ctx context.Context, productID int64, quantity int64) error
	GetProduct(ctx context.Context, productID int64, isAdmin bool) (*domain.Product, error)
}

type ServerAPI struct {
	productservice ProductService
	productservicev1.UnimplementedProductsServer
}

func RegisterServerAPI(gRPC *grpc.Server, productservice ProductService) {
	productservicev1.RegisterProductsServer(gRPC, &ServerAPI{productservice: productservice})
}

//------------------------------------------------------------

func (s *ServerAPI) GetProduct(ctx context.Context, req *productservicev1.GetProductRequest) (*productservicev1.GetProductResponse, error) {
	if err := ValidateGetProduct(req); err != nil {
		return nil, err
	}
	isAdmin, _ := ctx.Value("isAdmin").(bool)
	product, err := s.productservice.GetProduct(ctx, req.GetProductId(), isAdmin)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	protoProduct := product.ToProto()
	return &productservicev1.GetProductResponse{Product: protoProduct}, nil
}

func (s *ServerAPI) ReserveStock(ctx context.Context, req *productservicev1.ReserveStockRequest) (*productservicev1.ReserveStockResponse, error) {
	if err := ValidateReserveStock(req); err != nil {
		return nil, err
	}

	if err := s.productservice.ReserveStock(ctx, req.ProductId, req.GetQuantity()); err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		if errors.Is(err, customerrors.ErrInsufficientStock) {
			return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
		}
		if errors.Is(err, customerrors.ErrProductInactive) {
			return nil, status.Error(codes.FailedPrecondition, "product is inactive")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &productservicev1.ReserveStockResponse{}, nil
}

func (s *ServerAPI) ReleaseStock(ctx context.Context, req *productservicev1.ReleaseStockRequest) (*productservicev1.ReleaseStockResponse, error) {
	if err := ValidateReleaseStock(req); err != nil {
		return nil, err
	}

	if err := s.productservice.ReleaseStock(ctx, req.ProductId, req.GetQuantity()); err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			return nil, status.Error(codes.NotFound, "product not found")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &productservicev1.ReleaseStockResponse{}, nil
}
