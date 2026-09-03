package grpcapp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	notificationgrpc "github.com/zxCroshka/ecommerce/services/notification-service/internal/grpc"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	log      *slog.Logger
	server   *grpc.Server
	userConn *grpc.ClientConn
	port     int
}

func New(
	log *slog.Logger,
	notifications notificationgrpc.NotificationService,
	port int,
	userAddress string,
	authTimeout time.Duration,
	defaultListLimit, maxListLimit int,
) (*App, error) {
	userConn, err := grpc.NewClient(userAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create UserService gRPC client: %w", err)
	}
	authInterceptor := notificationgrpc.NewAuthInterceptor(userservicev1.NewUserClient(userConn), authTimeout)
	server := grpc.NewServer(grpc.UnaryInterceptor(authInterceptor.UnaryInterceptor()))
	notificationgrpc.Register(server, notifications, defaultListLimit, maxListLimit)
	return &App{log: log, server: server, userConn: userConn, port: port}, nil
}

func (a *App) Run() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("listen Notification gRPC: %w", err)
	}
	a.log.Info("Notification gRPC server is running", "address", listener.Addr().String())
	if err := a.server.Serve(listener); err != nil {
		return fmt.Errorf("serve Notification gRPC: %w", err)
	}
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		a.server.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		a.server.Stop()
		<-done
	}
	closeErr := a.userConn.Close()
	if ctx.Err() != nil {
		return fmt.Errorf("stop Notification gRPC: %w", ctx.Err())
	}
	if closeErr != nil {
		return fmt.Errorf("close UserService connection: %w", closeErr)
	}
	return nil
}
