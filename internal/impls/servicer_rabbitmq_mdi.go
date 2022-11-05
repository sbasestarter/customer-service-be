package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/args"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
)

func NewServicerRabbitMQMDI(mqURL string, m defs.ModelEx, logger l.Wrapper) defs.ServicerMDI {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	mq, err := NewRabbitMQ(mqURL, UserModeServicer, logger)
	if err != nil {
		return nil
	}

	return &servicerRabbitMQImpl{
		m:        m,
		logger:   logger,
		rabbitMQ: mq,
	}
}

type servicerRabbitMQImpl struct {
	m      defs.ModelEx
	logger l.Wrapper

	rabbitMQ RabbitMQ
}

func (impl *servicerRabbitMQImpl) GetM() defs.ModelEx {
	return impl.m
}

func (impl *servicerRabbitMQImpl) Load(ctx context.Context) error {
	return nil
}

func (impl *servicerRabbitMQImpl) AddTrackTalk(ctx context.Context, talkID string) error {
	if args.RabbitMQUseSharedChannel {
		return nil
	}

	return impl.rabbitMQ.AddTrackTalk(talkID)
}

func (impl *servicerRabbitMQImpl) RemoveTrackTalk(ctx context.Context, talkID string) {
	if args.RabbitMQUseSharedChannel {
		return
	}

	impl.rabbitMQ.RemoveTrackTalk(talkID)
}

func (impl *servicerRabbitMQImpl) SendMessage(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
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

func (impl *servicerRabbitMQImpl) SetServicerObserver(ob defs.ServicerObserver) {
	impl.rabbitMQ.SetServicerObserver(ob)
}

func (impl *servicerRabbitMQImpl) SendServicerAttachMessage(talkID string, servicerID uint64) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID:    talkID,
		ChannelID: specialTalkServicer,
		ServicerAttach: &mqDataServicerAttach{
			ServicerID: servicerID,
		},
	})
}

func (impl *servicerRabbitMQImpl) SendServiceDetachMessage(talkID string, servicerID uint64) {
	_ = impl.rabbitMQ.SendData(&mqData{
		TalkID:    talkID,
		ChannelID: specialTalkServicer,
		ServicerDetach: &mqDataServicerDetach{
			ServicerID: servicerID,
		},
	})
}
