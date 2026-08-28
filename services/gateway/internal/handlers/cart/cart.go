package cart

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/logging"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/response"
)

type CartService interface {
	GetCart(ctx context.Context, userID int64) (*domain.Cart, error)
	AddProduct(ctx context.Context, userID, productID, quantity int64) (*domain.AddProductResult, error)
	RemoveProduct(ctx context.Context, userID, productID int64) error
	ChangeProductQuantity(ctx context.Context, userID, productID, quantity int64) error
}

type CartHandlers struct {
	log *slog.Logger
	srv CartService
}

func New(log *slog.Logger, srv CartService) *CartHandlers {
	if log == nil {
		log = slog.Default()
	}
	return &CartHandlers{log: log, srv: srv}
}

func (h *CartHandlers) GetCart(ctx *gin.Context) {
	const op = "handlers.cart.GetCart"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	cart, err := h.srv.GetCart(ctx.Request.Context(), principal.Identity.UserID)
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	if cart == nil {
		err := customerrors.ErrInternal
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": cartItemsResponse(cart.Items),
		},
	})
}

func (h *CartHandlers) AddProduct(ctx *gin.Context) {
	const op = "handlers.cart.AddProduct"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	var request AddProductRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	result, err := h.srv.AddProduct(
		ctx.Request.Context(),
		principal.Identity.UserID,
		request.ProductID,
		*request.Quantity,
	)
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	if result == nil {
		err := customerrors.ErrInternal
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"product_id":        request.ProductID,
			"previous_quantity": result.CurrentQuantity,
			"new_quantity":      result.NewQuantity,
		},
	})
}

func (h *CartHandlers) ChangeQuantity(ctx *gin.Context) {
	const op = "handlers.cart.ChangeQuantity"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	productID, err := positivePathID(ctx.Param("product_id"))
	if err != nil {
		response.WriteError(ctx, err)
		return
	}

	var request ChangeQuantityRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	if err := h.srv.ChangeProductQuantity(
		ctx.Request.Context(),
		principal.Identity.UserID,
		productID,
		*request.Quantity,
	); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "cart item quantity updated successfully",
	})
}

func (h *CartHandlers) RemoveProduct(ctx *gin.Context) {
	const op = "handlers.cart.RemoveProduct"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	productID, err := positivePathID(ctx.Param("product_id"))
	if err != nil {
		response.WriteError(ctx, err)
		return
	}

	if err := h.srv.RemoveProduct(
		ctx.Request.Context(),
		principal.Identity.UserID,
		productID,
	); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func cartItemsResponse(items []domain.CartItem) []CartItemResponse {
	responseItems := make([]CartItemResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, CartItemResponse{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
		})
	}
	return responseItems
}

func positivePathID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, customerrors.ErrInvalidArgument
	}
	return id, nil
}
