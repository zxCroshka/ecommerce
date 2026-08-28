package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	authhandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/auth"
	carthandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/cart"
	producthandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/product"
	userhandlers "github.com/zxCroshka/ecommerce/services/gateway/internal/handlers/user"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/middleware"
)

type staticTokenValidator struct {
	identity *domain.Identity
	err      error
}

func (v *staticTokenValidator) ValidateToken(context.Context, string) (*domain.Identity, error) {
	return v.identity, v.err
}

func TestRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newTestRouter(&domain.Identity{UserID: 42, Role: "admin"})

	expected := map[string]struct{}{
		"GET /healthz":                          {},
		"POST /api/v1/auth/register":            {},
		"POST /api/v1/auth/login":               {},
		"POST /api/v1/auth/refresh":             {},
		"POST /api/v1/auth/logout":              {},
		"GET /api/v1/users/me":                  {},
		"PATCH /api/v1/users/me/email":          {},
		"PATCH /api/v1/users/me/name":           {},
		"PATCH /api/v1/users/me/password":       {},
		"GET /api/v1/products":                  {},
		"GET /api/v1/products/:id":              {},
		"POST /api/v1/products":                 {},
		"PATCH /api/v1/products/:id":            {},
		"DELETE /api/v1/products/:id":           {},
		"GET /api/v1/cart":                      {},
		"POST /api/v1/cart/items":               {},
		"PATCH /api/v1/cart/items/:product_id":  {},
		"DELETE /api/v1/cart/items/:product_id": {},
	}

	actual := make(map[string]struct{})
	for _, route := range router.GetEngine().Routes() {
		actual[route.Method+" "+route.Path] = struct{}{}
	}
	require.Equal(t, expected, actual)
}

func TestSetupRoutesIsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newTestRouter(&domain.Identity{UserID: 42, Role: "admin"})
	before := len(router.GetEngine().Routes())

	require.NotPanics(t, router.SetupRoutes)
	require.Equal(t, before, len(router.GetEngine().Routes()))
}

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newTestRouter(&domain.Identity{UserID: 42, Role: "admin"})
	recorder := httptest.NewRecorder()

	router.GetEngine().ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/healthz", nil),
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}

func TestProductMutationRequiresAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newTestRouter(&domain.Identity{UserID: 42, Role: "user"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/products", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	recorder := httptest.NewRecorder()

	router.GetEngine().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":"PERMISSION_DENIED"`)
}

func TestCartRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := newTestRouter(&domain.Identity{UserID: 42, Role: "user"})
	recorder := httptest.NewRecorder()

	router.GetEngine().ServeHTTP(
		recorder,
		httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil),
	)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"code":"UNAUTHENTICATED"`)
}

func newTestRouter(identity *domain.Identity) *Router {
	return NewRouter(
		nil,
		authhandlers.New(nil, nil),
		userhandlers.New(nil, nil),
		middleware.NewAuthMiddleware(nil, &staticTokenValidator{identity: identity}),
		producthandlers.New(nil, nil),
		carthandlers.New(nil, nil),
	)
}
