package defs

import (
	"context"
)

type MD interface {
	Load(ctx context.Context) (err error)
	InstallCustomer(ctx context.Context, customer Customer)
	UninstallCustomer(_ context.Context, customer Customer)
	CustomerMessageIncoming(ctx context.Context, customer Customer,
		seqID uint64, message *TalkMessageW)
	CustomerClose(ctx context.Context, customer Customer)
	InstallServicer(ctx context.Context, servicer Servicer)
	UninstallServicer(ctx context.Context, servicer Servicer)
	ServicerAttachTalk(ctx context.Context, talkID string, servicer Servicer)
	ServicerDetachTalk(ctx context.Context, talkID string, servicer Servicer)
	ServicerQueryAttachedTalks(ctx context.Context, servicer Servicer)
	ServicerQueryPendingTalks(_ context.Context, servicer Servicer)
	ServicerReloadTalk(ctx context.Context, servicer Servicer, talkID string)
	ServiceMessage(_ context.Context, servicer Servicer, talkID string, seqID uint64, message *TalkMessageW)
}
