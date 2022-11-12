package impls

import (
	"context"

	"github.com/sbasestarter/bizinters/userinters"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	anonymousmanager "github.com/sbasestarter/userlib/manager/anonymous"
	"github.com/sgostarter/libeasygo/commerr"
	"google.golang.org/grpc/metadata"
)

const (
	tokenKeyOnMetadata = "token"
)

func NewLocalCustomerUserTokenHelper(user userinters.UserCenter) defs.UserTokenHelper {
	return &localCustomerUserTokenHelperImpl{
		user:    user,
		manager: anonymousmanager.NewManager(),
	}
}

type localCustomerUserTokenHelperImpl struct {
	user    userinters.UserCenter
	manager anonymousmanager.Manager
}

func (impl *localCustomerUserTokenHelperImpl) ExtractTokenFromGRPCContext(ctx context.Context) (token string, err error) {
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

func (impl *localCustomerUserTokenHelperImpl) ExplainToken(ctx context.Context, token string,
	renewToken bool) (newToken string, userID uint64, userName string, err error) {
	newToken, userID, tokenDataList, err := impl.user.CheckToken(ctx, token, renewToken)
	if err != nil {
		return
	}

	userName, err = impl.manager.GetUser(ctx, userID, tokenDataList)
	if err != nil {
		return
	}

	return
}

func (impl *localCustomerUserTokenHelperImpl) ExtractUserFromGRPCContext(ctx context.Context,
	renewToken bool) (newToken string, userID uint64, userName string, err error) {
	token, err := impl.ExtractTokenFromGRPCContext(ctx)
	if err != nil {
		return
	}

	newToken, userID, userName, err = impl.ExplainToken(ctx, token, renewToken)

	return
}
