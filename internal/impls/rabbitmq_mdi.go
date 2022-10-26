package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
)

func NewRabbitMQMDI(mqURL string, m defs.ModelEx, logger l.Wrapper) defs.MDI {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	mq, err := NewRabbitMQ(mqURL, logger)
	if err != nil {
		return nil
	}

	return &rabbitMQMDIImpl{
		m:        m,
		logger:   logger,
		rabbitMQ: mq,
	}
}

type rabbitMQMDIImpl struct {
	m      defs.ModelEx
	logger l.Wrapper

	rabbitMQ RabbitMQ
}

func (impl *rabbitMQMDIImpl) GetM() defs.ModelEx {
	return impl.m
}

func (impl *rabbitMQMDIImpl) SetObserver(ob defs.Observer) {
	impl.rabbitMQ.SetObserver(ob)
}

func (impl *rabbitMQMDIImpl) Load(ctx context.Context) error {
	return nil
}

func (impl *rabbitMQMDIImpl) AddTrackTalk(ctx context.Context, talkID string) error {
	return impl.rabbitMQ.AddTrackTalk(talkID)
}

func (impl *rabbitMQMDIImpl) RemoveTrackTalk(ctx context.Context, talkID string) {
	impl.rabbitMQ.RemoveTrackTalk(talkID)
}

func (impl *rabbitMQMDIImpl) SendMessage(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID: talkID,
		Message: &mqDataMessage{
			SenderUniqueID: senderUniqueID,
			Message:        message,
		},
	})
}

func (impl *rabbitMQMDIImpl) SendTalkCloseMessage(talkID string) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID:    talkID,
		TalkClose: &mqDataTalkClose{},
	})
}

func (impl *rabbitMQMDIImpl) SendServicerAttachMessage(talkID string, servicerID uint64) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID: talkID,
		ServicerAttach: &mqDataServicerAttach{
			ServicerID: servicerID,
		},
	})
}

func (impl *rabbitMQMDIImpl) SendServiceDetachMessage(talkID string, servicerID uint64) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID: talkID,
		ServicerDetach: &mqDataServicerDetach{
			ServicerID: servicerID,
		},
	})
}
