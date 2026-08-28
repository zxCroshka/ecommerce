package userservice

import (
	"strings"

	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ValidateTokenResponse(response *userservicev1.ValidateTokenResponse) error {
	const op = "grpc.UserClient.ValidateTokenResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	if response.GetUserId() <= 0 {
		return invalidResponse(op, "user_id must be positive")
	}
	if !validRole(response.GetRole()) {
		return invalidResponse(op, "role is invalid")
	}
	return nil
}

func ValidateTokenPairResponse(response *userservicev1.TokenPairResponse) error {
	const op = "grpc.UserClient.ValidateTokenPairResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	if strings.TrimSpace(response.GetAccessToken()) == "" {
		return invalidResponse(op, "access_token is empty")
	}
	if strings.TrimSpace(response.GetRefreshToken()) == "" {
		return invalidResponse(op, "refresh_token is empty")
	}
	if !strings.EqualFold(strings.TrimSpace(response.GetTokenType()), "Bearer") {
		return invalidResponse(op, "token_type must be Bearer")
	}
	if response.GetExpiresIn() <= 0 {
		return invalidResponse(op, "expires_in must be positive")
	}
	return nil
}

func ValidateGetUserResponse(response *userservicev1.GetUserResponse) error {
	const op = "grpc.UserClient.ValidateGetUserResponse"

	if response == nil {
		return invalidResponse(op, "response is nil")
	}
	if response.GetUserId() <= 0 {
		return invalidResponse(op, "user_id must be positive")
	}
	if strings.TrimSpace(response.GetEmail()) == "" {
		return invalidResponse(op, "email is empty")
	}
	if strings.TrimSpace(response.GetName()) == "" {
		return invalidResponse(op, "name is empty")
	}
	if !validRole(response.GetRole()) {
		return invalidResponse(op, "role is invalid")
	}
	return nil
}

func ValidateRegisterResponse(response *userservicev1.RegisterResponse) error {
	return validatePresent("grpc.UserClient.ValidateRegisterResponse", response != nil)
}

func ValidateLogoutResponse(response *userservicev1.LogoutResponse) error {
	return validatePresent("grpc.UserClient.ValidateLogoutResponse", response != nil)
}

func ValidateUpdateEmailResponse(response *userservicev1.UpdateEmailResponse) error {
	return validatePresent("grpc.UserClient.ValidateUpdateEmailResponse", response != nil)
}

func ValidateUpdateNameResponse(response *userservicev1.UpdateNameResponse) error {
	return validatePresent("grpc.UserClient.ValidateUpdateNameResponse", response != nil)
}

func ValidateUpdatePasswordResponse(response *userservicev1.UpdatePasswordResponse) error {
	return validatePresent("grpc.UserClient.ValidateUpdatePasswordResponse", response != nil)
}

func validatePresent(op string, present bool) error {
	if !present {
		return invalidResponse(op, "response is nil")
	}
	return nil
}

func invalidResponse(op, message string) error {
	return mappingErrors(op, status.Error(codes.Internal, "invalid user service response: "+message))
}

func validRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "customer", "admin":
		return true
	default:
		return false
	}
}
