package impls

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libeasygo/commerr"
	"github.com/sgostarter/libeasygo/routineman"
	"github.com/streadway/amqp"
)

type RabbitMQ interface {
	AddTrackTalk(talkID string) error
	RemoveTrackTalk(talkID string)
	SendData(data *mqData) error
	SetCustomerObserver(ob defs.CustomerObserver)
	SetServicerObserver(ob defs.ServicerObserver)
}

func NewRabbitMQ(url string, logger l.Wrapper) (RabbitMQ, error) {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	impl := &rabbitMQImpl{
		logger: logger.WithFields(l.StringField(l.ClsKey, "rabbitMQImpl")),

		routineMan:              routineman.NewRoutineMan(context.TODO(), logger),
		chTalkTrackStartRequest: make(chan string, 10),
		chTalkTrackStopRequest:  make(chan string, 10),
		chTalkTrackStartedEvent: make(chan *talkTrackStartedEventData, 10),
		chTalkTrackStoppedEvent: make(chan *talkTrackStoppedEventData, 10),
		chSend:                  make(chan *mqData, 100),
	}

	if err := impl.init(url); err != nil {
		return nil, err
	}

	return impl, nil
}

type mqDataMessage struct {
	SenderUniqueID uint64
	Message        *defs.TalkMessageW
}

type mqDataTalkCreate struct {
	TalkID string
}

type mqDataTalkClose struct {
}

type mqDataServicerAttach struct {
	ServicerID uint64
}

type mqDataServicerDetach struct {
	ServicerID uint64
}

type mqData struct {
	TalkID         string                `json:"TalkID,omitempty"`
	Message        *mqDataMessage        `json:"Message,omitempty"`
	TalkCreate     *mqDataTalkCreate     `json:"TalkCreate,omitempty"`
	TalkClose      *mqDataTalkClose      `json:"TalkClose,omitempty"`
	ServicerAttach *mqDataServicerAttach `json:"ServicerAttach,omitempty"`
	ServicerDetach *mqDataServicerDetach `json:"ServicerDetach,omitempty"`
}

type talkTrackStartedEventData struct {
	talkID    string
	ctxCancel context.CancelFunc
}

type talkTrackStoppedEventData struct {
	talkID string
	err    error
}

type rabbitMQImpl struct {
	customerOb defs.CustomerObserver
	servicerOb defs.ServicerObserver
	logger     l.Wrapper

	conn *amqp.Connection

	routineMan routineman.RoutineMan

	chTalkTrackStartRequest chan string
	chTalkTrackStopRequest  chan string
	chTalkTrackStartedEvent chan *talkTrackStartedEventData
	chTalkTrackStoppedEvent chan *talkTrackStoppedEventData
	chSend                  chan *mqData
}

