package main

import (
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/bizmongolib/mongolib"
	userpassauthenticator "github.com/sbasestarter/bizmongolib/user/authenticator/userpass"
	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-be/internal/impls"
	"github.com/sbasestarter/customer-service-be/internal/server"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sbasestarter/userlib"
	memoryauthingdatastorage "github.com/sbasestarter/userlib/authingdatastorage/memory"
	userpassmanager "github.com/sbasestarter/userlib/manager/userpass"
	"github.com/sbasestarter/userlib/policy/single"
	memorystatuscontroller "github.com/sbasestarter/userlib/statuscontroller/memory"
	"github.com/sgostarter/libservicetoolset/servicetoolset"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

func main() {
	cfg := config.GetConfig()

	logger := cfg.Logger

	tlsConfig, err := servicetoolset.GRPCTlsConfigMap(cfg.GRPCTLSConfig)
	if err != nil {
		logger.Fatal(err)
	}

	grpcCfg := &servicetoolset.GRPCServerConfig{
		Address:           cfg.ServicerUserListen,
		TLSConfig:         tlsConfig,
		KeepAliveDuration: time.Minute * 10,
	}

	s, err := servicetoolset.NewGRPCServer(nil, grpcCfg,
		[]grpc.ServerOption{grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             time.Second * 10,
			PermitWithoutStream: true,
		})}, nil, logger)
	if err != nil {
		logger.Fatal(err)

		return
	}

	mongoCli, mongoOptions, err := mongolib.InitMongo(cfg.UserMongoDSN)
	if err != nil {
		logger.Fatal(err)

		return
	}

	servicerUserCenter := userlib.NewUserCenter(cfg.ServicerTokenSecret, single.NewPolicy(userinters.AuthMethodNameUserPassword),
		memorystatuscontroller.NewStatusController(), memoryauthingdatastorage.NewMemoryAuthingDataStorage(), logger)
	serviceUserPassModel := userpassauthenticator.NewMongoUserPasswordModel(mongoCli, mongoOptions.Auth.AuthSource, "servicer_users", logger)
	servicerManager := userpassmanager.NewManager(cfg.ServicerPasswordSecret, serviceUserPassModel)
	servicerUserTokenHelper := impls.NewLocalServicerUserTokenHelper(servicerUserCenter, servicerManager)

	grpcServicerUserServer := server.NewServicerUserServer(servicerManager, servicerUserCenter, servicerUserTokenHelper)

	err = s.Start(func(s *grpc.Server) error {
		customertalkpb.RegisterServicerUserServicerServer(s, grpcServicerUserServer)

		return nil
	})
	if err != nil {
		logger.Fatal(err)

		return
	}

	logger.Info("grpc server listen on: ", cfg.ServicerUserListen)
	s.Wait()
}
