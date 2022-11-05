package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/args"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
)

func NewCustomerRabbitMQMDI(mqURL string, m defs.ModelEx, logger l.Wrapper) defs.CustomerMDI {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	mq, err := NewRabbitMQ(mqURL, UserModeCustomer, logger)
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
	if args.RabbitMQUseSharedChannel {
		return nil
	}

	return impl.rabbitMQ.AddTrackTalk(talkID)
}

func (impl *customerRabbitMQImpl) RemoveTrackTalk(ctx context.Context, talkID string) {
	if args.RabbitMQUseSharedChannel {
		return
	}

	impl.rabbitMQ.RemoveTrackTalk(talkID)
}

func (impl *customerRabbitMQImpl) SendMessage(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	d := &mqData{
		TalkID: talkID,
		Message: &mqDataMessage{
			SenderUniqueID: senderUniqueID,
			Message:        message,
		},
	}

	if args.RabbitMQUseSharedChannel {
		d.ChannelID = specialTalkAll
	}

	_ = impl.rabbitMQ.SendData(d)
}

func (impl *customerRabbitMQImpl) SetCustomerObserver(ob defs.CustomerObserver) {
	impl.rabbitMQ.SetCustomerObserver(ob)
}

func (impl *customerRabbitMQImpl) SendTalkCloseMessage(talkID string) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID:    talkID,
		ChannelID: specialTalkAll,
		TalkClose: &mqDataTalkClose{},
	})
}

func (impl *customerRabbitMQImpl) SendTalkCreateMessage(talkID string) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID:    talkID,
		ChannelID: specialTalkServicer,
		TalkCreate: &mqDataTalkCreate{
			TalkID: talkID,
		},
	})
}
