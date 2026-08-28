package cartservice

import (
	cartservicev1 "github.com/zxCroshka/ecommerce/shared/cartservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateGetCartResponse(response *cartservicev1.GetCartResponse) error {
	const op = "grpc.CartClient.ValidateGetCartResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	return validateCartItems(op, response.GetItems(), true)
}

func ValidateAddProductResponse(response *cartservicev1.AddProductResponse) error {
	const op = "grpc.CartClient.ValidateAddProductResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	if response.GetNewQuantity() < 0 {
		return invalidResponse(op, "new_quantity cannot be negative")
	}
	if response.GetCurrentQuantity() < 0 {
		return invalidResponse(op, "current_quantity cannot be negative")
	}
	return nil
}

func ValidateRemoveProductResponse(response *cartservicev1.RemoveProductResponse) error {
	return validatePresent("grpc.CartClient.ValidateRemoveProductResponse", response != nil)
}

func ValidateChangeProductQuantityResponse(response *cartservicev1.ChangeProductQuantityResponse) error {
	return validatePresent("grpc.CartClient.ValidateChangeProductQuantityResponse", response != nil)
}

func ValidateCheckoutCartResponse(response *cartservicev1.CheckoutCartResponse) error {
	const op = "grpc.CartClient.ValidateCheckoutCartResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	return validateCartItems(op, response.GetItems(), false)
}

func validateCartItems(op string, items []*cartservicev1.CartItem, allowEmpty bool) error {
	if !allowEmpty && len(items) == 0 {
		return invalidResponse(op, "items are empty")
	}
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item == nil {
			return invalidResponse(op, "cart item is nil")
		}
		if item.GetProductId() <= 0 {
			return invalidResponse(op, "product_id must be positive")
		}
		if item.GetQuantity() <= 0 {
			return invalidResponse(op, "quantity must be positive")
		}
		if _, exists := seen[item.GetProductId()]; exists {
			return invalidResponse(op, "duplicate product_id")
		}
		seen[item.GetProductId()] = struct{}{}
	}
	return nil
}

func validatePresent(op string, present bool) error {
	if !present {
		return invalidResponse(op, "response is nil")
	}
	return nil
}

func invalidResponse(op, message string) error {
	return mappingErrors(op, status.Error(codes.Internal, "invalid cart service response: "+message))
}
