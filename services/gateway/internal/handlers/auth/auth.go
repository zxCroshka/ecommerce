package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/logging"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/response"
)

type AuthService interface {
	Login(ctx context.Context, email, password string) (*domain.TokenPair, error)
	RefreshTokens(ctx context.Context, refreshToken string) (*domain.TokenPair, error)
	Logout(ctx context.Context, accessToken, refreshToken string) error
	Register(ctx context.Context, email, password, name string) error
}

type AuthHandlers struct {
	log *slog.Logger
	srv AuthService
}

func New(log *slog.Logger, srv AuthService) *AuthHandlers {
	if log == nil {
		log = slog.Default()
	}
	return &AuthHandlers{log: log, srv: srv}
}

func (h *AuthHandlers) Register(ctx *gin.Context) {
	const op = "handlers.auth.Register"
	log := h.log.With(slog.String("op", op))

	var request RegisterRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "anonymous user"
	}
	if err := h.srv.Register(
		ctx.Request.Context(),
		request.Email,
		request.Password,
		name,
	); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "user registered successfully",
	})
}

func (h *AuthHandlers) Login(ctx *gin.Context) {
	const op = "handlers.auth.Login"
	log := h.log.With(slog.String("op", op))

	var request LoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	tokenPair, err := h.srv.Login(ctx.Request.Context(), request.Email, request.Password)
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	if tokenPair == nil {
		err := customerrors.ErrInternal
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	writeTokenPair(ctx, tokenPair)
}

func (h *AuthHandlers) RefreshToken(ctx *gin.Context) {
	const op = "handlers.auth.RefreshToken"
	log := h.log.With(slog.String("op", op))

	var request RefreshTokenRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	tokenPair, err := h.srv.RefreshTokens(ctx.Request.Context(), request.RefreshToken)
	if err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}
	if tokenPair == nil {
		err := customerrors.ErrInternal
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	writeTokenPair(ctx, tokenPair)
}

func (h *AuthHandlers) Logout(ctx *gin.Context) {
	const op = "handlers.auth.Logout"
	log := h.log.With(slog.String("op", op))

	principal, ok := gatewayauth.PrincipalFromContext(ctx)
	if !ok {
		response.WriteError(ctx, customerrors.ErrUnauthenticated)
		return
	}

	var request LogoutRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Warn("failed to bind request", "error", err)
		response.WriteError(ctx, customerrors.ErrInvalidArgument)
		return
	}

	if err := h.srv.Logout(
		ctx.Request.Context(),
		principal.AccessToken,
		request.RefreshToken,
	); err != nil {
		logging.WriteLog(ctx, log, err)
		response.WriteError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "successfully logged out",
	})
}

func writeTokenPair(ctx *gin.Context, tokenPair *domain.TokenPair) {
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  tokenPair.AccessToken,
			"refresh_token": tokenPair.RefreshToken,
			"token_type":    tokenPair.TokenType,
			"expires_in":    int64(tokenPair.ExpiresIn / time.Second),
		},
	})
}
