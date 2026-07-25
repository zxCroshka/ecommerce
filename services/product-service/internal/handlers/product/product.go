package product

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/domain"
	errs "github.com/zxCroshka/ecommerce/services/product-service/internal/handlers/err"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/repository/customerrors"
	"github.com/zxCroshka/ecommerce/services/product-service/internal/service"
)

type ProductHandlers struct {
	log *slog.Logger
	srv service.ProductServiceInterface
}

func New(log *slog.Logger, srv service.ProductServiceInterface) *ProductHandlers {
	return &ProductHandlers{
		log: log,
		srv: srv,
	}
}

type CreateProductRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description" binding:"required"`
	Price       int64    `json:"price" binding:"gte=0"`
	Stock       int64    `json:"stock" binding:"gte=0"`
	Category    string   `json:"category" binding:"required"`
	Images      []string `json:"images" binding:"omitempty,dive,url"`
	IsActive    *bool    `json:"is_active"`
}

func (h *ProductHandlers) CreateProduct(ctx *gin.Context) {
	const op = "handlers.product.CreateProduct"
	log := h.log.With(slog.String("op", op))

	var req CreateProductRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid request body"))
		ctx.Abort()
		return
	}

	isAdmin := adminFromContext(ctx)
	if !isAdmin {
		log.Warn("unauthorized access attempt to create product")
		_ = ctx.Error(errs.NewForbiddenError("only admins can create products"))
		ctx.Abort()
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	productID, err := h.srv.CreateProduct(ctx, req.Name, req.Description, req.Price, req.Stock, req.Category, req.Images, isActive, isAdmin)
	if err != nil {
		handleServiceError(ctx, err, "failed to create product")
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"product_id": productID})
}

type GetProductRequest struct {
	ProductID int64 `uri:"id" binding:"required,min=1"`
}

func (h *ProductHandlers) GetProduct(ctx *gin.Context) {
	const op = "handlers.product.GetProduct"
	log := h.log.With(slog.String("op", op))

	var req GetProductRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		log.Error("failed to bind URI", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid product ID"))
		ctx.Abort()
		return
	}

	// ИСПРАВЛЕНО: не требуем isAdmin для публичного эндпоинта
	isAdmin := false
	isAdmin = adminFromContext(ctx)

	product, err := h.srv.GetProduct(ctx, req.ProductID, isAdmin)
	if err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found", "product_id", req.ProductID)
			_ = ctx.Error(errs.NewNotFoundError("product not found"))
			ctx.Abort()
			return
		}
		log.Error("internal server error", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to get product"))
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusOK, product)
}

type UpdateProductRequest struct {
	ProductID   int64     `uri:"id" binding:"required,min=1"`
	Name        *string   `json:"name" binding:"omitempty"`
	Description *string   `json:"description" binding:"omitempty"`
	Price       *int64    `json:"price" binding:"omitempty,gte=0"`
	Stock       *int64    `json:"stock" binding:"omitempty,gte=0"`
	Category    *string   `json:"category" binding:"omitempty"`
	Images      *[]string `json:"images" binding:"omitempty,dive,url"`
	IsActive    *bool     `json:"is_active"`
}

func (h *ProductHandlers) UpdateProduct(ctx *gin.Context) {
	const op = "handlers.product.UpdateProduct"
	log := h.log.With(slog.String("op", op))

	var req UpdateProductRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		log.Error("failed to bind URI", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid product ID"))
		ctx.Abort()
		return
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind JSON", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid request body"))
		ctx.Abort()
		return
	}

	isAdmin := adminFromContext(ctx)
	if !isAdmin {
		log.Warn("unauthorized access attempt to update product", "product_id", req.ProductID)
		_ = ctx.Error(errs.NewForbiddenError("only admins can update products"))
		ctx.Abort()
		return
	}

	patch := domain.ProductPatch{
		Name: req.Name, Description: req.Description, Price: req.Price, Stock: req.Stock,
		Category: req.Category, IsActive: req.IsActive,
	}
	if req.Images != nil {
		patch.Images = *req.Images
		patch.ImagesSet = true
	}

	if patch.Empty() {
		log.Warn("no fields to update", "product_id", req.ProductID)
		_ = ctx.Error(errs.NewBadRequestError("no fields to update"))
		ctx.Abort()
		return
	}

	if err := h.srv.UpdateProductFields(ctx, req.ProductID, patch, isAdmin); err != nil {
		handleServiceError(ctx, err, "failed to update product")
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "product updated successfully"})
}

