package notification

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/response"
)

const (
	defaultPage     int32 = 1
	defaultPageSize int32 = 20
)

type Service interface {
	ListNotifications(context.Context, string, int32, int32) (*domain.NotificationList, error)
	MarkAsRead(context.Context, string, int64) (*domain.Notification, error)
}

type Handlers struct {
	service Service
}

func New(service Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) ListNotifications(ctx *gin.Context) {
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
	result, err := h.service.ListNotifications(
		ctx.Request.Context(), principal.AccessToken, query.PageSize, (query.Page-1)*query.PageSize,
	)
	if err != nil {
		response.WriteError(ctx, err)
		return
	}
	if result == nil {
		response.WriteError(ctx, customerrors.ErrInternal)
		return
	}
	values := make([]Response, 0, len(result.Notifications))
	for _, notification := range result.Notifications {
		values = append(values, responseFromDomain(notification))
	}
	totalPages := int64(0)
	if result.Limit > 0 {
		totalPages = (result.Total + int64(result.Limit) - 1) / int64(result.Limit)
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"notifications": values,
		"pagination":    gin.H{"page": query.Page, "page_size": result.Limit, "total": result.Total, "total_pages": totalPages},
	}})
}

func (h *Handlers) MarkAsRead(ctx *gin.Context) {
	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}
	notificationID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || notificationID <= 0 {
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}
	notification, err := h.service.MarkAsRead(ctx.Request.Context(), principal.AccessToken, notificationID)
	if err != nil {
		response.WriteError(ctx, err)
		return
	}
	if notification == nil {
		response.WriteError(ctx, customerrors.ErrInternal)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "data": responseFromDomain(notification)})
}
