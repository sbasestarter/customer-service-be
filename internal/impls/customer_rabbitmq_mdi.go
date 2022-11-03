package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
)

func NewCustomerRabbitMQMDI(mqURL string, m defs.ModelEx, logger l.Wrapper) defs.CustomerMDI {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	mq, err := NewRabbitMQ(mqURL, logger)
	if err != nil {
		return nil
	}

	return &customerRabbitMQImpl{
		m:        m,
		logger:   logger,
		rabbitMQ: mq,
	}
}

type customerRabbitMQImpl struct {
	m      defs.ModelEx
	logger l.Wrapper

	rabbitMQ RabbitMQ
}

//
// defs.CustomerMDI
//

func (impl *customerRabbitMQImpl) GetM() defs.ModelEx {
	return impl.m
}

func (impl *customerRabbitMQImpl) Load(ctx context.Context) error {
	return nil
}

func (impl *customerRabbitMQImpl) AddTrackTalk(ctx context.Context, talkID string) error {
	return impl.rabbitMQ.AddTrackTalk(talkID)
}

func (impl *customerRabbitMQImpl) RemoveTrackTalk(ctx context.Context, talkID string) {
	impl.rabbitMQ.RemoveTrackTalk(talkID)
}

func (impl *customerRabbitMQImpl) SendMessage(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID: talkID,
		Message: &mqDataMessage{
			SenderUniqueID: senderUniqueID,
			Message:        message,
		},
	})
}

func (impl *customerRabbitMQImpl) SetCustomerObserver(ob defs.CustomerObserver) {
	impl.rabbitMQ.SetCustomerObserver(ob)
}

func (impl *customerRabbitMQImpl) SendTalkCloseMessage(talkID string) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID:    talkID,
		TalkClose: &mqDataTalkClose{},
	})
}

func (impl *customerRabbitMQImpl) SendTalkCreateMessage(talkID string) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID: talkID,
		TalkCreate: &mqDataTalkCreate{
			TalkID: talkID,
		},
	})
}
