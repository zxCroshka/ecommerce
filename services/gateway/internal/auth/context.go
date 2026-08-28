package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

const principalContextKey = "auth.principal"

type Principal struct {
	Identity    domain.Identity
	AccessToken string
}

func SetPrincipal(ctx *gin.Context, principal Principal) {
	ctx.Set(principalContextKey, principal)
}

func PrincipalFromContext(ctx *gin.Context) (Principal, bool) {
	value, exists := ctx.Get(principalContextKey)
	if !exists {
		return Principal{}, false
	}

	principal, ok := value.(Principal)
	if !ok || principal.Identity.UserID <= 0 || principal.Identity.Role == "" {
		return Principal{}, false
	}
	return principal, true
}
