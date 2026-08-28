package middleware

import (
	"context"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/customerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/response"
)

const bearerScheme = "Bearer"

type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*domain.Identity, error)
}

type AuthMiddleware struct {
	log       *slog.Logger
	validator TokenValidator
}

func NewAuthMiddleware(log *slog.Logger, validator TokenValidator) *AuthMiddleware {
	if log == nil {
		log = slog.Default()
	}
	return &AuthMiddleware{
		log:       log,
		validator: validator,
	}
}

func (m *AuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		accessToken, ok := bearerToken(ctx.GetHeader("Authorization"))
		if !ok {
			response.WriteError(ctx, customerrors.ErrUnauthenticated)
			return
		}
		if m.validator == nil {
			m.log.Error("authentication validator is not configured")
			response.WriteError(ctx, customerrors.ErrInternal)
			return
		}

		identity, err := m.validator.ValidateToken(ctx.Request.Context(), accessToken)
		if err != nil {
			m.log.Warn("authentication failed", "error", err)
			response.WriteError(ctx, err)
			return
		}
		if identity == nil || identity.UserID <= 0 || strings.TrimSpace(identity.Role) == "" {
			m.log.Error("token validator returned an invalid identity")
			response.WriteError(ctx, customerrors.ErrInternal)
			return
		}

		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity:    *identity,
			AccessToken: accessToken,
		})
		ctx.Next()
	}
}

func (m *AuthMiddleware) RequireRole(allowedRoles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedRoles))
	for _, role := range allowedRoles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role != "" {
			allowed[role] = struct{}{}
		}
	}

	return func(ctx *gin.Context) {
		principal, ok := gatewayauth.PrincipalFromContext(ctx)
		if !ok {
			response.WriteError(ctx, customerrors.ErrUnauthenticated)
			return
		}
		if len(allowed) == 0 {
			m.log.Error("role middleware has no allowed roles")
			response.WriteError(ctx, customerrors.ErrInternal)
			return
		}

		role := strings.ToLower(strings.TrimSpace(principal.Identity.Role))
		if _, ok := allowed[role]; !ok {
			response.WriteError(ctx, customerrors.ErrPermissionDenied)
			return
		}
		ctx.Next()
	}
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], bearerScheme) {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	return token, token != ""
}
