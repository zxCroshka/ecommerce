package order

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/response"
)

const (
	defaultPage     int32 = 1
	defaultPageSize int32 = 20
	maxKeyLength          = 128
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

type Service interface {
	CreateOrder(context.Context, string, string) (*domain.CreateOrderResult, error)
	GetOrder(context.Context, string, int64) (*domain.Order, error)
	ListOrders(context.Context, string, int32, int32) (*domain.OrderList, error)
}

type Handlers struct {
	service Service
}

func New(service Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) CreateOrder(ctx *gin.Context) {
	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	key := strings.TrimSpace(ctx.GetHeader("Idempotency-Key"))
	if key == "" || len(key) > maxKeyLength || !idempotencyKeyPattern.MatchString(key) {
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}
	result, err := h.service.CreateOrder(ctx.Request.Context(), principal.AccessToken, key)
	if err != nil {
		response.WriteError(ctx, err)
		return
	}
	if result == nil || result.Order == nil {
		response.WriteError(ctx, customerrors.ErrInternal)
		return
	}
	statusCode := http.StatusOK
	if result.Order.Status == domain.OrderStatusPending {
		statusCode = http.StatusAccepted
	} else if result.Created {
		statusCode = http.StatusCreated
	}
	ctx.JSON(statusCode, gin.H{"success": true, "data": orderResponse(result.Order)})
}

func (h *Handlers) GetOrder(ctx *gin.Context) {
	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	orderID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}
	result, err := h.service.GetOrder(ctx.Request.Context(), principal.AccessToken, orderID)
	if err != nil {
		response.WriteError(ctx, err)
		return
	}
	if result == nil {
		response.WriteError(ctx, customerrors.ErrInternal)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "data": orderResponse(result)})
}

func (h *Handlers) ListOrders(ctx *gin.Context) {
	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	var query ListQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}
	if query.Page == 0 {
		query.Page = defaultPage
	}
	if query.PageSize == 0 {
		query.PageSize = defaultPageSize
	}
	result, err := h.service.ListOrders(
		ctx.Request.Context(),
		principal.AccessToken,
		query.PageSize,
		(query.Page-1)*query.PageSize,
	)
	if err != nil {
		response.WriteError(ctx, err)
		return
	}
	if result == nil {
		response.WriteError(ctx, customerrors.ErrInternal)
		return
	}
	orders := make([]OrderResponse, 0, len(result.Orders))
	for _, value := range result.Orders {
		orders = append(orders, orderResponse(value))
	}
	totalPages := int64(0)
	if result.Limit > 0 {
		totalPages = (result.Total + int64(result.Limit) - 1) / int64(result.Limit)
	}
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"orders": orders,
			"pagination": gin.H{
				"page":        query.Page,
				"page_size":   result.Limit,
				"total":       result.Total,
				"total_pages": totalPages,
			},
		},
	})
}
