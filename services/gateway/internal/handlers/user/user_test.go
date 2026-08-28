package user

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type fakeUserService struct {
	getUserFn        func(context.Context, string) (*domain.User, error)
	updateEmailFn    func(context.Context, string, string) error
	updateNameFn     func(context.Context, string, string) error
	updatePasswordFn func(context.Context, string, string, string) error
}

func (f *fakeUserService) GetUser(ctx context.Context, token string) (*domain.User, error) {
	return f.getUserFn(ctx, token)
}

func (f *fakeUserService) UpdateEmail(ctx context.Context, token, email string) error {
	return f.updateEmailFn(ctx, token, email)
}

func (f *fakeUserService) UpdateName(ctx context.Context, token, name string) error {
	return f.updateNameFn(ctx, token, name)
}

func (f *fakeUserService) UpdatePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	return f.updatePasswordFn(ctx, token, oldPassword, newPassword)
}

func TestGetUserUsesPrincipalToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeUserService{
		getUserFn: func(_ context.Context, token string) (*domain.User, error) {
			require.Equal(t, "access-token", token)
			return &domain.User{UserID: 42, Email: "user@example.com", Name: "User", Role: "user"}, nil
		},
	}
	router := authenticatedUserRouter()
	router.GET("/me", New(userTestLogger(), service).GetUser)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"user_id":42`)
	require.Contains(t, recorder.Body.String(), `"email":"user@example.com"`)
}

func TestUpdateEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeUserService{
		updateEmailFn: func(_ context.Context, token, email string) error {
			require.Equal(t, "access-token", token)
			require.Equal(t, "new@example.com", email)
			return nil
		},
	}
	router := authenticatedUserRouter()
	router.PATCH("/me/email", New(userTestLogger(), service).UpdateEmail)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/me/email",
		bytes.NewBufferString(`{"new_email":"new@example.com"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdatePasswordRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serviceCalled := false
	service := &fakeUserService{
		updatePasswordFn: func(context.Context, string, string, string) error {
			serviceCalled = true
			return nil
		},
	}
	router := authenticatedUserRouter()
	router.PATCH("/me/password", New(userTestLogger(), service).UpdatePassword)

	request := httptest.NewRequest(
		http.MethodPatch,
		"/me/password",
		bytes.NewBufferString(`{"old_password":"old-password"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, serviceCalled)
}

func TestUserHandlerRequiresPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serviceCalled := false
	service := &fakeUserService{
		getUserFn: func(context.Context, string) (*domain.User, error) {
			serviceCalled = true
			return nil, nil
		},
	}
	router := gin.New()
	router.GET("/me", New(userTestLogger(), service).GetUser)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.False(t, serviceCalled)
}

func authenticatedUserRouter() *gin.Engine {
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity: domain.Identity{UserID: 42, Role: "user"}, AccessToken: "access-token",
		})
		ctx.Next()
	})
	return router
}

func userTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
