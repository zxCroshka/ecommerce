package domain

type CartItem struct {
	ProductID int64
	Quantity  int64
}

type CartSnapshot struct {
	Items    []CartItem
	Revision int64
}

type Product struct {
	ID       int64
	Name     string
	Price    int64
	IsActive bool
}
