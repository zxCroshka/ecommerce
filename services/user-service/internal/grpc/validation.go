package grpc

import (
	userservicrev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)
const (
	emptyValue = 0
)
func ValidateValidateToken(req *userservicrev1.ValidateTokenRequest) error{
	if len(req.GetToken()) == 0{
		return status.Error(codes.InvalidArgument,"empty token")
	}
	return nil
}

func ValidateGetUser(req *userservicrev1.GetUserRequest) error{
	if req.GetUserId() == emptyValue{
		return status.Error(codes.InvalidArgument,"userID is required")
	}
	return nil
}