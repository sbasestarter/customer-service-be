package impls

import (
	"testing"
	"time"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
	"github.com/stretchr/testify/assert"
)

const UtMqURL = "amqp://admin:admin@127.0.0.1:8310/"

type obImpl struct {
	t  *testing.T
	id string
}

func (impl *obImpl) OnMessageIncoming(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	impl.t.Log(impl.id+" => OnMessageIncoming:", senderUniqueID, talkID, message.Text)
}

func (impl *obImpl) OnTalkClose(talkID string) {
	impl.t.Log(impl.id+" => OnTalkClose:", talkID)
}

func (impl *obImpl) OnServicerAttachMessage(talkID string, servicerID uint64) {
	impl.t.Log(impl.id+" => OnServicerAttachMessage:", talkID, servicerID)
}

func (impl *obImpl) OnServicerDetachMessage(talkID string, servicerID uint64) {
	impl.t.Log(impl.id+" => OnServicerDetachMessage:", talkID, servicerID)
}

func TestRabbitMQImpl(t *testing.T) {
	mq1, err := NewRabbitMQ(UtMqURL, l.NewConsoleLoggerWrapper())
	assert.Nil(t, err)
	mq1.SetObserver(&obImpl{t: t, id: "mq1"})

	mq2, err := NewRabbitMQ(UtMqURL, l.NewConsoleLoggerWrapper())
	assert.Nil(t, err)
	mq2.SetObserver(&obImpl{t: t, id: "mq2"})

	talk1 := "t1"
	talk2 := "t2"

	err = mq1.AddTrackTalk(talk1)
	assert.Nil(t, err)

	err = mq2.AddTrackTalk(talk1)
	assert.Nil(t, err)
	err = mq2.AddTrackTalk(talk2)
	assert.Nil(t, err)

	time.Sleep(time.Second * 2)

	err = mq2.SendData(&mqData{
		TalkID: talk1,
		Message: &mqDataMessage{
			SenderUniqueID: 100,
			Message: &defs.TalkMessageW{
				CustomerMessage: true,
				Type:            defs.TalkMessageTypeText,
				SenderID:        100,
				Text:            "hello1",
			},
		},
	})
	assert.Nil(t, err)

	err = mq2.SendData(&mqData{
		TalkID: talk2,
		Message: &mqDataMessage{
			SenderUniqueID: 100,
			Message: &defs.TalkMessageW{
				CustomerMessage: true,
				Type:            defs.TalkMessageTypeText,
				SenderID:        100,
				Text:            "hello2",
			},
		},
	})
	assert.Nil(t, err)

	err = mq2.SendData(&mqData{
		TalkID: talk2,
		Message: &mqDataMessage{
			SenderUniqueID: 100,
			Message: &defs.TalkMessageW{
				CustomerMessage: true,
				Type:            defs.TalkMessageTypeText,
				SenderID:        100,
				Text:            "hello3",
			},
		},
	})
	assert.Nil(t, err)

	time.Sleep(time.Second * 5)
}
