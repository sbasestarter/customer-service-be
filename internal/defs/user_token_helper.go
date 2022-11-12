package defs

import "context"

type UserTokenExtractor interface {
	ExtractTokenFromGRPCContext(ctx context.Context) (token string, err error)
}

type UserTokenExplain interface {
	ExplainToken(ctx context.Context, token string, renewToken bool) (newToken string, userID uint64, userName string, err error)
}

type UserTokenHelper interface {
	UserTokenExtractor
	UserTokenExplain
	ExtractUserFromGRPCContext(ctx context.Context, renewToken bool) (newToken string, userID uint64, userName string, err error)
}
