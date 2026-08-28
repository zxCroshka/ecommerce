package productservice

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestValidateGetProductResponse(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateGetProductResponse(&productservicev1.GetProductResponse{
		Product: validProduct(),
	}))
	require.ErrorIs(t, ValidateGetProductResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateGetProductResponse(&productservicev1.GetProductResponse{}), customerrors.ErrInternal)

	invalid := validProduct()
	invalid.Price = -1
	require.ErrorIs(t, ValidateGetProductResponse(&productservicev1.GetProductResponse{
		Product: invalid,
	}), customerrors.ErrInternal)
}

func TestValidateListProductsResponse(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateListProductsResponse(&productservicev1.ListProductsResponse{
		Products: []*productservicev1.Product{validProduct()},
		Total:    1,
		Limit:    20,
	}))
	require.NoError(t, ValidateListProductsResponse(&productservicev1.ListProductsResponse{
		Products: []*productservicev1.Product{},
		Total:    0,
		Limit:    20,
	}))
	require.ErrorIs(t, ValidateListProductsResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateListProductsResponse(&productservicev1.ListProductsResponse{
		Products: []*productservicev1.Product{validProduct()},
		Total:    0,
		Limit:    20,
	}), customerrors.ErrInternal)
}

func TestValidateProductMutationResponses(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateCreateProductResponse(&productservicev1.CreateProductResponse{ProductId: 10}))
	require.NoError(t, ValidateUpdateProductResponse(&productservicev1.UpdateProductFieldsResponse{}))
	require.NoError(t, ValidateSoftDeleteResponse(&productservicev1.SoftDeleteResponse{}))
	require.NoError(t, ValidateReserveStockResponse(&productservicev1.ReserveStockResponse{}))
	require.NoError(t, ValidateReleaseStockResponse(&productservicev1.ReleaseStockResponse{}))

	require.ErrorIs(t, ValidateCreateProductResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateCreateProductResponse(&productservicev1.CreateProductResponse{}), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateUpdateProductResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateSoftDeleteResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateReserveStockResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateReleaseStockResponse(nil), customerrors.ErrInternal)
}

func TestListRequestToProto(t *testing.T) {
	t.Parallel()

	category := "books"
	isActive := true
	request, err := listRequestToProto(domain.ProductListRequest{
		Category: &category,
		IsActive: &isActive,
		Sort:     domain.ProductSortByPrice,
		Order:    domain.ProductOrderAsc,
		Limit:    10,
		Offset:   20,
	})
	require.NoError(t, err)
	require.Equal(t, productservicev1.ProductSortField_PRODUCT_SORT_FIELD_PRICE, request.GetSort())
	require.Equal(t, productservicev1.ProductSortOrder_PRODUCT_SORT_ORDER_ASC, request.GetOrder())
	require.Equal(t, category, request.GetCategory())
	require.True(t, request.GetIsActive())
	require.EqualValues(t, 10, request.GetLimit())
	require.EqualValues(t, 20, request.GetOffset())

	_, err = listRequestToProto(domain.ProductListRequest{Sort: "unknown"})
	require.ErrorIs(t, err, customerrors.ErrInvalidArgument)
	_, err = listRequestToProto(domain.ProductListRequest{Order: "unknown"})
	require.ErrorIs(t, err, customerrors.ErrInvalidArgument)
}

func validProduct() *productservicev1.Product {
	now := timestamppb.Now()
	return &productservicev1.Product{
		Id:          1,
		Name:        "Product",
		Description: "Description",
		Price:       100,
		Stock:       5,
		Category:    "books",
		Images:      []string{"https://example.com/image.jpg"},
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
