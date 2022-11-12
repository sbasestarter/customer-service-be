package impls

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libeasygo/commerr"
	"github.com/sgostarter/libeasygo/routineman"
	"github.com/streadway/amqp"
	"go.uber.org/atomic"
)

const (
	tickerRetryDuration = time.Second
	tickerCheckDuration = time.Hour

	specialTalkCustomer = "customerC"
	specialTalkServicer = "servicerC"
	specialTalkAll      = "C"
)

type UserMode int

const (
	UserModeCustomer UserMode = iota
	UserModeServicer
)

type RabbitMQ interface {
	AddTrackTalk(talkID string) error
	RemoveTrackTalk(talkID string)
	SendData(data *mqData) error
	SetCustomerObserver(ob defs.CustomerObserver)
	SetServicerObserver(ob defs.ServicerObserver)
}

func NewRabbitMQ(url string, userMode UserMode, logger l.Wrapper) (RabbitMQ, error) {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	impl := &rabbitMQImpl{
		mqURL:    url,
		userMode: userMode,
		logger:   logger.WithFields(l.StringField(l.ClsKey, "rabbitMQImpl")),

		routineMan:              routineman.NewRoutineMan(context.TODO(), logger),
		chTalkTrackStartRequest: make(chan string, 10),
		chTalkTrackStopRequest:  make(chan string, 10),
		chTalkTrackStartedEvent: make(chan *talkTrackStartedEventData, 10),
		chTalkTrackStoppedEvent: make(chan *talkTrackStoppedEventData, 10),
		chSend:                  make(chan *mqData, 100),
		chConnDialSuccess:       make(chan *amqp.Connection, 10),
	}

	if err := impl.init(); err != nil {
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
	ChannelID      string                `json:"ChannelID"` // empty channel id equal talk id
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

type connWrapper struct {
	conn *amqp.Connection
}

type rabbitMQImpl struct {
	customerOb defs.CustomerObserver
	servicerOb defs.ServicerObserver
	mqURL      string
	userMode   UserMode
	logger     l.Wrapper

	routineMan routineman.RoutineMan

	chTalkTrackStartRequest chan string
	chTalkTrackStopRequest  chan string
	chTalkTrackStartedEvent chan *talkTrackStartedEventData
	chTalkTrackStoppedEvent chan *talkTrackStoppedEventData
	chSend                  chan *mqData

	conn              atomic.Value
	chConnDialSuccess chan *amqp.Connection
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

func (impl *rabbitMQImpl) init() (err error) {
	impl.conn.Store(connWrapper{})
	impl.routineMan.StartRoutine(impl.dialRoutine, "dialRoutine")
	impl.routineMan.StartRoutine(impl.mainRoutine, "mainRoutine")

	return
}

type trackTalkData struct {
	cancel context.CancelFunc
}

func (impl *rabbitMQImpl) dialRoutine(ctx context.Context, exiting func() bool) {
	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "dialRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	chConnBroken := make(chan *amqp.Error)

	fnDialConnection := func(closeNotifier chan *amqp.Error) (conn *amqp.Connection, err error) {
		logger.Debug("StartDialConnection")
		conn, err = amqp.DialConfig(impl.mqURL, amqp.Config{
			ChannelMax: math.MaxInt,
		})
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("DialFailed")

			return
		}

		conn.NotifyClose(closeNotifier)

		logger.Debug("SuccessDialConnection")

		return
	}

	var conn *amqp.Connection
	var err error

	loop := true

	checkTicker := time.NewTicker(tickerRetryDuration)

	for loop {
		select {
		case <-ctx.Done():
			loop = false

			break
		case <-checkTicker.C:
			checkTicker.Reset(tickerCheckDuration)

			if conn != nil {
				break
			}

			conn, err = fnDialConnection(chConnBroken)
			if err != nil {
				checkTicker.Reset(tickerRetryDuration)

				break
			}

			impl.conn.Store(connWrapper{conn: conn})

			select {
			case impl.chConnDialSuccess <- conn:
			default:
			}
		case connErr, _ := <-chConnBroken:
			logger.WithFields(l.StringField("desc", impl.mqErrorDesc(connErr))).Error("ConnBroken")

			chConnBroken = make(chan *amqp.Error)

			if conn != nil {
				if conn.IsClosed() {
					_ = conn.Close()
				}

				conn = nil
			}

			impl.conn.Store(connWrapper{})

			checkTicker.Reset(tickerRetryDuration)
		}
	}
}

func (impl *rabbitMQImpl) mainRoutine(ctx context.Context, exiting func() bool) {
	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "mainRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	trackTalkMap := make(map[string]*trackTalkData)

	var channelSend *amqp.Channel

	sendChannelBrokenNotifier := make(chan *amqp.Error)

	impl.routineMan.StartRoutine(func(ctx context.Context, exiting func() bool) {
		impl.trackTalkRoutine(ctx, specialTalkAll, nil)
	}, "trackTalkRoutine_0")

	impl.routineMan.StartRoutine(func(ctx context.Context, exiting func() bool) {
		talkID := specialTalkCustomer
		if impl.userMode == UserModeServicer {
			talkID = specialTalkServicer
		}

		impl.trackTalkRoutine(ctx, talkID, nil)
	}, "trackTalkRoutine_1")

	var err error

	checkTicker := time.NewTicker(tickerRetryDuration)

	loop := true

	cachedSendData := make([]*mqData, 0, 100)

	for loop {
		select {
		case <-ctx.Done():
			loop = false

			break
		case _ = <-impl.chConnDialSuccess:
			if channelSend != nil {
				_ = channelSend.Close()

				channelSend = nil
			}

			checkTicker.Reset(tickerRetryDuration)
		case <-checkTicker.C:
			checkTicker.Reset(tickerCheckDuration)

			if channelSend != nil {
				break
			}

			connW, ok := impl.conn.Load().(connWrapper)
			if !ok || connW.conn == nil {
				checkTicker.Reset(tickerRetryDuration)

				break
			}

			channelSend, err = connW.conn.Channel()
			if err != nil {
				impl.logger.WithFields(l.ErrorField(err)).Error("ChannelFailed")

				checkTicker.Reset(tickerRetryDuration)

				break
			}

			channelSend.NotifyClose(sendChannelBrokenNotifier)

			for _, data := range cachedSendData {
				d, _ := json.Marshal(data)

				if err = channelSend.Publish(impl.exchangeName(data.TalkID), "", false, false,
					amqp.Publishing{
						Body: d,
					}); err != nil {
					logger.WithFields(l.ErrorField(err), l.StringField("talkID", data.TalkID)).Error("PublishFailed")
				}
			}
		case mqErr, ok := <-sendChannelBrokenNotifier:
			logger.WithFields().WithFields(l.BoolField("ok", ok), l.StringField("desc", impl.mqErrorDesc(mqErr))).Error("SendChannelClosed")

			if channelSend != nil {
				_ = channelSend.Close()
				channelSend = nil
			}
			sendChannelBrokenNotifier = make(chan *amqp.Error)

			checkTicker.Reset(tickerRetryDuration)
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
			if channelSend == nil {
				cachedSendData = append(cachedSendData, sendD)

				break
			}

			d, _ := json.Marshal(sendD)

			channelID := sendD.ChannelID
			if channelID == "" {
				channelID = sendD.TalkID
			}

			if err = channelSend.Publish(impl.exchangeName(channelID), "", false, false,
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

func (impl *rabbitMQImpl) trackTalkSetup(talkID string, brokenNotifier chan *amqp.Error, logger l.Wrapper) (deliveries <-chan amqp.Delivery, err error) {
	connW, ok := impl.conn.Load().(connWrapper)
	if !ok {
		logger.Error("LoadConnWrapperFailed")

		err = commerr.ErrInternal

		return
	}

	if connW.conn == nil {
		logger.Error("NoConnection")

		err = commerr.ErrNotFound

		return
	}

	channel, err := connW.conn.Channel()
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("GetChannelFailed")

		return
	}

	err = channel.ExchangeDeclare(impl.exchangeName(talkID), "fanout", false, true,
		false, false, nil)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("ExchangeDeclareFailed")

		return
	}

	q, err := channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("QueueDeclareFailed")

		return
	}

	err = channel.QueueBind(q.Name, "", impl.exchangeName(talkID), false, nil)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("QueueBindFailed")

		return
	}

	deliveries, err = channel.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("ConsumeFailed")

		return
	}

	channel.NotifyClose(brokenNotifier)

	return
}

func (impl *rabbitMQImpl) mqErrorDesc(err *amqp.Error) string {
	if err == nil {
		return "noError"
	}

	return fmt.Sprintf("Code:%d, Reason:%s, Server:%t, Recover:%t", err.Code, err.Reason, err.Server, err.Recover)
}

func (impl *rabbitMQImpl) trackTalkRoutine(ctx context.Context, talkID string, start chan<- error) {
	logger := impl.logger.WithFields(l.StringField("talkID", talkID), l.StringField(l.RoutineKey,
		"trackTalkRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	if start != nil {
		start <- nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	select {
	case impl.chTalkTrackStartedEvent <- &talkTrackStartedEventData{
		talkID:    talkID,
		ctxCancel: cancel,
	}:
	default:
	}

	checkTicker := time.NewTicker(tickerRetryDuration)

	var deliveries <-chan amqp.Delivery

	brokenNotifier := make(chan *amqp.Error)
	var err error

	loop := true

	for loop {
		select {
		case <-ctx.Done():
			loop = false

			continue
		case <-checkTicker.C:
			if deliveries != nil {
				break
			}

			checkTicker.Reset(tickerCheckDuration)

			logger.Info("StartTrackTalkSetup")
			deliveries, err = impl.trackTalkSetup(talkID, brokenNotifier, logger)
			if err != nil {
				checkTicker.Reset(tickerRetryDuration)

				break
			}

			logger.Info("SuccessTrackTalkSetup")
		case mqError := <-brokenNotifier:
			logger.WithFields(l.StringField("desc", impl.mqErrorDesc(mqError))).Error("channelBroken")

			brokenNotifier = make(chan *amqp.Error)

			deliveries = nil
			checkTicker.Reset(tickerRetryDuration)
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
				if impl.servicerOb != nil {
					impl.servicerOb.OnTalkCreate(obj.TalkID)
				}
			} else if obj.ServicerAttach != nil {
				if impl.servicerOb != nil {
					impl.servicerOb.OnServicerAttachMessage(obj.TalkID, obj.ServicerAttach.ServicerID)
				}
			} else if obj.ServicerDetach != nil {
				if impl.servicerOb != nil {
					impl.servicerOb.OnServicerDetachMessage(obj.TalkID, obj.ServicerDetach.ServicerID)
				}
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
