package grpcapp

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	productgrpc "github.com/zxCroshka/ecommerce/services/product-service/internal/grpc"
	userservicev1 "github.com/zxCroshka/ecommerce/shared/userservice/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	log        *slog.Logger
	gRPCServer *grpc.Server
	userConn   *grpc.ClientConn
	port       int
}

func New(
	log *slog.Logger,
	productservice productgrpc.ProductService,
	port int,
	internalToken string,
	userServiceAddress string,
	defaultListLimit int,
	maxListLimit int,
) (*App, error) {
	userConn, err := grpc.NewClient(
		userServiceAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("create UserService gRPC client: %w", err)
	}

	userClient := userservicev1.NewUserClient(userConn)
	authInterceptor := productgrpc.NewAuthInterceptor(internalToken, userClient)
	gRPCServer := grpc.NewServer(
		grpc.UnaryInterceptor(authInterceptor.UnaryInterceptor()),
	)

	productgrpc.RegisterServerAPI(
		gRPCServer,
		productservice,
		defaultListLimit,
		maxListLimit,
	)
	return &App{
		log:        log,
		gRPCServer: gRPCServer,
		userConn:   userConn,
		port:       port,
	}, nil
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "grpcapp.Run"

	log := a.log.With(slog.String("op", op), slog.Int("port", a.port))
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	log.Info("gRPC server is running", slog.String("addr", l.Addr().String()))
	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil

}

func (a *App) Stop(ctx context.Context) error {
	const op = "grpcapp.Stop"
	a.log.With(slog.String("op", op)).Info("stopping gRPC server ", slog.Int("port", a.port))
	done := make(chan struct{})
	go func() {
		a.gRPCServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		a.gRPCServer.Stop()
		<-done
	}
	if err := a.userConn.Close(); err != nil {
		return fmt.Errorf("close UserService gRPC connection: %w", err)
	}
	return ctx.Err()
}
