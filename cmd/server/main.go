package main

import (
	"context"
	"os"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	"github.com/go-tangra/go-tangra-common/service"
	"github.com/go-tangra/go-tangra-sharing/cmd/server/assets"
	"github.com/go-tangra/go-tangra-sharing/internal/registration"
)

var (
	moduleID    = "sharing"
	moduleName  = "Sharing"
	version     = "1.0.0"
	description = "Share secrets and documents via one-time email links"
)

var globalRegClient *registration.Client

func newApp(
	ctx *bootstrap.Context,
	gs *grpc.Server,
	hs *kratosHttp.Server,
) *kratos.App {
	adminEndpoint := getEnvOrDefault("ADMIN_GRPC_ENDPOINT", "")

	grpcAddr := getEnvOrDefault("GRPC_ADVERTISE_ADDR", "")
	if grpcAddr == "" {
		grpcAddr = "0.0.0.0:9600"
		cfg := ctx.GetConfig()
		if cfg.Server != nil && cfg.Server.Grpc != nil && cfg.Server.Grpc.Addr != "" {
			grpcAddr = cfg.Server.Grpc.Addr
		}
	}

	logger := ctx.GetLogger()
	logHelper := log.NewHelper(logger)

	if adminEndpoint != "" {
		logHelper.Infof("Will register with admin gateway at: %s", adminEndpoint)

		go func() {
			time.Sleep(3 * time.Second)

			regConfig := &registration.Config{
				ModuleID:          moduleID,
				ModuleName:        moduleName,
				Version:           version,
				Description:       description,
				GRPCEndpoint:      grpcAddr,
				AdminEndpoint:     adminEndpoint,
				OpenapiSpec:       assets.OpenApiData,
				ProtoDescriptor:   assets.DescriptorData,
				MenusYaml:         assets.MenusData,
				HeartbeatInterval: 30 * time.Second,
				RetryInterval:     5 * time.Second,
				MaxRetries:        60,
			}

			regClient, err := registration.NewClient(logger, regConfig)
			if err != nil {
				logHelper.Warnf("Failed to create registration client: %v", err)
				return
			}
			globalRegClient = regClient

			regCtx := context.Background()
			if err := regClient.Register(regCtx); err != nil {
				logHelper.Errorf("Failed to register with admin gateway: %v", err)
				return
			}

			go regClient.StartHeartbeat(regCtx)
		}()
	} else {
		logHelper.Info("ADMIN_GRPC_ENDPOINT not set, skipping module registration")
	}

	return bootstrap.NewApp(ctx, gs, hs)
}

func stopRegistration() {
	if globalRegClient != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := globalRegClient.Unregister(shutdownCtx); err != nil {
			log.Warnf("Failed to unregister from admin gateway: %v", err)
		}
		_ = globalRegClient.Close()
	}
}

func runApp() error {
	ctx := bootstrap.NewContext(
		context.Background(),
		&conf.AppInfo{
			Project: service.Project,
			AppId:   "sharing.service",
			Version: version,
		},
	)

	defer stopRegistration()

	return bootstrap.RunApp(ctx, initApp)
}

func main() {
	if err := runApp(); err != nil {
		panic(err)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