type DeleteProductRequest struct {
	ProductID int64 `uri:"id" binding:"required,min=1"`
}

func (h *ProductHandlers) DeleteProduct(ctx *gin.Context) {
	const op = "handlers.product.DeleteProduct"
	log := h.log.With(slog.String("op", op))

	var req DeleteProductRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		log.Error("failed to bind URI", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid product ID"))
		ctx.Abort()
		return
	}

	isAdmin := adminFromContext(ctx)
	if !isAdmin {
		log.Warn("unauthorized access attempt to delete product", "product_id", req.ProductID)
		_ = ctx.Error(errs.NewForbiddenError("only admins can delete products"))
		ctx.Abort()
		return
	}

	if err := h.srv.SoftDelete(ctx, req.ProductID, isAdmin); err != nil {
		if errors.Is(err, customerrors.ErrProductNotFound) {
			log.Warn("product not found", "product_id", req.ProductID)
			_ = ctx.Error(errs.NewNotFoundError("product not found"))
			ctx.Abort()
			return
		}
		log.Error("internal server error", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to delete product"))
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "product deleted successfully"})
}

type ListProductsRequest struct {
	Category *string `form:"category" binding:"omitempty"`
	IsActive *bool   `form:"is_active" binding:"omitempty"`
	Page     int     `form:"page" binding:"omitempty,min=1"`
	PageSize int     `form:"page_size" binding:"omitempty,min=1,max=100"`
	Sort     string  `form:"sort" binding:"omitempty,oneof=price_asc price_desc name_asc name_desc created_at_desc created_at_asc"`
}

func (h *ProductHandlers) ListProducts(ctx *gin.Context) {
	const op = "handlers.product.ListProducts"
	log := h.log.With(slog.String("op", op))

	var req ListProductsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		log.Error("failed to bind query parameters", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid query parameters"))
		ctx.Abort()
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	limit := req.PageSize
	offset := (req.Page - 1) * req.PageSize

	// ИСПРАВЛЕНО: не требуем isAdmin для публичного эндпоинта
	isAdmin := false
	isAdmin = adminFromContext(ctx)

	products, total, err := h.srv.ListProducts(ctx, domain.ProductListRequest{
		Filter: domain.ProductFilter{
			Category: req.Category,
			IsActive: req.IsActive,
		},
		Sort:   parseSort(req.Sort),
		Order:  parseOrder(req.Sort),
		Limit:  limit,
		Offset: offset,
	}, isAdmin)

	if err != nil {
		log.Error("internal server error", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to list products"))
		ctx.Abort()
		return
	}

	totalPages := int64(0)
	if req.PageSize > 0 {
		totalPages = (total + int64(req.PageSize) - 1) / int64(req.PageSize)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"products": products,
		"pagination": gin.H{
			"total":       total,
			"page":        req.Page,
			"page_size":   req.PageSize,
			"total_pages": totalPages,
		},
	})
}

// ========== HELPERS ==========

func parseSort(sort string) domain.SortField {
	switch sort {
	case "price_asc", "price_desc":
		return domain.SortByPrice
	case "created_at_desc", "created_at_asc":
		return domain.SortByCreatedAt
	case "name_asc", "name_desc":
		return domain.SortByName
	default:
		return domain.SortByCreatedAt
	}
}

func adminFromContext(ctx *gin.Context) bool {
	isAdmin, ok := ctx.Get("isAdmin")
	if !ok {
		return false
	}
	admin, ok := isAdmin.(bool)
	return ok && admin
}

func handleServiceError(ctx *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, customerrors.ErrInvalidProductData):
		_ = ctx.Error(errs.NewBadRequestError(err.Error()))
	case errors.Is(err, customerrors.ErrForbidden):
		_ = ctx.Error(errs.NewForbiddenError("access denied"))
	case errors.Is(err, customerrors.ErrProductNotFound):
		_ = ctx.Error(errs.NewNotFoundError("product not found"))
	case errors.Is(err, customerrors.ErrProductExists):
		_ = ctx.Error(errs.NewConflictError("product already exists"))
	default:
		_ = ctx.Error(errs.NewInternalServerError(fallback))
	}
}

func parseOrder(sort string) domain.SortOrder {
	if strings.HasSuffix(sort, "_desc") {
		return domain.SortDesc
	}
	return domain.SortAsc
}
