package defs

import (
	"context"
)

type MainRoutineRunner interface {
	Post(func())
}

type CustomerMD interface {
	Setup(mr MainRoutineRunner)
	InstallCustomer(ctx context.Context, customer Customer)
	UninstallCustomer(ctx context.Context, customer Customer)
	CustomerMessageIncoming(ctx context.Context, customer Customer,
		seqID uint64, message *TalkMessageW)
	CustomerClose(ctx context.Context, customer Customer)
}

type ServicerMD interface {
	Setup(mr MainRoutineRunner)
	InstallServicer(ctx context.Context, servicer Servicer)
	UninstallServicer(ctx context.Context, servicer Servicer)
	ServicerAttachTalk(ctx context.Context, talkID string, servicer Servicer)
	ServicerDetachTalk(ctx context.Context, talkID string, servicer Servicer)
	ServicerQueryAttachedTalks(ctx context.Context, servicer Servicer)
	ServicerQueryPendingTalks(ctx context.Context, servicer Servicer)
	ServicerReloadTalk(ctx context.Context, servicer Servicer, talkID string)
	ServiceMessage(ctx context.Context, servicer Servicer, talkID string, seqID uint64, message *TalkMessageW)
}

type MD interface {
	Load(ctx context.Context) (err error)
	CustomerMD
	ServicerMD
}
