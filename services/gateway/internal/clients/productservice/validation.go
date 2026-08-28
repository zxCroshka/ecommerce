package productservice

import (
	"strings"

	productservicev1 "github.com/zxCroshka/ecommerce/shared/productservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateGetProductResponse(response *productservicev1.GetProductResponse) error {
	const op = "grpc.ProductClient.ValidateGetProductResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	return validateProduct(op, response.GetProduct())
}

func ValidateListProductsResponse(response *productservicev1.ListProductsResponse) error {
	const op = "grpc.ProductClient.ValidateListProductsResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	if response.GetTotal() < 0 {
		return invalidResponse(op, "total cannot be negative")
	}
	if response.GetLimit() <= 0 {
		return invalidResponse(op, "limit must be positive")
	}
	if response.GetOffset() < 0 {
		return invalidResponse(op, "offset cannot be negative")
	}
	if int64(len(response.GetProducts())) > response.GetTotal() {
		return invalidResponse(op, "products count exceeds total")
	}
	for _, product := range response.GetProducts() {
		if err := validateProduct(op, product); err != nil {
			return err
		}
	}
	return nil
}

func ValidateCreateProductResponse(response *productservicev1.CreateProductResponse) error {
	const op = "grpc.ProductClient.ValidateCreateProductResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	if response.GetProductId() <= 0 {
		return invalidResponse(op, "product_id must be positive")
	}
	return nil
}

func ValidateUpdateProductResponse(response *productservicev1.UpdateProductFieldsResponse) error {
	return validatePresent("grpc.ProductClient.ValidateUpdateProductResponse", response != nil)
}

func ValidateSoftDeleteResponse(response *productservicev1.SoftDeleteResponse) error {
	return validatePresent("grpc.ProductClient.ValidateSoftDeleteResponse", response != nil)
}

func ValidateReserveStockResponse(response *productservicev1.ReserveStockResponse) error {
	return validatePresent("grpc.ProductClient.ValidateReserveStockResponse", response != nil)
}

func ValidateReleaseStockResponse(response *productservicev1.ReleaseStockResponse) error {
	return validatePresent("grpc.ProductClient.ValidateReleaseStockResponse", response != nil)
}

func validateProduct(op string, product *productservicev1.Product) error {
	if product == nil {
		return invalidResponse(op, "product is nil")
	}
	if product.GetId() <= 0 {
		return invalidResponse(op, "product id must be positive")
	}
	if strings.TrimSpace(product.GetName()) == "" {
		return invalidResponse(op, "product name is empty")
	}
	if product.GetPrice() < 0 {
		return invalidResponse(op, "product price cannot be negative")
	}
	if product.GetStock() < 0 {
		return invalidResponse(op, "product stock cannot be negative")
	}
	if strings.TrimSpace(product.GetCategory()) == "" {
		return invalidResponse(op, "product category is empty")
	}
	for _, image := range product.GetImages() {
		if strings.TrimSpace(image) == "" {
			return invalidResponse(op, "product image is empty")
		}
	}
	if product.GetCreatedAt() == nil || product.GetCreatedAt().CheckValid() != nil {
		return invalidResponse(op, "created_at is invalid")
	}
	if product.GetUpdatedAt() == nil || product.GetUpdatedAt().CheckValid() != nil {
		return invalidResponse(op, "updated_at is invalid")
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
	return mappingErrors(op, status.Error(codes.Internal, "invalid product service response: "+message))
}
