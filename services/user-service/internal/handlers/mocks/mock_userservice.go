package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
)

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) Register(ctx context.Context, email, password, name string) error {
	args := m.Called(ctx, email, password, name)
	return args.Error(0)
}

func (m *MockUserService) Login(ctx context.Context, email, password string) (*jwt.TokenPair, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenPair), args.Error(1)
}

func (m *MockUserService) RefreshTokens(ctx context.Context, refreshToken string) (*jwt.TokenPair, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenPair), args.Error(1)
}

func (m *MockUserService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	args := m.Called(ctx, accessToken, refreshToken)
	return args.Error(0)
}

func (m *MockUserService) UpdateEmail(ctx context.Context, token, newEmail string) error {
	args := m.Called(ctx, token, newEmail)
	return args.Error(0)
}

func (m *MockUserService) UpdateName(ctx context.Context, token, newName string) error {
	args := m.Called(ctx, token, newName)
	return args.Error(0)
}

func (m *MockUserService) UpdatePassword(ctx context.Context, token, oldPassword, newPassword string) error {
	args := m.Called(ctx, token, oldPassword, newPassword)
	return args.Error(0)
}

func (m *MockUserService) GetUser(ctx context.Context, userID int64) (domain.User, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(domain.User), args.Error(1)
}

func (m *MockUserService) ValidateToken(ctx context.Context, token string) (int64, domain.Role, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(int64), args.Get(1).(domain.Role), args.Error(2)
}
