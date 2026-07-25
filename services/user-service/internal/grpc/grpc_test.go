package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	userservicrev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) ValidateToken(ctx context.Context, token string) (int64, domain.Role, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(int64), args.Get(1).(domain.Role), args.Error(2)
}

func (m *MockUserService) GetUser(ctx context.Context, userID int64) (domain.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.User), args.Error(1)
}

// ==================== ТЕСТЫ ДЛЯ VALIDATE_TOKEN ====================

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name        string
		req         *userservicrev1.ValidateTokenRequest
		setupMock   func(*MockUserService)
		expectedRes *userservicrev1.ValidateTokenResponse
		expectedErr error
	}{
		{
			name: "success - valid token",
			req:  &userservicrev1.ValidateTokenRequest{Token: "valid-token"},
			setupMock: func(m *MockUserService) {
				m.On("ValidateToken", mock.Anything, "valid-token").
					Return(int64(123), domain.RoleAdmin, nil)
			},
			expectedRes: &userservicrev1.ValidateTokenResponse{UserId: 123, Role: "admin"},
			expectedErr: nil,
		},
		{
			name:        "error - empty token",
			req:         &userservicrev1.ValidateTokenRequest{Token: ""},
			setupMock:   func(m *MockUserService) {},
			expectedRes: nil,
			expectedErr: status.Error(codes.InvalidArgument, "empty token"),
		},
		{
			name: "error - blacklisted token",
			req:  &userservicrev1.ValidateTokenRequest{Token: "blacklisted"},
			setupMock: func(m *MockUserService) {
				m.On("ValidateToken", mock.Anything, "blacklisted").
					Return(int64(0), domain.Role(""), customerrors.ErrTokenBlacklisted)
			},
			expectedRes: nil,
			expectedErr: status.Error(codes.Unauthenticated, "token is blacklisted"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserService)
			tt.setupMock(mockService)

			server := &ServerAPI{usrservice: mockService}
			res, err := server.ValidateToken(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRes, res)
			}

			mockService.AssertExpectations(t)
		})
	}
}

// ==================== ТЕСТЫ ДЛЯ GET_USER ====================

func TestGetUser(t *testing.T) {
	tests := []struct {
		name        string
		req         *userservicrev1.GetUserRequest
		setupMock   func(*MockUserService)
		expectedRes *userservicrev1.GetUserResponse
		expectedErr error
	}{
		{
			name: "success - get user",
			req:  &userservicrev1.GetUserRequest{UserId: 123},
			setupMock: func(m *MockUserService) {
				m.On("GetUser", mock.Anything, int64(123)).
					Return(domain.User{
						Id:    123,
						Email: "test@example.com",
						Name:  "Test User",
						Role:  domain.RoleCustomer,
					}, nil)
			},
			expectedRes: &userservicrev1.GetUserResponse{
				Email: "test@example.com",
				Name:  "Test User",
				Role:  "customer",
			},
			expectedErr: nil,
		},
		{
			name: "error - user not found",
			req:  &userservicrev1.GetUserRequest{UserId: 999},
			setupMock: func(m *MockUserService) {
				m.On("GetUser", mock.Anything, int64(999)).
					Return(domain.User{}, customerrors.ErrUserNotFound)
			},
			expectedRes: nil,
			expectedErr: status.Error(codes.NotFound, "user not found"),
		},
		{
			name:        "error - empty user id",
			req:         &userservicrev1.GetUserRequest{UserId: 0},
			setupMock:   func(m *MockUserService) {},
			expectedRes: nil,
			expectedErr: status.Error(codes.InvalidArgument, "userID is required"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockUserService)
			tt.setupMock(mockService)

			server := &ServerAPI{usrservice: mockService}
			res, err := server.GetUser(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
				assert.Nil(t, res)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRes, res)
			}

			mockService.AssertExpectations(t)
		})
	}
}

// ==================== ТЕСТЫ ДЛЯ ВАЛИДАЦИИ ====================

func TestValidateValidateToken(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		shouldErr bool
	}{
		{"valid token", "some-token", false},
		{"empty token", "", true},
		{"whitespace", "   ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateValidateToken(&userservicrev1.ValidateTokenRequest{Token: tt.token})
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGetUser(t *testing.T) {
	tests := []struct {
		name      string
		userID    int64
		shouldErr bool
	}{
		{"valid id", 123, false},
		{"zero id", 0, true},
		{"negative id", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGetUser(&userservicrev1.GetUserRequest{UserId: tt.userID})
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ==================== БЕНЧМАРКИ (опционально) ====================

func BenchmarkValidateToken(b *testing.B) {
	mockService := new(MockUserService)
	mockService.On("ValidateToken", mock.Anything, "bench-token").
		Return(int64(123), false, nil)

	server := &ServerAPI{usrservice: mockService}
	req := &userservicrev1.ValidateTokenRequest{Token: "bench-token"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = server.ValidateToken(context.Background(), req)
	}
}
