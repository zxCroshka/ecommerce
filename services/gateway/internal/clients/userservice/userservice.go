package userservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/clients/grpcerrors"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/domain"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const authorizationMetadataKey = "authorization"

type UserClient struct {
	api  userservicev1.UserClient
	conn *grpc.ClientConn
}

type ClientConfig struct {
	Address    string
	RetryCount int
	Timeout    time.Duration
}

func New(cfg ClientConfig) (*UserClient, error) {
	const op = "grpc.UserClient.New"
	if strings.TrimSpace(cfg.Address) == "" {
		return nil, fmt.Errorf("%s: address is required", op)
	}
	if cfg.RetryCount < 0 {
		return nil, fmt.Errorf("%s: retry count cannot be negative", op)
	}
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("%s: timeout must be positive", op)
	}

	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.Unavailable),
		grpcretry.WithMax(uint(cfg.RetryCount)),
		grpcretry.WithPerRetryTimeout(cfg.Timeout),
	}
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOpts...)),
	}

	conn, err := grpc.NewClient(cfg.Address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &UserClient{
		api:  userservicev1.NewUserClient(conn),
		conn: conn,
	}, nil
}

func (c *UserClient) Close() error {
	const op = "grpc.UserClient.Close"

	if c == nil || c.conn == nil {
		return nil
	}
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *UserClient) ValidateToken(ctx context.Context, token string) (*domain.Identity, error) {
	const op = "grpc.UserClient.ValidateToken"

	response, err := c.api.ValidateToken(ctx, &userservicev1.ValidateTokenRequest{Token: token})
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateTokenResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &domain.Identity{
		UserID: response.GetUserId(),
		Role:   response.GetRole(),
	}, nil
}

func (c *UserClient) Register(ctx context.Context, email, password, name string) error {
	const op = "grpc.UserClient.Register"

	response, err := c.api.Register(ctx, &userservicev1.RegisterRequest{
		Email:    email,
		Password: password,
		Name:     name,
	}, grpcretry.Disable())
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateRegisterResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *UserClient) Login(ctx context.Context, email, password string) (*domain.TokenPair, error) {
	const op = "grpc.UserClient.Login"

	response, err := c.api.Login(ctx, &userservicev1.LoginRequest{
		Email:    email,
		Password: password,
	}, grpcretry.Disable())
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateTokenPairResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return tokenPairFromResponse(response), nil
}

func (c *UserClient) RefreshTokens(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	const op = "grpc.UserClient.RefreshTokens"

	response, err := c.api.RefreshTokens(ctx, &userservicev1.RefreshTokensRequest{
		RefreshToken: refreshToken,
	}, grpcretry.Disable())
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateTokenPairResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return tokenPairFromResponse(response), nil
}

func (c *UserClient) Logout(ctx context.Context, accessToken, refreshToken string) error {
	const op = "grpc.UserClient.Logout"

	response, err := c.api.Logout(withBearerToken(ctx, accessToken), &userservicev1.LogoutRequest{
		RefreshToken: refreshToken,
	}, grpcretry.Disable())
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateLogoutResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *UserClient) GetUser(ctx context.Context, accessToken string) (*domain.User, error) {
	const op = "grpc.UserClient.GetUser"

	response, err := c.api.GetUser(withBearerToken(ctx, accessToken), &userservicev1.GetUserRequest{})
	if err != nil {
		return nil, mappingErrors(op, err)
	}
	if err := ValidateGetUserResponse(response); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &domain.User{
		Email:  response.GetEmail(),
		Name:   response.GetName(),
		Role:   response.GetRole(),
		UserID: response.GetUserId(),
	}, nil
}

func (c *UserClient) UpdateEmail(ctx context.Context, accessToken, newEmail string) error {
	const op = "grpc.UserClient.UpdateEmail"

	response, err := c.api.UpdateEmail(withBearerToken(ctx, accessToken), &userservicev1.UpdateEmailRequest{
		NewEmail: newEmail,
	}, grpcretry.Disable())
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateUpdateEmailResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *UserClient) UpdateName(ctx context.Context, accessToken, newName string) error {
	const op = "grpc.UserClient.UpdateName"

	response, err := c.api.UpdateName(withBearerToken(ctx, accessToken), &userservicev1.UpdateNameRequest{
		NewName: newName,
	}, grpcretry.Disable())
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateUpdateNameResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (c *UserClient) UpdatePassword(ctx context.Context, accessToken, oldPassword, newPassword string) error {
	const op = "grpc.UserClient.UpdatePassword"

	response, err := c.api.UpdatePassword(withBearerToken(ctx, accessToken), &userservicev1.UpdatePasswordRequest{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}, grpcretry.Disable())
	if err != nil {
		return mappingErrors(op, err)
	}
	if err := ValidateUpdatePasswordResponse(response); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func withBearerToken(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(
		ctx,
		authorizationMetadataKey,
		"Bearer "+strings.TrimSpace(token),
	)
}

func tokenPairFromResponse(response *userservicev1.TokenPairResponse) *domain.TokenPair {
	return &domain.TokenPair{
		AccessToken:  response.GetAccessToken(),
		RefreshToken: response.GetRefreshToken(),
		TokenType:    response.GetTokenType(),
		ExpiresIn:    time.Duration(response.GetExpiresIn()) * time.Second,
	}
}

func mappingErrors(op string, err error) error {
	return grpcerrors.Map(op, err)
}
