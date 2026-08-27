package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type productServiceStub struct {
	createProduct       func(context.Context, string, string, int64, int64, string, []string, bool, bool) (int64, error)
	getProduct          func(context.Context, int64, bool) (*domain.Product, error)
	listProducts        func(context.Context, domain.ProductListRequest, bool) ([]*domain.Product, int64, error)
	updateProductFields func(context.Context, int64, domain.ProductPatch, bool) error
	softDelete          func(context.Context, int64, bool) error
	reserveStock        func(context.Context, string, int64, int64) error
	releaseStock        func(context.Context, string, int64) error
}

func (s *productServiceStub) CreateProduct(
	ctx context.Context,
	name, description string,
	price, stock int64,
	category string,
	images []string,
	isActive, isAdmin bool,
) (int64, error) {
	return s.createProduct(ctx, name, description, price, stock, category, images, isActive, isAdmin)
}

func (s *productServiceStub) GetProduct(
	ctx context.Context,
	productID int64,
	isAdmin bool,
) (*domain.Product, error) {
	return s.getProduct(ctx, productID, isAdmin)
}

func (s *productServiceStub) ListProducts(
	ctx context.Context,
	req domain.ProductListRequest,
	isAdmin bool,
) ([]*domain.Product, int64, error) {
	return s.listProducts(ctx, req, isAdmin)
}

func (s *productServiceStub) UpdateProductFields(
	ctx context.Context,
	productID int64,
	patch domain.ProductPatch,
	isAdmin bool,
) error {
	return s.updateProductFields(ctx, productID, patch, isAdmin)
}

func (s *productServiceStub) SoftDelete(
	ctx context.Context,
	productID int64,
	isAdmin bool,
) error {
	return s.softDelete(ctx, productID, isAdmin)
}

func (s *productServiceStub) ReserveStock(
	ctx context.Context,
	reservationID string,
	productID, quantity int64,
) error {
	return s.reserveStock(ctx, reservationID, productID, quantity)
}

func (s *productServiceStub) ReleaseStock(
	ctx context.Context,
	reservationID string,
	productID int64,
) error {
	return s.releaseStock(ctx, reservationID, productID)
}

func adminContext() context.Context {
	return withIdentity(context.Background(), Identity{UserID: 1, Role: adminRole})
}

func TestServerAPI_GetProduct(t *testing.T) {
	now := time.Now().UTC()
	service := &productServiceStub{
		getProduct: func(_ context.Context, productID int64, isAdmin bool) (*domain.Product, error) {
			require.Equal(t, int64(10), productID)
			require.True(t, isAdmin)
			return &domain.Product{
				Id: 10, Name: "Phone", Price: 100, IsActive: true,
				CreatedAt: now, UpdatedAt: now,
			}, nil
		},
	}

	response, err := (&ServerAPI{productservice: service}).GetProduct(
		adminContext(),
		&productservicev1.GetProductRequest{ProductId: 10},
	)

	require.NoError(t, err)
	require.Equal(t, int64(10), response.GetProduct().GetId())
	require.Equal(t, "Phone", response.GetProduct().GetName())
	require.Equal(t, now.Unix(), response.GetProduct().GetCreatedAt().AsTime().Unix())
}

func TestServerAPI_ListProductsConvertsRequestAndDefaults(t *testing.T) {
	category := "phones"
	active := false
	service := &productServiceStub{
		listProducts: func(
			_ context.Context,
			req domain.ProductListRequest,
			isAdmin bool,
		) ([]*domain.Product, int64, error) {
			require.True(t, isAdmin)
			require.Equal(t, category, *req.Filter.Category)
			require.False(t, *req.Filter.IsActive)
			require.Equal(t, domain.SortByCreatedAt, req.Sort)
			require.Equal(t, domain.SortDesc, req.Order)
			require.Equal(t, 15, req.Limit)
			require.Equal(t, 5, req.Offset)
			return []*domain.Product{{Id: 1, Name: "Phone"}}, 21, nil
		},
	}

	response, err := (&ServerAPI{
		productservice:   service,
		defaultListLimit: 15,
		maxListLimit:     50,
	}).ListProducts(
		adminContext(),
		&productservicev1.ListProductsRequest{
			Category: &category,
			IsActive: &active,
			Offset:   5,
		},
	)

	require.NoError(t, err)
	require.Len(t, response.GetProducts(), 1)
	require.Equal(t, int64(21), response.GetTotal())
	require.Equal(t, int32(15), response.GetLimit())
	require.Equal(t, int32(5), response.GetOffset())
}

