package defs

import "context"

type CustomerObserver interface {
	OnMessageIncoming(senderUniqueID uint64, talkID string, message *TalkMessageW)
	OnTalkClose(talkID string)
}

type ServicerObserver interface {
	OnMessageIncoming(senderUniqueID uint64, talkID string, message *TalkMessageW)

	OnTalkCreate(talkID string)
	OnTalkClose(talkID string)

	OnServicerAttachMessage(talkID string, servicerID uint64)
	OnServicerDetachMessage(talkID string, servicerID uint64)
}

type Observer interface {
	CustomerObserver
	ServicerObserver
}

type MDIBase interface {
	GetM() ModelEx

	Load(ctx context.Context) error

	AddTrackTalk(ctx context.Context, talkID string) error
	RemoveTrackTalk(ctx context.Context, talkID string)

	SendMessage(senderUniqueID uint64, talkID string, message *TalkMessageW)
}

type CustomerMDI interface {
	MDIBase
	SetCustomerObserver(ob CustomerObserver)
	SendTalkCloseMessage(talkID string)
	SendTalkCreateMessage(talkID string)
}

type ServicerMDI interface {
	MDIBase
	SetServicerObserver(ob ServicerObserver)
	SendServicerAttachMessage(talkID string, servicerID uint64)
	SendServiceDetachMessage(talkID string, servicerID uint64)
}

type MDI interface {
	CustomerMDI
	ServicerMDI
}
