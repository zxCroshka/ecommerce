package domain

type CartItem struct {
	ProductID int64
	Quantity  int64
}

type Cart struct {
	Items []CartItem
}

type AddProductResult struct {
	NewQuantity     int64
	CurrentQuantity int64
}
