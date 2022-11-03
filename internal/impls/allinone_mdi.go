package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
)

func NewAllInOneMDI(m defs.ModelEx, logger l.Wrapper) defs.MDI {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	return &allInOneMDIImpl{
		m:      m,
		logger: logger,
	}
}

type allInOneMDIImpl struct {
	m      defs.ModelEx
	logger l.Wrapper

	customerOb defs.CustomerObserver
	servicerOb defs.ServicerObserver
}

func (impl *allInOneMDIImpl) GetM() defs.ModelEx {
	return impl.m
}

func (impl *allInOneMDIImpl) Load(ctx context.Context) error {
	return nil
}

func (impl *allInOneMDIImpl) AddTrackTalk(ctx context.Context, talkID string) error {
	return nil
}

func (impl *allInOneMDIImpl) RemoveTrackTalk(ctx context.Context, talkID string) {

}

func (impl *allInOneMDIImpl) SendMessage(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	impl.customerOb.OnMessageIncoming(senderUniqueID, talkID, message)
	impl.servicerOb.OnMessageIncoming(senderUniqueID, talkID, message)
}

func (impl *allInOneMDIImpl) SetCustomerObserver(ob defs.CustomerObserver) {
	impl.customerOb = ob
}

func (impl *allInOneMDIImpl) SendTalkCloseMessage(talkID string) {
	impl.customerOb.OnTalkClose(talkID)
	impl.servicerOb.OnTalkClose(talkID)
}

func (impl *allInOneMDIImpl) SendTalkCreateMessage(talkID string) {
	impl.servicerOb.OnTalkCreate(talkID)
}

func (impl *allInOneMDIImpl) SetServicerObserver(ob defs.ServicerObserver) {
	impl.servicerOb = ob
}

func (impl *allInOneMDIImpl) SendServicerAttachMessage(talkID string, servicerID uint64) {
	impl.servicerOb.OnServicerAttachMessage(talkID, servicerID)
}

func (impl *allInOneMDIImpl) SendServiceDetachMessage(talkID string, servicerID uint64) {
	impl.servicerOb.OnServicerDetachMessage(talkID, servicerID)
}
