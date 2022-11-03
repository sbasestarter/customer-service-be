package controller

import (
	"context"
	"github.com/sgostarter/libeasygo/commerr"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libeasygo/routineman"
)

func NewCustomerController(md defs.CustomerMD, m defs.ModelEx, logger l.Wrapper) *CustomerController {
	return NewCustomerControllerEx(0, 0, md, m, logger)
}

func NewCustomerControllerEx(maxCache, maxMessageCache int, md defs.CustomerMD, m defs.ModelEx, logger l.Wrapper) *CustomerController {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	if maxCache <= 0 {
		maxCache = defMaxCache
	}

	if maxMessageCache <= 0 {
		maxMessageCache = defMaxMessageCache
	}

	controller := &CustomerController{
		md:                  md,
		m:                   m,
		logger:              logger,
		routineMan:          routineman.NewRoutineMan(context.Background(), logger),
		chInstallCustomer:   make(chan defs.Customer, maxCache),
		chUninstallCustomer: make(chan defs.Customer, maxCache),
		chCustomerClose:     make(chan defs.Customer, maxCache),
		chCustomerMessage:   make(chan *customerMessage, maxMessageCache),
		chMainRoutineRunner: make(chan func(), maxMessageCache),
	}

	controller.init()

	return controller
}

type customerMessage struct {
	customer defs.Customer
	seqID    uint64
	message  *defs.TalkMessageW
}

type CustomerController struct {
	md         defs.CustomerMD
	m          defs.ModelEx
	logger     l.Wrapper
	routineMan routineman.RoutineMan

	chInstallCustomer   chan defs.Customer
	chUninstallCustomer chan defs.Customer
	chCustomerMessage   chan *customerMessage
	chCustomerClose     chan defs.Customer

	chMainRoutineRunner chan func()
}

func (c *CustomerController) Post(f func()) {
	if f == nil {
		return
	}
	select {
	case c.chMainRoutineRunner <- f:
	default:
	}
}

func (c *CustomerController) InstallCustomer(customer defs.Customer) error {
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

func (c *CustomerController) UninstallCustomer(customer defs.Customer) error {
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

func (c *CustomerController) CustomerClose(customer defs.Customer) error {
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

func (c *CustomerController) CustomerMessageIncoming(customer defs.Customer, seqID uint64, message *defs.TalkMessageW) error {
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

func (c *CustomerController) init() {
	c.md.Setup(c)
	c.routineMan.StartRoutine(c.mainRoutine, "mainRoutine")
}

func (c *CustomerController) mainRoutine(ctx context.Context, exiting func() bool) {
	logger := c.logger.WithFields(l.StringField(l.RoutineKey, "mainRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	md := c.md

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
		case runner := <-c.chMainRoutineRunner:
			runner()
		}
	}
}