func TestServerAPI_CreateProductDefaultsToActive(t *testing.T) {
	service := &productServiceStub{
		createProduct: func(
			_ context.Context,
			name, description string,
			price, stock int64,
			category string,
			images []string,
			isActive, isAdmin bool,
		) (int64, error) {
			require.Equal(t, "Phone", name)
			require.Equal(t, "Description", description)
			require.Equal(t, int64(100), price)
			require.Equal(t, int64(3), stock)
			require.Equal(t, "phones", category)
			require.Equal(t, []string{"https://example.com/phone.jpg"}, images)
			require.True(t, isActive)
			require.True(t, isAdmin)
			return 42, nil
		},
	}

	response, err := (&ServerAPI{productservice: service}).CreateProduct(
		adminContext(),
		&productservicev1.CreateProductRequest{
			Name:        "Phone",
			Description: "Description",
			Price:       100,
			Stock:       3,
			Category:    "phones",
			Images:      []string{"https://example.com/phone.jpg"},
		},
	)

	require.NoError(t, err)
	require.Equal(t, int64(42), response.GetProductId())
}

func TestServerAPI_UpdateProductFieldsPreservesPresence(t *testing.T) {
	name := "Updated phone"
	price := int64(0)
	service := &productServiceStub{
		updateProductFields: func(
			_ context.Context,
			productID int64,
			patch domain.ProductPatch,
			isAdmin bool,
		) error {
			require.Equal(t, int64(7), productID)
			require.True(t, isAdmin)
			require.Equal(t, name, *patch.Name)
			require.Equal(t, int64(0), *patch.Price)
			require.True(t, patch.ImagesSet)
			require.Empty(t, patch.Images)
			require.Nil(t, patch.Stock)
			return nil
		},
	}

	_, err := (&ServerAPI{productservice: service}).UpdateProductFields(
		adminContext(),
		&productservicev1.UpdateProductFieldsRequest{
			ProductId: 7,
			Name:      &name,
			Price:     &price,
			Images:    &productservicev1.ProductImages{Values: []string{}},
		},
	)

	require.NoError(t, err)
}

func TestServerAPI_SoftDelete(t *testing.T) {
	service := &productServiceStub{
		softDelete: func(_ context.Context, productID int64, isAdmin bool) error {
			require.Equal(t, int64(8), productID)
			require.True(t, isAdmin)
			return nil
		},
	}

	_, err := (&ServerAPI{productservice: service}).SoftDelete(
		adminContext(),
		&productservicev1.SoftDeleteRequest{ProductId: 8},
	)

	require.NoError(t, err)
}

func TestServerAPI_StockRequests(t *testing.T) {
	service := &productServiceStub{
		reserveStock: func(_ context.Context, reservationID string, productID, quantity int64) error {
			require.Equal(t, "order-1:item-2", reservationID)
			require.Equal(t, int64(2), productID)
			require.Equal(t, int64(4), quantity)
			return nil
		},
		releaseStock: func(_ context.Context, reservationID string, productID int64) error {
			require.Equal(t, "order-1:item-2", reservationID)
			require.Equal(t, int64(2), productID)
			return nil
		},
	}
	server := &ServerAPI{productservice: service}

	_, err := server.ReserveStock(context.Background(), &productservicev1.ReserveStockRequest{
		ReservationId: "order-1:item-2",
		ProductId:     2,
		Quantity:      4,
	})
	require.NoError(t, err)

	_, err = server.ReleaseStock(context.Background(), &productservicev1.ReleaseStockRequest{
		ReservationId: "order-1:item-2",
		ProductId:     2,
	})
	require.NoError(t, err)
}

func TestMapServiceError(t *testing.T) {
	tests := []struct {
		err      error
		wantCode codes.Code
	}{
		{customerrors.ErrInvalidProductData, codes.InvalidArgument},
		{customerrors.ErrForbidden, codes.PermissionDenied},
		{customerrors.ErrProductNotFound, codes.NotFound},
		{customerrors.ErrProductExists, codes.AlreadyExists},
		{customerrors.ErrInsufficientStock, codes.FailedPrecondition},
		{customerrors.ErrProductInactive, codes.FailedPrecondition},
		{customerrors.ErrReservationNotFound, codes.NotFound},
		{customerrors.ErrReservationConflict, codes.FailedPrecondition},
		{errors.New("database failed"), codes.Internal},
	}

	for _, tt := range tests {
		require.Equal(t, tt.wantCode, status.Code(mapServiceError(tt.err)))
	}
}

func TestServerAPI_ValidationRejectsInvalidRequests(t *testing.T) {
	server := &ServerAPI{}

	_, err := server.GetProduct(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = server.ListProducts(context.Background(), &productservicev1.ListProductsRequest{Limit: 101})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = server.CreateProduct(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = server.UpdateProductFields(
		context.Background(),
		&productservicev1.UpdateProductFieldsRequest{ProductId: 1},
	)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = server.SoftDelete(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = server.ReserveStock(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = server.ReleaseStock(context.Background(), nil)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
