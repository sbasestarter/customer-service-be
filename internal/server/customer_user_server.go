package server

import (
	"context"
	"time"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	anonymousauthenticator "github.com/sbasestarter/userlib/authenticator/anonymous"
	"github.com/sgostarter/libeasygo/commerr"
	"google.golang.org/grpc/codes"
)

func NewCustomerUserServer(user userinters.UserCenter, tokenHelper defs.UserTokenHelper) customertalkpb.CustomerUserServicerServer {
	return &customerUserServerImpl{
		user:        user,
		tokenHelper: tokenHelper,
	}
}

type customerUserServerImpl struct {
	customertalkpb.UnimplementedCustomerUserServicerServer

	user        userinters.UserCenter
	tokenHelper defs.UserTokenHelper
}

func (impl *customerUserServerImpl) CheckToken(ctx context.Context, request *customertalkpb.CheckTokenRequest) (*customertalkpb.CheckTokenResponse, error) {
	newToken, userName, err := impl.checkToken(ctx)
	if err != nil {
		return &customertalkpb.CheckTokenResponse{
			Valid: false,
		}, nil
	}

	return &customertalkpb.CheckTokenResponse{
		Valid:    true,
		UserName: userName,
		NewToken: newToken,
	}, nil
}

func (impl *customerUserServerImpl) checkToken(ctx context.Context) (newToken, userName string, err error) {
	newToken, _, userName, err = impl.tokenHelper.ExtractUserFromGRPCContext(ctx, true)

	return
}

func (impl *customerUserServerImpl) CreateToken(ctx context.Context, request *customertalkpb.CreateTokenRequest) (*customertalkpb.CreateTokenResponse, error) {
	if request == nil {
		return nil, gRpcMessageError(codes.InvalidArgument, "noRequest")
	}

	token, userName, err := impl.createToken(ctx, request.GetUserName())
	if err != nil {
		return nil, gRpcError(codes.Internal, err)
	}

	return &customertalkpb.CreateTokenResponse{
		Token:    token,
		UserName: userName,
	}, nil
}

func (impl *customerUserServerImpl) createToken(ctx context.Context, userName string) (token, tokenUserName string, err error) {
	tokenUserName = userName
	if tokenUserName == "" {
		tokenUserName = "Guest"
	}

	resp, err := impl.user.Login(ctx, &userinters.LoginRequest{
		ContinueID:        0,
		Authenticators:    []userinters.Authenticator{anonymousauthenticator.NewAuthenticator(tokenUserName)},
		TokenLiveDuration: time.Hour * 24 * 7,
	})
	if err != nil {
		return
	}

	if resp.Status != userinters.LoginStatusSuccess {
		err = commerr.ErrInternal

		return
	}

	token = resp.Token

	return
}
