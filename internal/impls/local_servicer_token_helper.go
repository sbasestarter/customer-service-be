package impls

import (
	"context"
	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/userlib/manager/userpass"
	"github.com/sgostarter/libeasygo/commerr"
	"google.golang.org/grpc/metadata"
)

func NewLocalServicerUserTokenHelper(user userinters.UserCenter, manager userpass.Manager) defs.UserTokenHelper {
	return &localServicerUserTokenHelperImpl{
		user:    user,
		manager: manager,
	}
}

type localServicerUserTokenHelperImpl struct {
	user    userinters.UserCenter
	manager userpass.Manager
}

func (impl *localServicerUserTokenHelperImpl) ExtractTokenFromGRPCContext(ctx context.Context) (token string, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		err = commerr.ErrUnauthenticated

		return
	}

	tokens := md.Get(tokenKeyOnMetadata)
	if len(tokens) == 0 || tokens[0] == "" {
		err = commerr.ErrUnauthenticated

		return
	}

	token = tokens[0]

	return
}

func (impl *localServicerUserTokenHelperImpl) ExplainToken(ctx context.Context, token string, renewToken bool) (newToken string, userID uint64, userName string, err error) {
	newToken, userID, _, err = impl.user.CheckToken(ctx, token, renewToken)
	if err != nil {
		return
	}

	user, err := impl.manager.GetUser(ctx, userID)
	if err != nil {
		return
	}

	userName = user.UserName

	return
}

func (impl *localServicerUserTokenHelperImpl) ExtractUserFromGRPCContext(ctx context.Context, renewToken bool) (newToken string, userID uint64, userName string, err error) {
	token, err := impl.ExtractTokenFromGRPCContext(ctx)
	if err != nil {
		return
	}

	newToken, userID, userName, err = impl.ExplainToken(ctx, token, renewToken)

	return
}
