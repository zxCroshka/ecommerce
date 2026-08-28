package user

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/logging"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/response"
)

type UserService interface {
	GetUser(ctx context.Context, accessToken string) (*domain.User, error)
	UpdateEmail(ctx context.Context, accessToken, newEmail string) error
	UpdateName(ctx context.Context, accessToken, newName string) error
	UpdatePassword(ctx context.Context, accessToken, oldPassword, newPassword string) error
}

type UserHandlers struct {
	log *slog.Logger
	srv UserService
}

func New(log *slog.Logger, srv UserService) *UserHandlers {
	if log == nil {
		log = slog.Default()
	}
	return &UserHandlers{log: log, srv: srv}
}

func (h *UserHandlers) UpdateEmail(ctx *gin.Context) {
	const op = "handlers.user.UpdateEmail"
	log := h.log.With(slog.String("op", op))

	principal, ok := authenticatedPrincipal(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	var request UpdateEmailRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	if err := h.srv.UpdateEmail(
		ctx.Request.Context(),
		principal.AccessToken,
		request.NewEmail,
	); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "email updated successfully",
	})
}

func (h *UserHandlers) UpdateName(ctx *gin.Context) {
	const op = "handlers.user.UpdateName"
	log := h.log.With(slog.String("op", op))

	principal, ok := authenticatedPrincipal(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	var request UpdateNameRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	if err := h.srv.UpdateName(
		ctx.Request.Context(),
		principal.AccessToken,
		request.NewName,
	); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "name updated successfully",
	})
}

func (h *UserHandlers) UpdatePassword(ctx *gin.Context) {
	const op = "handlers.user.UpdatePassword"
	log := h.log.With(slog.String("op", op))

	principal, ok := authenticatedPrincipal(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	var request UpdatePasswordRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	if err := h.srv.UpdatePassword(
		ctx.Request.Context(),
		principal.AccessToken,
		request.OldPassword,
		request.NewPassword,
	); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "password updated successfully",
	})
}

func (h *UserHandlers) GetUser(ctx *gin.Context) {
	const op = "handlers.user.GetUser"
	log := h.log.With(slog.String("op", op))

	principal, ok := authenticatedPrincipal(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	user, err := h.srv.GetUser(ctx.Request.Context(), principal.AccessToken)
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	if user == nil {
		err := customerrors.ErrInternal
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id": user.UserID,
			"email":   user.Email,
			"name":    user.Name,
			"role":    user.Role,
		},
	})
}

func authenticatedPrincipal(ctx *gin.Context) (gatewayauth.Principal, bool) {
	return gatewayauth.PrincipalFromContext(ctx)
}
