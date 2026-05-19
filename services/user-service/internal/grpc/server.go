package grpc

import (
	"context"
	"errors"

	"github.com/zxCroshka/ecommerce/services/user-service/internal/domain"
	customerrors "github.com/zxCroshka/ecommerce/services/user-service/internal/repository/custom_errors"
	userservicrev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserService interface {
	ValidateToken(ctx context.Context, token string) (int64, bool, error)
	GetUser(ctx context.Context, userID int64) (domain.User, error)
}

type ServerAPI struct {
	userservicrev1.UnimplementedUserServer
	usrservice UserService
}

func RegisterServerAPI(gRPC *grpc.Server, usrservice UserService) {
	userservicrev1.RegisterUserServer(gRPC, &ServerAPI{usrservice: usrservice})
}

//--------------------------------------------

func (s *ServerAPI) ValidateToken(ctx context.Context, req *userservicrev1.ValidateTokenRequest) (res *userservicrev1.ValidateTokenResponse, err error) {
	if err := ValidateValidateToken(req); err != nil {
		return nil, err
	}
	userID, isAdmin, err := s.usrservice.ValidateToken(ctx, req.GetToken())
	if err != nil {
		if errors.Is(err, customerrors.ErrTokenBlacklisted) {
			return nil, status.Error(codes.PermissionDenied, "token is black listed")
		}
		return nil, status.Error(codes.Internal, "internal server error")
	}
	return &userservicrev1.ValidateTokenResponse{
		UserId:  userID,
		IsAdmin: isAdmin,
	}, nil
}

func (s *ServerAPI) GetUser(ctx context.Context, req *userservicrev1.GetUserRequest) (res *userservicrev1.GetUserResponse, err error) {
	if err := ValidateGetUser(req); err != nil {
		return nil, err
	}
	user, err := s.usrservice.GetUser(ctx,req.GetUserId())
	if err != nil{
		if errors.Is(err,customerrors.ErrUserNotFound){
			return nil,status.Error(codes.NotFound,"user not found")
		}
		return nil,status.Error(codes.Internal,"internal server error")
	}

	return &userservicrev1.GetUserResponse{
		Email: user.Email,
		IsAdmin: user.IsAdmin,
		Name: user.Name,
	},nil

}
