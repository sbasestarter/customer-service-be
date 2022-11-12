package server

import (
	"context"
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sbasestarter/userlib/authenticator/userpass"
	userpassmanager "github.com/sbasestarter/userlib/manager/userpass"
	"google.golang.org/grpc/codes"
)

func NewServicerUserServer(userManager userpassmanager.Manager, user userinters.UserCenter, tokenHelper defs.UserTokenHelper) customertalkpb.ServicerUserServicerServer {
	return &servicerUserServerImpl{
		userManager: userManager,
		user:        user,
		tokenHelper: tokenHelper,
	}
}

type servicerUserServerImpl struct {
	customertalkpb.UnimplementedServicerUserServicerServer

	userManager userpassmanager.Manager
	user        userinters.UserCenter
	tokenHelper defs.UserTokenHelper
}

func (impl *servicerUserServerImpl) Register(ctx context.Context, request *customertalkpb.RegisterRequest) (*customertalkpb.RegisterResponse, error) {
	token, code, err := impl.register(ctx, request)
	if code != codes.OK {
		return nil, gRpcError(code, err)
	}

	return &customertalkpb.RegisterResponse{
		Token:    token,
		UserName: request.GetUserName(),
	}, nil
}

func (impl *servicerUserServerImpl) Login(ctx context.Context, request *customertalkpb.LoginRequest) (*customertalkpb.LoginResponse, error) {
	if request == nil || request.GetUserName() == "" || request.GetPassword() == "" {
		return nil, gRpcMessageError(codes.InvalidArgument, "")
	}

	token, code, err := impl.login(ctx, request.GetUserName(), request.GetPassword())
	if code != codes.OK {
		return nil, gRpcError(code, err)
	}

	return &customertalkpb.LoginResponse{
		Token:    token,
		UserName: request.GetUserName(),
	}, nil
}

//
//
//

func (impl *servicerUserServerImpl) register(ctx context.Context, request *customertalkpb.RegisterRequest) (
	token string, code codes.Code, err error) {
	if request == nil || request.GetUserName() == "" || request.GetPassword() == "" {
		code = codes.InvalidArgument

		return
	}

	_, err = impl.userManager.Register(ctx, request.GetUserName(), request.GetPassword())
	if err != nil {
		code = codes.Internal

		return
	}

	token, code, err = impl.login(ctx, request.GetUserName(), request.GetPassword())
	if code != codes.OK {
		return
	}

	return
}

func (impl *servicerUserServerImpl) login(ctx context.Context, userName, password string) (
	token string, code codes.Code, err error) {
	authenticator, err := userpass.NewAuthenticator(userName, password, impl.userManager)
	if err != nil {
		code = codes.Internal

		return
	}

	resp, err := impl.user.Login(ctx, &userinters.LoginRequest{
		ContinueID: 0,
		Authenticators: []userinters.Authenticator{
			authenticator,
		},
		TokenLiveDuration: time.Hour * 24 * 7,
	})
	if err != nil {
		code = codes.Internal

		return
	}

	if resp.Status != userinters.LoginStatusSuccess {
		code = codes.Unauthenticated

		return
	}

	token = resp.Token
	code = codes.OK

	return
}
