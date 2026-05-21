// user/user.go
package user

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	errs "github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/err"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service"
)

type UserHandlers struct {
	log *slog.Logger
	srv service.UserServiceInterface
}

func New(log *slog.Logger, srv service.UserServiceInterface) *UserHandlers {
	return &UserHandlers{
		log: log,
		srv: srv,
	}
}

type UpdateEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

func (h *UserHandlers) UpdateEmail(ctx *gin.Context) {
	const op = "handlers.user.UpdateEmail"
	log := h.log.With(slog.String("op", op))

	token, exists := ctx.Get("access_token")
	if !exists {
		_ = ctx.Error(errs.NewUnauthorizedError("unauthorized"))
		ctx.Abort()
		return
	}

	var req UpdateEmailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid request body"))
		ctx.Abort()
		return
	}

	if err := h.srv.UpdateEmail(ctx, token.(string), req.NewEmail); err != nil {
		if errors.Is(err, customerrors.ErrDuplicateEmail) {
			log.Error("duplicate email", "email", req.NewEmail)
			_ = ctx.Error(errs.NewConflictError("email already exists"))
			ctx.Abort()
			return
		}
		if errors.Is(err, customerrors.ErrInvalidToken) {
			_ = ctx.Error(errs.NewUnauthorizedError("invalid token"))
			ctx.Abort()
			return
		}
		log.Error("failed to update email", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to update email"))
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "email updated successfully",
	})
}

type UpdateNameRequest struct {
	NewName string `json:"new_name" binding:"required,min=2,max=100"`
}

func (h *UserHandlers) UpdateName(ctx *gin.Context) {
	const op = "handlers.user.UpdateName"
	log := h.log.With(slog.String("op", op))

	token, exists := ctx.Get("access_token")
	if !exists {
		_ = ctx.Error(errs.NewUnauthorizedError("unauthorized"))
		ctx.Abort()
		return
	}

	var req UpdateNameRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid request body"))
		ctx.Abort()
		return
	}

	if err := h.srv.UpdateName(ctx, token.(string), req.NewName); err != nil {
		if errors.Is(err, customerrors.ErrInvalidToken) {
			_ = ctx.Error(errs.NewUnauthorizedError("invalid token"))
			ctx.Abort()
			return
		}
		log.Error("failed to update name", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to update name"))
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "name updated successfully",
	})
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

func (h *UserHandlers) UpdatePassword(ctx *gin.Context) {
	const op = "handlers.user.UpdatePassword"
	log := h.log.With(slog.String("op", op))

	token, exists := ctx.Get("access_token")
	if !exists {
		_ = ctx.Error(errs.NewUnauthorizedError("unauthorized"))
		ctx.Abort()
		return
	}

	var req UpdatePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid request body"))
		ctx.Abort()
		return
	}

	if err := h.srv.UpdatePassword(ctx, token.(string), req.OldPassword, req.NewPassword); err != nil {
		if errors.Is(err, customerrors.ErrInvalidCredentials) {
			_ = ctx.Error(errs.NewUnauthorizedError("invalid old password"))
			ctx.Abort()
			return
		}
		if errors.Is(err, customerrors.ErrInvalidToken) {
			_ = ctx.Error(errs.NewUnauthorizedError("invalid token"))
			ctx.Abort()
			return
		}
		log.Error("failed to update password", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to update password"))
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "password updated successfully",
	})
}

func (h *UserHandlers) GetProfile(ctx *gin.Context) {
	const op = "handlers.user.GetProfile"
	log := h.log.With(slog.String("op", op))

	UserID, exists := ctx.Get("user_id")
	if !exists {
		_ = ctx.Error(errs.NewUnauthorizedError("unauthorized"))
		ctx.Abort()
		return
	}

	user, err := h.srv.GetUser(ctx, UserID.(int64))
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidToken) {
			_ = ctx.Error(errs.NewUnauthorizedError("invalid token"))
			ctx.Abort()
			return
		}
		if errors.Is(err, customerrors.ErrUserNotFound) {
			_ = ctx.Error(errs.NewUnauthorizedError("user not found"))
			ctx.Abort()
			return
		}
		log.Error("failed to get user", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to get user profile"))
		ctx.Abort()
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":  user.Id,
			"email":    user.Email,
			"name":     user.Name,
			"is_admin": user.IsAdmin,
		},
	})
}
