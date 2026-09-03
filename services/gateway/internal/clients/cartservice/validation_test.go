package cartservice

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	cartservicev1 "github.com/zxCroshka/ecommerce/shared/cartservice/gen/go"
)

func TestValidateGetCartResponse(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateGetCartResponse(&cartservicev1.GetCartResponse{}))
	require.NoError(t, ValidateGetCartResponse(&cartservicev1.GetCartResponse{
		Items: []*cartservicev1.CartItem{{ProductId: 1, Quantity: 2}},
	}))
	require.ErrorIs(t, ValidateGetCartResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateGetCartResponse(&cartservicev1.GetCartResponse{
		Items: []*cartservicev1.CartItem{nil},
	}), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateGetCartResponse(&cartservicev1.GetCartResponse{
		Items: []*cartservicev1.CartItem{{ProductId: 1, Quantity: 0}},
	}), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateGetCartResponse(&cartservicev1.GetCartResponse{
		Items: []*cartservicev1.CartItem{
			{ProductId: 1, Quantity: 1},
			{ProductId: 1, Quantity: 2},
		},
	}), customerrors.ErrInternal)
}

func TestValidateAddProductResponse(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateAddProductResponse(&cartservicev1.AddProductResponse{
		NewQuantity:     3,
		CurrentQuantity: 1,
	}))
	require.ErrorIs(t, ValidateAddProductResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateAddProductResponse(&cartservicev1.AddProductResponse{
		NewQuantity: -1,
	}), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateAddProductResponse(&cartservicev1.AddProductResponse{
		CurrentQuantity: -1,
	}), customerrors.ErrInternal)
}

func TestValidateCartMutationResponses(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateRemoveProductResponse(&cartservicev1.RemoveProductResponse{}))
	require.NoError(t, ValidateChangeProductQuantityResponse(&cartservicev1.ChangeProductQuantityResponse{}))
	require.ErrorIs(t, ValidateRemoveProductResponse(nil), customerrors.ErrInternal)
	require.ErrorIs(t, ValidateChangeProductQuantityResponse(nil), customerrors.ErrInternal)
}
