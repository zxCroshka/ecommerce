package app

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/app/grpcapp"
	kaf "github.com/zxCroshka/ecommerce/services/user-service/internal/kafka"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/lib/jwt"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/repository/redis"
	"github.com/zxCroshka/ecommerce/services/user-service/internal/service"
)

type App struct {
	GRPCSrv   *grpcapp.App
	withPprof bool
}

func New(
	ctx context.Context,
	log *slog.Logger,
	grpcPort int,
	producer *kaf.Producer,
	storageURL string,
	accessTTL time.Duration,
	refreshTTl time.Duration,
	redisHost string,
	redisPort int,
	redisPassword string,
	redisDB int,
	jwtSecret string,
	pprofEnabled bool,
) *App {
	storage, err := repository.New(ctx, storageURL)
	if err != nil {
		panic(err)
	}
	cfg := redis.Config{
		Host:     redisHost,
		Port:     redisPort,
		Password: redisPassword,
		DB:       redisDB,
	}
	redisClient, err := redis.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	jwtmanager := jwt.NewJWTService(jwtSecret, accessTTL, refreshTTl)
	userService := service.NewUserService(log, storage, redisClient, producer, jwtmanager)
	grpcApp := grpcapp.New(log, userService, grpcPort)
	return &App{
		GRPCSrv:   grpcApp,
		withPprof: pprofEnabled,
	}

}

func (a *App) Start(ctx context.Context) {

	if a.withPprof {
		go func() {
			ServePProf(ctx)
		}()
	}
	go a.GRPCSrv.MustRun()
	<-ctx.Done()
	a.GRPCSrv.Stop()

}

func ServePProf(ctx context.Context) {
	engine := gin.New()
	SetupPProfHandlers(engine)
	srv := &http.Server{
		Addr:    "0.0.0.0:3366",
		Handler: engine,
	}
	go func() {
		<-ctx.Done()
		shutDownctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()
		if err := srv.Shutdown(shutDownctx); err != nil {
			log.Printf("pprof shutdown err:%v", err)
		}
	}()
	log.Println("start pprof on :3366")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("pprof server error: %v", err)
	}

}

func pprofHandler(name string) gin.HandlerFunc {
	handler := pprof.Handler(name)
	return func(ctx *gin.Context) {
		handler.ServeHTTP(ctx.Writer, ctx.Request)
	}
}

func SetupPProfHandlers(engine *gin.Engine) {

	engine.GET("/debug/pprof/", func(c *gin.Context) {
		pprof.Index(c.Writer, c.Request)
	})

	engine.GET("/debug/pprof/cmdline", func(ctx *gin.Context) {
		pprof.Cmdline(ctx.Writer, ctx.Request)
	})
	engine.GET("/debug/pprof/profile", func(ctx *gin.Context) {
		pprof.Profile(ctx.Writer, ctx.Request)
	})
	engine.Any("/debug/pprof/symbol", func(ctx *gin.Context) {
		pprof.Symbol(ctx.Writer, ctx.Request)
	})
	engine.GET("/debug/pprof/trace", func(ctx *gin.Context) {
		pprof.Trace(ctx.Writer, ctx.Request)
	})
	engine.GET("/debug/pprof/block", pprofHandler("block"))
	engine.GET("/debug/pprof/goroutine", pprofHandler("goroutine"))
	engine.GET("/debug/pprof/heap", pprofHandler("heap"))
	engine.GET("/debug/pprof/threadcreate", pprofHandler("threadcreate"))
}
