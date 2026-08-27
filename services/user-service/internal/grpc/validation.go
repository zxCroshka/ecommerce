package grpc

import (
	"net/mail"
	"strings"

	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateValidateToken(req *userservicev1.ValidateTokenRequest) error {
	if req == nil || strings.TrimSpace(req.GetToken()) == "" {
		return status.Error(codes.InvalidArgument, "token is required")
	}
	return nil
}

func ValidateRegister(req *userservicev1.RegisterRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if !validEmail(req.GetEmail()) {
		return status.Error(codes.InvalidArgument, "valid email is required")
	}
	if len(req.GetPassword()) < 8 {
		return status.Error(codes.InvalidArgument, "password must contain at least 8 characters")
	}
	return nil
}

func ValidateLogin(req *userservicev1.LoginRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if !validEmail(req.GetEmail()) {
		return status.Error(codes.InvalidArgument, "valid email is required")
	}
	if req.GetPassword() == "" {
		return status.Error(codes.InvalidArgument, "password is required")
	}
	return nil
}

func ValidateRefreshTokens(req *userservicev1.RefreshTokensRequest) error {
	if req == nil || strings.TrimSpace(req.GetRefreshToken()) == "" {
		return status.Error(codes.InvalidArgument, "refresh token is required")
	}
	return nil
}

func ValidateLogout(req *userservicev1.LogoutRequest) error {
	if req == nil || strings.TrimSpace(req.GetRefreshToken()) == "" {
		return status.Error(codes.InvalidArgument, "refresh token is required")
	}
	return nil
}

func ValidateUpdateEmail(req *userservicev1.UpdateEmailRequest) error {
	if req == nil || !validEmail(req.GetNewEmail()) {
		return status.Error(codes.InvalidArgument, "valid new email is required")
	}
	return nil
}

func ValidateUpdateName(req *userservicev1.UpdateNameRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	name := strings.TrimSpace(req.GetNewName())
	if len(name) < 2 || len(name) > 100 {
		return status.Error(codes.InvalidArgument, "name length must be between 2 and 100 characters")
	}
	return nil
}

func ValidateUpdatePassword(req *userservicev1.UpdatePasswordRequest) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	if req.GetOldPassword() == "" {
		return status.Error(codes.InvalidArgument, "old password is required")
	}
	if len(req.GetNewPassword()) < 8 {
		return status.Error(codes.InvalidArgument, "new password must contain at least 8 characters")
	}
	if req.GetOldPassword() == req.GetNewPassword() {
		return status.Error(codes.InvalidArgument, "new password must differ from old password")
	}
	return nil
}

func validEmail(value string) bool {
	value = strings.TrimSpace(value)
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}
