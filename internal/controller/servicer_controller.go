package controller

import (
	"context"
	"github.com/sgostarter/libeasygo/commerr"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libeasygo/routineman"
)

func NewServicerController(md defs.ServicerMD, m defs.ModelEx, logger l.Wrapper) *ServicerController {
	return NewServicerControllerEx(0, 0, md, m, logger)
}

func NewServicerControllerEx(maxCache, maxMessageCache int, md defs.ServicerMD, m defs.ModelEx, logger l.Wrapper) *ServicerController {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	if maxCache <= 0 {
		maxCache = defMaxCache
	}

	if maxMessageCache <= 0 {
		maxMessageCache = defMaxMessageCache
	}

	controller := &ServicerController{
		md:                           md,
		m:                            m,
		logger:                       logger,
		routineMan:                   routineman.NewRoutineMan(context.Background(), logger),
		chInstallServicer:            make(chan defs.Servicer, maxCache),
		chUninstallServicer:          make(chan defs.Servicer, maxCache),
		chServicerAttachTalk:         make(chan *servicerWithTalk, maxCache),
		chServicerDetachTalk:         make(chan *servicerWithTalk, maxCache),
		chServicerQueryAttachedTalks: make(chan defs.Servicer),
		chServicerQueryPendingTalks:  make(chan defs.Servicer),
		chServicerReloadTalk:         make(chan *servicerWithTalk, maxCache),
		chServicerMessage:            make(chan *servicerMessage, maxMessageCache),
		chMainRoutineRunner:          make(chan func(), maxMessageCache),
	}

	controller.init()

	return controller
}

type servicerMessage struct {
	servicer defs.Servicer
	seqID    uint64
	talkID   string
	message  *defs.TalkMessageW
}

type servicerWithTalk struct {
	talkID   string
	servicer defs.Servicer
}

type ServicerController struct {
	md         defs.ServicerMD
	m          defs.ModelEx
	logger     l.Wrapper
	routineMan routineman.RoutineMan

	chInstallServicer            chan defs.Servicer
	chUninstallServicer          chan defs.Servicer
	chServicerAttachTalk         chan *servicerWithTalk
	chServicerDetachTalk         chan *servicerWithTalk
	chServicerQueryAttachedTalks chan defs.Servicer
	chServicerQueryPendingTalks  chan defs.Servicer
	chServicerReloadTalk         chan *servicerWithTalk
	chServicerMessage            chan *servicerMessage
	chMainRoutineRunner          chan func()
}

func (c *ServicerController) Post(f func()) {
	if f == nil {
		return
	}
	select {
	case c.chMainRoutineRunner <- f:
	default:
	}
}

func (c *ServicerController) InstallServicer(servicer defs.Servicer) error {
	if servicer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chInstallServicer <- servicer:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) UninstallServicer(servicer defs.Servicer) error {
	if servicer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chUninstallServicer <- servicer:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) ServicerAttachTalk(servicer defs.Servicer, talkID string) error {
	if servicer == nil || talkID == "" {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chServicerAttachTalk <- &servicerWithTalk{
		servicer: servicer,
		talkID:   talkID,
	}:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) ServicerDetachTalk(servicer defs.Servicer, talkID string) error {
	if servicer == nil || talkID == "" {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chServicerDetachTalk <- &servicerWithTalk{
		servicer: servicer,
		talkID:   talkID,
	}:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) ServicerQueryAttachedTalks(servicer defs.Servicer) error {
	if servicer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chServicerQueryAttachedTalks <- servicer:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) ServicerQueryPendingTalks(servicer defs.Servicer) error {
	if servicer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chServicerQueryPendingTalks <- servicer:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) ServicerReloadTalk(servicer defs.Servicer, talkID string) error {
	if servicer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chServicerReloadTalk <- &servicerWithTalk{
		talkID:   talkID,
		servicer: servicer,
	}:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) ServicerMessageIncoming(servicer defs.Servicer, seqID uint64, talkID string, message *defs.TalkMessageW) error {
	if servicer == nil || talkID == "" || message == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chServicerMessage <- &servicerMessage{
		servicer: servicer,
		seqID:    seqID,
		talkID:   talkID,
		message:  message,
	}:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *ServicerController) init() {
	c.md.Setup(c)
	c.routineMan.StartRoutine(c.mainRoutine, "mainRoutine")
}

func (c *ServicerController) mainRoutine(ctx context.Context, exiting func() bool) {
	logger := c.logger.WithFields(l.StringField(l.RoutineKey, "mainRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	md := c.md

	for !exiting() {
		select {
		case <-ctx.Done():
			continue
		case servicer := <-c.chInstallServicer:
			md.InstallServicer(ctx, servicer)
		case servicer := <-c.chUninstallServicer:
			md.UninstallServicer(ctx, servicer)
		case at := <-c.chServicerAttachTalk:
			md.ServicerAttachTalk(ctx, at.talkID, at.servicer)
		case at := <-c.chServicerDetachTalk:
			md.ServicerDetachTalk(ctx, at.talkID, at.servicer)
		case servicer := <-c.chServicerQueryAttachedTalks:
			md.ServicerQueryAttachedTalks(ctx, servicer)
		case servicer := <-c.chServicerQueryPendingTalks:
			md.ServicerQueryPendingTalks(ctx, servicer)
		case at := <-c.chServicerReloadTalk:
			md.ServicerReloadTalk(ctx, at.servicer, at.talkID)
		case msgD := <-c.chServicerMessage:
			md.ServiceMessage(ctx, msgD.servicer, msgD.talkID, msgD.seqID, msgD.message)
		case runner := <-c.chMainRoutineRunner:
			runner()
		}
	}
}
