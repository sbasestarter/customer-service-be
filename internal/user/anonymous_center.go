package user

import (
	"context"
	"crypto/md5" // nolint:gosec
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/godruoyi/go-snowflake"
	"github.com/sgostarter/libeasygo/commerr"
	"google.golang.org/grpc/metadata"
)

type AnonymousCenter interface {
	Center

	Login(ctx context.Context, userName string) (outCtx context.Context, err error)
}

func NewAnonymousCenter(secKey string, expireDuration time.Duration) AnonymousCenter {
	if expireDuration < time.Second {
		expireDuration = time.Second
	}

	// nolint: gosec
	h := md5.Sum([]byte(secKey))

	return &anonymousCenterImpl{
		secKey:         h[:],
		expireDuration: expireDuration,
	}
}

type anonymousCenterImpl struct {
	secKey         interface{}
	expireDuration time.Duration
}

type anonymousUserClaims struct {
	Info
	jwt.StandardClaims
}

func (impl *anonymousCenterImpl) Login(ctx context.Context, userName string) (outCtx context.Context, err error) {
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, anonymousUserClaims{
		Info: Info{
			ID:       snowflake.ID(),
			UserName: userName,
		},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(impl.expireDuration).Unix(),
		},
	}).SignedString(impl.secKey)
	if err != nil {
		return
	}

	outCtx = metadata.NewOutgoingContext(ctx, metadata.Pairs("token", token))

	return
}

func (impl *anonymousCenterImpl) ExtractUserInfoFromGRPCContext(ctx context.Context) (userInfo *Info, err error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		err = commerr.ErrUnauthenticated

		return
	}

	tokens := md.Get("token")
	if len(tokens) == 0 {
		err = commerr.ErrUnauthenticated

		return
	}

	var claims anonymousUserClaims

	token, err := jwt.ParseWithClaims(tokens[0], &claims, func(token *jwt.Token) (interface{}, error) {
		return impl.secKey, nil
	})

	if anonymousClaims, ok := token.Claims.(*anonymousUserClaims); ok && token.Valid {
		userInfo = &Info{
			ID:       anonymousClaims.ID,
			UserName: anonymousClaims.UserName,
		}
	} else {
		err = commerr.ErrUnauthenticated
	}

	return
}
