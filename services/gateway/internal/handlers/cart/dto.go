package cart

type AddProductRequest struct {
	ProductID int64  `json:"product_id" binding:"required,gt=0"`
	Quantity  *int64 `json:"quantity" binding:"required,gte=0"`
}

type ChangeQuantityRequest struct {
	Quantity *int64 `json:"quantity" binding:"required,gte=0"`
}

type CartItemResponse struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}
