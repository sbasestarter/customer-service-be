package defs

import (
	"context"
)

type Observer interface {
	OnMessageIncoming(senderUniqueID uint64, talkID string, message *TalkMessageW)
	OnTalkClose(talkID string)
	OnServicerAttachMessage(talkID string, servicerID uint64)
	OnServicerDetachMessage(talkID string, servicerID uint64)
}

type MDI interface {
	GetM() ModelEx

	SetObserver(ob Observer)

	Load(ctx context.Context) error

	AddTrackTalk(ctx context.Context, talkID string) error
	RemoveTrackTalk(ctx context.Context, talkID string)

	SendMessage(senderUniqueID uint64, talkID string, message *TalkMessageW)
	SendTalkCloseMessage(talkID string)
	SendServicerAttachMessage(talkID string, servicerID uint64)
	SendServiceDetachMessage(talkID string, servicerID uint64)
}
