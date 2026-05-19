// auth/auth.go
package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	errs "github.com/zxCroshka/ecommerce/services/user-service/internal/handlers/err"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service"
)

type AuthHandlers struct {
	log *slog.Logger
	srv *service.UserService
}

func New(log *slog.Logger, srv *service.UserService) *AuthHandlers {
	return &AuthHandlers{
		log: log,
		srv: srv,
	}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name" binding:"omitempty"`
}

func (h *AuthHandlers) Register(ctx *gin.Context) {
	const op = "handlers.auth.Register"
	log := h.log.With(slog.String("op", op))

	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid request body"))
		return
	}

	name := req.Name
	if name == "" {
		name = "anonymous user"
	}

	if err := h.srv.Register(ctx, req.Email, req.Password, name, false); err != nil {
		if errors.Is(err, customerrors.ErrDuplicateEmail) {
			log.Error("duplicate email error", "email", req.Email)
			_ = ctx.Error(errs.NewConflictError("user with this email already exists"))
			return
		}
		log.Error("internal server error", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to register user"))
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "user registered successfully",
	})
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandlers) Login(ctx *gin.Context) {
	const op = "handlers.auth.Login"
	log := h.log.With(slog.String("op", op))

	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("invalid request body"))
		return
	}

	tokenPair, err := h.srv.Login(ctx, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidCredentials) {
			log.Error("invalid credentials", "email", req.Email)
			_ = ctx.Error(errs.NewUnauthorizedError("invalid email or password"))
			return
		}
		log.Error("internal server error", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to login"))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"token_type":    "Bearer",
			"expires_in":    900, // 15 минут в секундах
		},
	})
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandlers) RefreshToken(ctx *gin.Context) {
	const op = "handlers.auth.RefreshToken"
	log := h.log.With(slog.String("op", op))

	var req RefreshTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("refresh_token is required"))
		return
	}

	tokenPair, err := h.srv.RefreshTokens(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidToken) {
			log.Error("invalid refresh token")
			_ = ctx.Error(errs.NewUnauthorizedError("invalid or expired refresh token"))
			return
		}
		log.Error("internal server error", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to refresh tokens"))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"token_type":    "Bearer",
			"expires_in":    900,
		},
	})
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *AuthHandlers) Logout(ctx *gin.Context) {
	const op = "handlers.auth.Logout"
	log := h.log.With(slog.String("op", op))

	// Получаем access token из заголовка
	accessToken := ctx.GetHeader("Authorization")
	if accessToken == "" {
		_ = ctx.Error(errs.NewBadRequestError("authorization header is required"))
		return
	}

	// Убираем "Bearer " если есть
	if len(accessToken) > 7 && accessToken[:7] == "Bearer " {
		accessToken = accessToken[7:]
	}

	var req LogoutRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("failed to bind request", "error", err)
		_ = ctx.Error(errs.NewBadRequestError("refresh_token is required"))
		return
	}

	if err := h.srv.Logout(ctx, accessToken, req.RefreshToken); err != nil {
		if errors.Is(err, customerrors.ErrInvalidToken) {
			log.Error("invalid token during logout")
			_ = ctx.Error(errs.NewUnauthorizedError("invalid token"))
			return
		}
		log.Error("internal server error", "error", err)
		_ = ctx.Error(errs.NewInternalServerError("failed to logout"))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "successfully logged out",
	})
}
