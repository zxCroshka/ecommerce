package grpc

import (
	"context"
	"errors"
	"sort"

	"github.com/zxCroshka/ecommerce/services/cart-service/internal/auth"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/cart-service/internal/domain"
	cartservicev1 "github.com/zxCroshka/ecommerce/shared/cartservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CartService interface {
	GetCartProducts(ctx context.Context) (*domain.Cart, error)
	AddProductToCart(ctx context.Context, productID, quantity int64) (int64, int64, error)
	RemoveProductFromCart(ctx context.Context, productID int64) error
	ChangeProductQuantity(ctx context.Context, productID, quantity int64) error
	GetCartForCheckout(ctx context.Context, userID int64) (*domain.CartSnapshot, error)
	ClearCartIfUnchanged(ctx context.Context, userID, revision int64) (bool, error)
}

type ServerAPI struct {
	cartService CartService
	cartservicev1.UnimplementedCartServer
}

func RegisterServerAPI(server *grpc.Server, cartService CartService) {
	cartservicev1.RegisterCartServer(server, &ServerAPI{cartService: cartService})
}

func (s *ServerAPI) GetCart(ctx context.Context, req *cartservicev1.GetCartRequest) (*cartservicev1.GetCartResponse, error) {
	if err := validateAuthenticatedUserID(ctx, req.GetUserId()); err != nil {
		return nil, err
	}

	cart, err := s.cartService.GetCartProducts(ctx)
	if err != nil {
		return nil, mapError(err)
	}

	return &cartservicev1.GetCartResponse{Items: cartItemsToProto(cart)}, nil
}

func (s *ServerAPI) AddProduct(ctx context.Context, req *cartservicev1.AddProductRequest) (*cartservicev1.AddProductResponse, error) {
	if err := validateAuthenticatedUserID(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	if err := validateCartItem(req.GetProduct()); err != nil {
		return nil, mapError(err)
	}

	item := req.GetProduct()
	newQuantity, currentQuantity, err := s.cartService.AddProductToCart(
		ctx, item.GetProductId(), item.GetQuantity(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	return &cartservicev1.AddProductResponse{
		NewQuantity:     newQuantity,
		CurrentQuantity: currentQuantity,
	}, nil
}

func (s *ServerAPI) RemoveProduct(ctx context.Context, req *cartservicev1.RemoveProductRequest) (*cartservicev1.RemoveProductResponse, error) {
	if err := validateAuthenticatedUserID(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	if req.GetProductId() <= 0 {
		return nil, mapError(customerrors.ErrInvalidProductID)
	}
	if err := s.cartService.RemoveProductFromCart(ctx, req.GetProductId()); err != nil {
		return nil, mapError(err)
	}
	return &cartservicev1.RemoveProductResponse{}, nil
}

func (s *ServerAPI) ChangeProductQuantity(ctx context.Context, req *cartservicev1.ChangeProductQuantityRequest) (*cartservicev1.ChangeProductQuantityResponse, error) {
	if err := validateAuthenticatedUserID(ctx, req.GetUserId()); err != nil {
		return nil, err
	}
	if err := validateChangeCartItem(req.GetProduct()); err != nil {
		return nil, mapError(err)
	}

	item := req.GetProduct()
	if err := s.cartService.ChangeProductQuantity(
		ctx, item.GetProductId(), item.GetQuantity(),
	); err != nil {
		return nil, mapError(err)
	}
	return &cartservicev1.ChangeProductQuantityResponse{}, nil
}

func (s *ServerAPI) CheckoutCart(ctx context.Context, req *cartservicev1.CheckoutCartRequest) (*cartservicev1.CheckoutCartResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, mapError(customerrors.ErrInvalidUserID)
	}
	snapshot, err := s.cartService.GetCartForCheckout(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return &cartservicev1.CheckoutCartResponse{
		Items:    cartItemsToProto(&domain.Cart{Items: snapshot.Items}),
		Revision: snapshot.Revision,
	}, nil
}

func (s *ServerAPI) ClearCartIfUnchanged(
	ctx context.Context,
	req *cartservicev1.ClearCartIfUnchangedRequest,
) (*cartservicev1.ClearCartIfUnchangedResponse, error) {
	if req.GetUserId() <= 0 {
		return nil, mapError(customerrors.ErrInvalidUserID)
	}
	if req.GetRevision() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "revision must be positive")
	}
	cleared, err := s.cartService.ClearCartIfUnchanged(ctx, req.GetUserId(), req.GetRevision())
	if err != nil {
		return nil, mapError(err)
	}
	return &cartservicev1.ClearCartIfUnchangedResponse{Cleared: cleared}, nil
}

func validateChangeCartItem(item *cartservicev1.CartItem) error {
	switch {
	case item == nil || item.GetProductId() <= 0:
		return customerrors.ErrInvalidProductID
	default:
		return nil
	}
}

func validateCartItem(item *cartservicev1.CartItem) error {
	switch {
	case item == nil || item.GetProductId() <= 0:
		return customerrors.ErrInvalidProductID
	case item.GetQuantity() < 0:
		return customerrors.ErrInvalidQuantity
	default:
		return nil
	}
}

func validateAuthenticatedUserID(ctx context.Context, requestedUserID int64) error {
	identity, ok := auth.UserIdentityFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "authenticated user identity is missing")
	}
	if requestedUserID < 0 {
		return mapError(customerrors.ErrInvalidUserID)
	}
	if requestedUserID > 0 && requestedUserID != identity.UserID {
		return status.Error(codes.PermissionDenied, "cart belongs to another user")
	}
	return nil
}

func mapError(err error) error {
	switch {
	case errors.Is(err, customerrors.ErrInvalidUserID),
		errors.Is(err, customerrors.ErrInvalidProductID),
		errors.Is(err, customerrors.ErrInvalidQuantity),
		errors.Is(err, customerrors.ErrInvalidTTL):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, customerrors.ErrProductNotFound):
		return status.Error(codes.NotFound, customerrors.ErrProductNotFound.Error())
	case errors.Is(err, customerrors.ErrProductInactive),
		errors.Is(err, customerrors.ErrProductOutOfStock),
		errors.Is(err, customerrors.ErrQuantityExceedsStock),
		errors.Is(err, customerrors.ErrQuantityExceedsLimit),
		errors.Is(err, customerrors.ErrCartEmpty):
		return status.Error(codes.FailedPrecondition, rootBusinessMessage(err))
	case errors.Is(err, customerrors.ErrProductServiceUnavailable):
		return status.Error(codes.Unavailable, customerrors.ErrProductServiceUnavailable.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}

func rootBusinessMessage(err error) string {
	for _, candidate := range []error{
		customerrors.ErrProductInactive,
		customerrors.ErrProductOutOfStock,
		customerrors.ErrQuantityExceedsStock,
		customerrors.ErrQuantityExceedsLimit,
		customerrors.ErrCartEmpty,
	} {
		if errors.Is(err, candidate) {
			return candidate.Error()
		}
	}
	return "operation cannot be completed"
}

func cartItemsToProto(cart *domain.Cart) []*cartservicev1.CartItem {
	productIDs := make([]int64, 0, len(cart.Items))
	for productID := range cart.Items {
		productIDs = append(productIDs, int64(productID))
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })

	items := make([]*cartservicev1.CartItem, 0, len(productIDs))
	for _, productID := range productIDs {
		items = append(items, &cartservicev1.CartItem{
			ProductId: productID,
			Quantity:  int64(cart.Items[domain.ProductID(productID)]),
		})
	}
	return items
}
