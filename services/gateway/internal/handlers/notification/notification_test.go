package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	gatewayauth "github.com/zxCroshka/ecommerce/services/gateway/internal/auth"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
)

type serviceStub struct {
	list func(context.Context, string, int32, int32) (*domain.NotificationList, error)
	mark func(context.Context, string, int64) (*domain.Notification, error)
}

func (s *serviceStub) ListNotifications(ctx context.Context, token string, limit, offset int32) (*domain.NotificationList, error) {
	return s.list(ctx, token, limit, offset)
}

func (s *serviceStub) MarkAsRead(ctx context.Context, token string, id int64) (*domain.Notification, error) {
	return s.mark(ctx, token, id)
}

func notificationRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		gatewayauth.SetPrincipal(ctx, gatewayauth.Principal{
			Identity: domain.Identity{UserID: 42, Role: "customer"}, AccessToken: "access-token",
		})
		ctx.Next()
	})
	return router
}

func TestListNotificationsForwardsBearerAndPagination(t *testing.T) {
	now := time.Now().UTC()
	service := &serviceStub{list: func(_ context.Context, token string, limit, offset int32) (*domain.NotificationList, error) {
		require.Equal(t, "access-token", token)
		require.Equal(t, int32(10), limit)
		require.Equal(t, int32(20), offset)
		return &domain.NotificationList{Notifications: []*domain.Notification{{ID: 1, Type: "order_created", Title: "Заказ", Body: "Готов", CreatedAt: now}}, Total: 21, Limit: limit, Offset: offset}, nil
	}}
	router := notificationRouter()
	router.GET("/notifications", New(service).ListNotifications)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/notifications?page=3&page_size=10", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"total_pages":3`)
}

func TestMarkAsReadUsesPathID(t *testing.T) {
	now := time.Now().UTC()
	service := &serviceStub{mark: func(_ context.Context, token string, id int64) (*domain.Notification, error) {
		require.Equal(t, "access-token", token)
		require.Equal(t, int64(17), id)
		return &domain.Notification{ID: id, Type: "order_created", Title: "Заказ", Body: "Готов", CreatedAt: now, ReadAt: &now}, nil
	}}
	router := notificationRouter()
	router.PATCH("/notifications/:id/read", New(service).MarkAsRead)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, "/notifications/17/read", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"id":17`)
}
