package auth

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type fakeAuthService struct {
	loginFn    func(context.Context, string, string) (*domain.TokenPair, error)
	refreshFn  func(context.Context, string) (*domain.TokenPair, error)
	logoutFn   func(context.Context, string, string) error
	registerFn func(context.Context, string, string, string) error
}

func (f *fakeAuthService) Login(ctx context.Context, email, password string) (*domain.TokenPair, error) {
	return f.loginFn(ctx, email, password)
}

func (f *fakeAuthService) RefreshTokens(ctx context.Context, token string) (*domain.TokenPair, error) {
	return f.refreshFn(ctx, token)
}

func (f *fakeAuthService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	return f.logoutFn(ctx, accessToken, refreshToken)
}

func (f *fakeAuthService) Register(ctx context.Context, email, password, name string) error {
	return f.registerFn(ctx, email, password, name)
}

func TestLoginUsesTokenPairExpiration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeAuthService{
		loginFn: func(_ context.Context, email, password string) (*domain.TokenPair, error) {
			require.Equal(t, "user@example.com", email)
			require.Equal(t, "password", password)
			return &domain.TokenPair{
				AccessToken: "access", RefreshToken: "refresh", TokenType: "Bearer", ExpiresIn: 15 * time.Minute,
			}, nil
		},
	}
	router := gin.New()
	router.POST("/login", New(authTestLogger(), service).Login)

	request := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(`{
		"email":"user@example.com",
		"password":"password"
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"expires_in":900`)
	require.Contains(t, recorder.Body.String(), `"token_type":"Bearer"`)
}

func TestLogoutUsesPrincipalAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &fakeAuthService{
		logoutFn: func(_ context.Context, accessToken, refreshToken string) error {
			require.Equal(t, "access-token", accessToken)
			require.Equal(t, "refresh-token", refreshToken)
			return nil
		},
	}
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity: domain.Identity{UserID: 42, Role: "user"}, AccessToken: "access-token",
		})
		ctx.Next()
	})
	router.POST("/logout", New(authTestLogger(), service).Logout)

	request := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewBufferString(`{
		"refresh_token":"refresh-token"
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestRegisterRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serviceCalled := false
	service := &fakeAuthService{
		registerFn: func(context.Context, string, string, string) error {
			serviceCalled = true
			return nil
		},
	}
	router := gin.New()
	router.POST("/register", New(authTestLogger(), service).Register)

	request := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBufferString(`{"email":"invalid"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, serviceCalled)
}

func authTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
