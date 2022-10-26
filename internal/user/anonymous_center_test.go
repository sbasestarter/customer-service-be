package user

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestAnonymousCenterImpl(t *testing.T) {
	ctx := context.TODO()

	impl := NewAnonymousCenter("a", time.Second)
	newCtx, err := impl.Login(ctx, "abc")
	assert.Nil(t, err)

	md, ok := metadata.FromOutgoingContext(newCtx)
	assert.True(t, ok)

	userInfo, err := impl.ExtractUserInfoFromGRPCContext(metadata.NewIncomingContext(ctx, md))
	assert.Nil(t, err)
	assert.EqualValues(t, "abc", userInfo.UserName)

	time.Sleep(time.Second * 2)

	_, err = impl.ExtractUserInfoFromGRPCContext(metadata.NewIncomingContext(ctx, md))
	assert.NotNil(t, err)
}
