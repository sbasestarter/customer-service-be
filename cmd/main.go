package main

import (
	"context"
	"time"

	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-be/internal/server"
	"github.com/sbasestarter/customer-service-be/internal/user"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/libservicetoolset/servicetoolset"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

func main() {
	cfg := config.GetConfig()

	logger := cfg.Logger

	tlsConfig, err := servicetoolset.GRPCTlsConfigMap(cfg.GRPCTLSConfig)
	if err != nil {
		logger.Fatal(err)
	}

	grpcCfg := &servicetoolset.GRPCServerConfig{
		Address:           cfg.Listen,
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

	userCenter := user.NewAnonymousCenter("1", time.Hour*24*31)
	ctxOut, _ := userCenter.Login(context.TODO(), "zjz")
	logger.Info(metadata.FromOutgoingContext(ctxOut))

	grpcServer := server.NewServer(userCenter, logger)

	err = s.Start(func(s *grpc.Server) error {
		customertalkpb.RegisterCustomerTalkServiceServer(s, grpcServer)
		customertalkpb.RegisterServiceTalkServiceServer(s, grpcServer)

		return nil
	})
	if err != nil {
		logger.Fatal(err)

		return
	}

	logger.Info("grpc server listen on: ", cfg.Listen)
	s.Wait()
}
