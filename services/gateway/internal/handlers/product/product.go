package product

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

const (
	defaultPage     int32 = 1
	defaultPageSize int32 = 20
)

type ProductService interface {
	GetProduct(ctx context.Context, productID int64, accessToken string) (*domain.Product, error)
	ListProducts(ctx context.Context, request domain.ProductListRequest, accessToken string) (*domain.ProductList, error)
	CreateProduct(ctx context.Context, accessToken string, input domain.CreateProductInput) (int64, error)
	UpdateProduct(ctx context.Context, accessToken string, productID int64, patch domain.ProductPatch) error
	SoftDelete(ctx context.Context, accessToken string, productID int64) error
}

type ProductHandlers struct {
	log *slog.Logger
	srv ProductService
}

func New(log *slog.Logger, srv ProductService) *ProductHandlers {
	if log == nil {
		log = slog.Default()
	}
	return &ProductHandlers{log: log, srv: srv}
}

func (h *ProductHandlers) GetProduct(ctx *gin.Context) {
	const op = "handlers.product.GetProduct"
	log := h.log.With(slog.String("op", op))

	productID, err := positivePathID(ctx.Param("id"))
	if err != nil {
		response.WriteError(ctx, err)
		return
	}

	product, err := h.srv.GetProduct(ctx.Request.Context(), productID, "")
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	if product == nil {
		err := customerrors.ErrInternal
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    productResponse(*product),
	})
}

func (h *ProductHandlers) ListProducts(ctx *gin.Context) {
	const op = "handlers.product.ListProducts"
	log := h.log.With(slog.String("op", op))

	var query ListProductsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		log.Warn("failed to bind query", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}
	if query.Page == 0 {
		query.Page = defaultPage
	}
	if query.PageSize == 0 {
		query.PageSize = defaultPageSize
	}

	list, err := h.srv.ListProducts(ctx.Request.Context(), domain.ProductListRequest{
		Category: query.Category,
		IsActive: query.IsActive,
		Sort:     domain.ProductSortField(query.Sort),
		Order:    domain.ProductSortOrder(query.Order),
		Limit:    query.PageSize,
		Offset:   (query.Page - 1) * query.PageSize,
	}, "")
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	if list == nil {
		err := customerrors.ErrInternal
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	products := make([]ProductResponse, 0, len(list.Products))
	for _, item := range list.Products {
		products = append(products, productResponse(item))
	}
	totalPages := int64(0)
	if list.Limit > 0 {
		totalPages = (list.Total + int64(list.Limit) - 1) / int64(list.Limit)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"products": products,
			"pagination": gin.H{
				"page":        query.Page,
				"page_size":   list.Limit,
				"total":       list.Total,
				"total_pages": totalPages,
			},
		},
	})
}

func (h *ProductHandlers) CreateProduct(ctx *gin.Context) {
	const op = "handlers.product.CreateProduct"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	var request CreateProductRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	productID, err := h.srv.CreateProduct(ctx.Request.Context(), principal.AccessToken, domain.CreateProductInput{
		Name:        request.Name,
		Description: request.Description,
		Price:       *request.Price,
		Stock:       *request.Stock,
		Category:    request.Category,
		Images:      request.Images,
		IsActive:    request.IsActive,
	})
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"product_id": productID,
		},
	})
}

func (h *ProductHandlers) UpdateProduct(ctx *gin.Context) {
	const op = "handlers.product.UpdateProduct"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	productID, err := positivePathID(ctx.Param("id"))
	if err != nil {
		response.WriteError(ctx, err)
		return
	}

	var request UpdateProductRequest
	if err := ctx.ShouldBindJSON(&request); err != nil || request.Empty() {
		if err != nil {
			log.Warn("failed to bind request", "error", err)
		}
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	err = h.srv.UpdateProduct(ctx.Request.Context(), principal.AccessToken, productID, domain.ProductPatch{
		Name:        request.Name,
		Description: request.Description,
		Price:       request.Price,
		Stock:       request.Stock,
		Category:    request.Category,
		Images:      request.Images,
		IsActive:    request.IsActive,
	})
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "product updated successfully",
	})
}

func (h *ProductHandlers) DeleteProduct(ctx *gin.Context) {
	const op = "handlers.product.DeleteProduct"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	productID, err := positivePathID(ctx.Param("id"))
	if err != nil {
		response.WriteError(ctx, err)
		return
	}

	if err := h.srv.SoftDelete(ctx.Request.Context(), principal.AccessToken, productID); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func positivePathID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, customerrors.ErrInvalidArgument
	}
	return id, nil
}
