package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
)

func NewMemMDI(m defs.ModelEx, logger l.Wrapper) defs.MDI {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	return &memMDIImpl{
		m:      m,
		logger: logger,
	}
}

type memMDIImpl struct {
	m      defs.ModelEx
	logger l.Wrapper

	ob defs.Observer
}

func (impl *memMDIImpl) GetM() defs.ModelEx {
	return impl.m
}

func (impl *memMDIImpl) SetObserver(ob defs.Observer) {
	impl.ob = ob
}

func (impl *memMDIImpl) Load(ctx context.Context) (_ error) {
	return
}

func (impl *memMDIImpl) AddTrackTalk(ctx context.Context, talkID string) error {
	return nil
}

func (impl *memMDIImpl) RemoveTrackTalk(ctx context.Context, talkID string) {

}

func (impl *memMDIImpl) SendMessage(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	impl.ob.OnMessageIncoming(senderUniqueID, talkID, message)
}

func (impl *memMDIImpl) SendTalkCloseMessage(talkID string) {
	impl.ob.OnTalkClose(talkID)
}

func (impl *memMDIImpl) SendServicerAttachMessage(talkID string, servicerID uint64) {
	impl.ob.OnServicerAttachMessage(talkID, servicerID)
}

func (impl *memMDIImpl) SendServiceDetachMessage(talkID string, servicerID uint64) {
	impl.ob.OnServicerDetachMessage(talkID, servicerID)
}
