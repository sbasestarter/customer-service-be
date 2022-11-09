package user

import "context"

type Info struct {
	ID       uint64
	UserName string
}

type Center interface {
	ExtractUserInfoFromGRPCContext(ctx context.Context) (userInfo *Info, err error)
	NewToken(userInfo *Info) (string, error)
}
