package controller

import (
	"context"

	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-be/internal/impls"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libeasygo/commerr"
	"github.com/sgostarter/libeasygo/routineman"
)

const (
	defMaxCache        = 10
	defMaxMessageCache = 100
)

type customerMessage struct {
	customer defs.Customer
	seqID    uint64
	message  *defs.TalkMessageW
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

type Controller struct {
	m          defs.ModelEx
	logger     l.Wrapper
	routineMan routineman.RoutineMan

	chInstallCustomer   chan defs.Customer
	chUninstallCustomer chan defs.Customer
	chCustomerMessage   chan *customerMessage
	chCustomerClose     chan defs.Customer

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

func NewController(m defs.ModelEx, logger l.Wrapper) *Controller {
	return NewControllerEx(0, 0, m, logger)
}

func NewControllerEx(maxCache, maxMessageCache int, m defs.ModelEx, logger l.Wrapper) *Controller {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	if maxCache <= 0 {
		maxCache = defMaxCache
	}

	if maxMessageCache <= 0 {
		maxMessageCache = defMaxMessageCache
	}

	controller := &Controller{
		m:                            m,
		logger:                       logger,
		routineMan:                   routineman.NewRoutineMan(context.Background(), logger),
		chInstallCustomer:            make(chan defs.Customer, maxCache),
		chUninstallCustomer:          make(chan defs.Customer, maxCache),
		chCustomerClose:              make(chan defs.Customer, maxCache),
		chCustomerMessage:            make(chan *customerMessage, maxMessageCache),
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

//
//
//

func (c *Controller) Post(f func()) {
	if f == nil {
		return
	}
	select {
	case c.chMainRoutineRunner <- f:
	default:
	}
}

//
//
//

func (c *Controller) InstallCustomer(customer defs.Customer) error {
	if customer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chInstallCustomer <- customer:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *Controller) UninstallCustomer(customer defs.Customer) error {
	if customer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chUninstallCustomer <- customer:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *Controller) CustomerClose(customer defs.Customer) error {
	if customer == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chCustomerClose <- customer:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *Controller) CustomerMessageIncoming(customer defs.Customer, seqID uint64, message *defs.TalkMessageW) error {
	if customer == nil || message == nil {
		return commerr.ErrInvalidArgument
	}

	select {
	case c.chCustomerMessage <- &customerMessage{
		customer: customer,
		seqID:    seqID,
		message:  message,
	}:
	default:
		return commerr.ErrCanceled
	}

	return nil
}

func (c *Controller) ServicerMessageIncoming(servicer defs.Servicer, seqID uint64, talkID string, message *defs.TalkMessageW) error {
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

func (c *Controller) InstallServicer(servicer defs.Servicer) error {
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

func (c *Controller) UninstallServicer(servicer defs.Servicer) error {
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

func (c *Controller) ServicerAttachTalk(servicer defs.Servicer, talkID string) error {
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

func (c *Controller) ServicerDetachTalk(servicer defs.Servicer, talkID string) error {
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

func (c *Controller) ServicerQueryAttachedTalks(servicer defs.Servicer) error {
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

func (c *Controller) ServicerQueryPendingTalks(servicer defs.Servicer) error {
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

func (c *Controller) ServicerReloadTalk(servicer defs.Servicer, talkID string) error {
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

//
//
//

func (c *Controller) init() {
	c.routineMan.StartRoutine(c.mainRoutine, "mainRoutine")
}

func (c *Controller) mainRoutine(ctx context.Context, exiting func() bool) {
	logger := c.logger.WithFields(l.StringField(l.RoutineKey, "mainRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	// mdi := impls.NewMemMDI(c.m, logger)
	mdi := impls.NewRabbitMQMDI(config.GetConfig().RabbitMQURL, c.m, logger)
	md := impls.NewMD(c, mdi, c.logger)
	// md := impls.NewMainData(c.m, c.logger)

	err := md.Load(ctx)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Fatal("LoadFailed")

		return
	}

	for !exiting() {
		select {
		case <-ctx.Done():
			continue
		case customer := <-c.chInstallCustomer:
			md.InstallCustomer(ctx, customer)
		case customer := <-c.chUninstallCustomer:
			md.UninstallCustomer(ctx, customer)
		case msgD := <-c.chCustomerMessage:
			md.CustomerMessageIncoming(ctx, msgD.customer, msgD.seqID, msgD.message)
		case customer := <-c.chCustomerClose:
			md.CustomerClose(ctx, customer)
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