func (impl *rabbitMQImpl) SendData(data *mqData) error {
	if data == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case impl.chSend <- data:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (impl *rabbitMQImpl) AddTrackTalk(talkID string) error {
	if talkID == "" {
		return commerr.ErrInvalidArgument
	}

	select {
	case impl.chTalkTrackStartRequest <- talkID:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (impl *rabbitMQImpl) RemoveTrackTalk(talkID string) {
	if talkID == "" {
		return
	}

	select {
	case impl.chTalkTrackStopRequest <- talkID:
	default:
	}
}

func (impl *rabbitMQImpl) SetCustomerObserver(ob defs.CustomerObserver) {
	impl.customerOb = ob
}

func (impl *rabbitMQImpl) SetServicerObserver(ob defs.ServicerObserver) {
	impl.servicerOb = ob
}

func (impl *rabbitMQImpl) init(url string) (err error) {
	impl.conn, err = amqp.DialConfig(url, amqp.Config{
		ChannelMax: math.MaxInt,
	})
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("DialFailed")

		return
	}

	impl.routineMan.StartRoutine(impl.mainRoutine, "mainRoutine")

	return
}

type trackTalkData struct {
	cancel context.CancelFunc
}

func (impl *rabbitMQImpl) mainRoutine(ctx context.Context, exiting func() bool) {
	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "mainRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	loop := true

	trackTalkMap := make(map[string]*trackTalkData)

	channelSend, err := impl.conn.Channel()
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("ChannelFailed")

		return
	}

	for loop {
		select {
		case <-ctx.Done():
			loop = false

			break
		case talkID := <-impl.chTalkTrackStartRequest:
			if _, ok := trackTalkMap[talkID]; ok {
				logger.WithFields(l.StringField("talkID", talkID)).Error("TrackTalkExists")

				continue
			}

			trackTalkMap[talkID] = &trackTalkData{}

			ret := make(chan error, 2)
			impl.routineMan.StartRoutine(func(ctx context.Context, exiting func() bool) {
				impl.trackTalkRoutine(ctx, talkID, ret)
			}, "trackTalkRoutine")
			<-ret
		case talkID := <-impl.chTalkTrackStopRequest:
			if trackTalk, ok := trackTalkMap[talkID]; ok {
				if trackTalk.cancel != nil {
					trackTalk.cancel()
				}
			} else {
				logger.WithFields(l.StringField("talkID", talkID)).Error("TackNotExists")
			}
		case d := <-impl.chTalkTrackStartedEvent:
			if trackTalk, ok := trackTalkMap[d.talkID]; ok {
				trackTalk.cancel = d.ctxCancel
			} else {
				logger.WithFields(l.StringField("talkID", d.talkID)).Error("TackNotExists")
			}
		case d := <-impl.chTalkTrackStoppedEvent:
			if trackTalk, ok := trackTalkMap[d.talkID]; ok {
				if trackTalk.cancel != nil {
					trackTalk.cancel()
				}

				delete(trackTalkMap, d.talkID)
			} else {
				logger.WithFields(l.StringField("talkID", d.talkID)).Error("TrackNotExists")
			}
		case sendD := <-impl.chSend:
			d, _ := json.Marshal(sendD)

			if err = channelSend.Publish(impl.exchangeName(sendD.TalkID), "", false, false,
				amqp.Publishing{
					Body: d,
				}); err != nil {
				logger.WithFields(l.ErrorField(err), l.StringField("talkID", sendD.TalkID)).Error("PublishFailed")
			}
		}
	}
}

func (impl *rabbitMQImpl) exchangeName(talkID string) string {
	return "talk:" + talkID
}

func (impl *rabbitMQImpl) trackTalkRoutine(ctx context.Context, talkID string, start chan<- error) {
	logger := impl.logger.WithFields(l.StringField("talkID", talkID), l.StringField(l.RoutineKey,
		"trackTalkRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	fnQuitWithErrorAndLabel := func(err error, label string) {
		start <- err
		select {
		case impl.chTalkTrackStoppedEvent <- &talkTrackStoppedEventData{
			talkID: talkID,
			err:    fmt.Errorf("%s: %w", label, err),
		}:
		default:
		}
	}

	channel, err := impl.conn.Channel()
	if err != nil {
		fnQuitWithErrorAndLabel(err, "Channel")

		return
	}

	err = channel.ExchangeDeclare(impl.exchangeName(talkID), "fanout", false, true,
		false, false, nil)
	if err != nil {
		fnQuitWithErrorAndLabel(err, "ExchangeDeclare")

		return
	}

	q, err := channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		fnQuitWithErrorAndLabel(err, "QueueDeclare")

		return
	}

	err = channel.QueueBind(q.Name, "", impl.exchangeName(talkID), false, nil)
	if err != nil {
		fnQuitWithErrorAndLabel(err, "QueueBind")

		return
	}

	deliveries, err := channel.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		fnQuitWithErrorAndLabel(err, "Consume")

		return
	}

	start <- nil

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	select {
	case impl.chTalkTrackStartedEvent <- &talkTrackStartedEventData{
		talkID:    talkID,
		ctxCancel: cancel,
	}:
	default:
	}

	loop := true

	for loop {
		select {
		case <-ctx.Done():
			loop = false

			continue
		case d := <-deliveries:
			var obj mqData

			err = json.Unmarshal(d.Body, &obj)
			if err != nil {
				logger.WithFields(l.ErrorField(err), l.StringField("payload", string(d.Body))).
					Error("UnmarshalFailed")

				continue
			}

			if obj.Message != nil {
				if impl.customerOb != nil {
					impl.customerOb.OnMessageIncoming(obj.Message.SenderUniqueID, obj.TalkID, obj.Message.Message)
				}

				if impl.servicerOb != nil {
					impl.servicerOb.OnMessageIncoming(obj.Message.SenderUniqueID, obj.TalkID, obj.Message.Message)
				}
			} else if obj.TalkClose != nil {
				if impl.customerOb != nil {
					impl.customerOb.OnTalkClose(obj.TalkID)
				}

				if impl.servicerOb != nil {
					impl.servicerOb.OnTalkClose(obj.TalkID)
				}
			} else if obj.TalkCreate != nil {
				impl.servicerOb.OnTalkCreate(obj.TalkID)
			} else if obj.ServicerAttach != nil {
				impl.servicerOb.OnServicerAttachMessage(obj.TalkID, obj.ServicerAttach.ServicerID)
			} else if obj.ServicerDetach != nil {
				impl.servicerOb.OnServicerDetachMessage(obj.TalkID, obj.ServicerDetach.ServicerID)
			} else {
				logger.Error("UnknownMqData")
			}
		}
	}

	select {
	case impl.chTalkTrackStoppedEvent <- &talkTrackStoppedEventData{
		talkID: talkID,
		err:    nil,
	}:
	default:
	}
}
