package server

import (
	"context"
	"fmt"
	"time"

	"github.com/godruoyi/go-snowflake"
	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-be/internal/controller"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-be/internal/impls"
	"github.com/sbasestarter/customer-service-be/internal/model"
	"github.com/sbasestarter/customer-service-be/internal/user"
	"github.com/sbasestarter/customer-service-be/internal/vo"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server interface {
	customertalkpb.CustomerTalkServiceServer
	customertalkpb.ServiceTalkServiceServer
}

func NewServer(userCenter user.Center, logger l.Wrapper) Server {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	m := impls.NewModelEx(model.NewMongoModel(&config.GetConfig().MongoConfig, logger))

	return &serverImpl{
		logger:     logger,
		userCenter: userCenter,
		model:      m,
		controller: controller.NewController(m, logger),
	}
}

type serverImpl struct {
	customertalkpb.UnimplementedCustomerTalkServiceServer
	customertalkpb.UnimplementedServiceTalkServiceServer

	logger     l.Wrapper
	userCenter user.Center
	model      defs.ModelEx

	controller *controller.Controller
}

func (impl *serverImpl) statusError(c codes.Code, err error) error {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	return impl.messageError(c, errMsg)
}

func (impl *serverImpl) messageError(c codes.Code, msg string) error {
	return status.Error(c, msg)
}

func (impl *serverImpl) QueryTalks(ctx context.Context, request *customertalkpb.QueryTalksRequest) (*customertalkpb.QueryTalksResponse, error) {
	u, err := impl.userCenter.ExtractUserInfoFromGRPCContext(ctx)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("ExtractUserInfoFromGRPCContextFailed")

		return nil, impl.statusError(codes.Unauthenticated, nil)
	}

	talkInfos, err := impl.model.QueryTalks(ctx, u.ID, 0, "", vo.TaskStatusesMapPb2Db(request.GetStatuses()))
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("QueryTalksFailed")

		return nil, impl.statusError(codes.Internal, err)
	}

	return &customertalkpb.QueryTalksResponse{Talks: vo.TalkInfoRsDB2Pb(talkInfos)}, nil
}

func (impl *serverImpl) Talk(server customertalkpb.CustomerTalkService_TalkServer) error {
	if server == nil {
		impl.logger.Error("noServerStream")

		return impl.messageError(codes.InvalidArgument, "noServerStream")
	}

	u, err := impl.userCenter.ExtractUserInfoFromGRPCContext(server.Context())
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("ExtractUserInfoFromGRPCContextFailed")

		return impl.statusError(codes.Unauthenticated, nil)
	}

	uniqueID := snowflake.ID()

	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "Talk"),
		l.StringField("u", fmt.Sprintf("%d:%s", u.ID, u.UserName)),
		l.UInt64Field("uniqueID", uniqueID))

	logger.Debug("enter")

	defer func() {
		logger.Debugf("leave")
	}()

	request, err := server.Recv()
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("ReceiveOpMessageFailed")

		return impl.statusError(codes.Unknown, err)
	}

	talkID, createTalkFlag, err := impl.handleTalkStart(server.Context(), u.ID, request)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("handleTalkStartFailed")

		return err
	}

	logger = logger.WithFields(l.StringField("talkID", talkID))

	chSendMessage := make(chan *customertalkpb.TalkResponse, 100)

	customer := controller.NewCustomer(uniqueID, talkID, createTalkFlag, u.ID, chSendMessage)

	err = impl.controller.InstallCustomer(customer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("InstallCustomerFailed")

		return err
	}

	chTerminal := make(chan error, 2)

	go impl.customerReceiveRoutine(server, customer, u.ID, chTerminal, logger)

	loop := true

	for loop {
		select {
		case <-chTerminal:
			loop = false

			continue
		case message := <-chSendMessage:
			err = server.Send(message)

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("SendMessageToStreamFailed")

				loop = false

				continue
			}

			if message.GetKickOut() != nil {
				logger.WithFields(l.ErrorField(err)).Error("KickOut")

				loop = false

				continue
			}
		}
	}

	err = impl.controller.UninstallCustomer(customer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("UninstallCustomerFailed")

		return err
	}

	return nil
}

func (impl *serverImpl) customerReceiveRoutine(server customertalkpb.CustomerTalkService_TalkServer,
	customer defs.Customer, userID uint64, chTerminal chan<- error, logger l.Wrapper) {
	var err error

	var request *customertalkpb.TalkRequest

	defer func() {
		chTerminal <- err
	}()

	for {
		request, err = server.Recv()
		if err != nil {
			break
		}

		if message := request.GetMessage(); message != nil {
			dbMessage := vo.TalkMessageWPb2Db(message)
			dbMessage.At = time.Now().Unix()
			dbMessage.CustomerMessage = true
			dbMessage.SenderID = userID

			err = impl.model.AddTalkMessage(server.Context(), customer.GetTalkID(), dbMessage)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("AddTalkMessageFailed")

				continue
			}

			err = impl.controller.CustomerMessageIncoming(customer, message.SeqId, dbMessage)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("CustomerMessageIncomingFailed")

				break
			}
		} else if talkClose := request.GetClose(); talkClose != nil {
			err = impl.controller.CustomerClose(customer)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("CustomerCloseFailed")

				break
			}
		} else {
			logger.Error("ReceivedUnknownMessage")
		}
	}
}

func (impl *serverImpl) handleTalkStart(ctx context.Context, userID uint64,
	request *customertalkpb.TalkRequest) (talkID string, talkCreateFlag bool, err error) {
	if request == nil {
		err = impl.messageError(codes.InvalidArgument, "noRequest")

		return
	}

	if request.GetCreate() != nil {
		if request.GetCreate().GetTitle() == "" {
			err = impl.messageError(codes.InvalidArgument, "noTitle")

			return
		}

		talkID, err = impl.model.CreateTalk(ctx, &defs.TalkInfoW{
			Status:    defs.TalkStatusOpened,
			Title:     request.GetCreate().GetTitle(),
			StartAt:   time.Now().Unix(),
			CreatorID: userID,
		})

		talkCreateFlag = true

		return
	}

	if request.GetOpen() != nil {
		talkID = request.GetOpen().GetTalkId()

		err = impl.model.OpenTalk(ctx, talkID)

		return
	}

	err = impl.messageError(codes.InvalidArgument, "invalidRequest")

	return
}

func (impl *serverImpl) Service(server customertalkpb.ServiceTalkService_ServiceServer) error {
	if server == nil {
		impl.logger.Error("noServerStream")

		return impl.messageError(codes.InvalidArgument, "noServerStream")
	}

	u, err := impl.userCenter.ExtractUserInfoFromGRPCContext(server.Context())
	if err != nil {
		return impl.statusError(codes.Unauthenticated, err)
	}

	uniqueID := snowflake.ID()

	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "Service"),
		l.StringField("u", fmt.Sprintf("%d:%s", u.ID, u.UserName)),
		l.UInt64Field("uniqueID", uniqueID))

	logger.Debug("enter")

	defer func() {
		logger.Debugf("leave")
	}()

	chSendMessage := make(chan *customertalkpb.ServiceResponse, 100)

	servicer := controller.NewServicer(u.ID, uniqueID, chSendMessage)

	err = impl.controller.InstallServicer(servicer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("InstallServicerFailed")

		return err
	}

	chTerminal := make(chan error, 2)

	go impl.serverReceiveRoutine(server, servicer, u.ID, chTerminal, logger)

	loop := true

	for loop {
		select {
		case <-chTerminal:
			loop = false

			continue
		case message := <-chSendMessage:
			err = server.Send(message)

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("SendMessageToStreamFailed")

				loop = false

				continue
			}

			if message.GetKickOut() != nil {
				logger.WithFields(l.ErrorField(err)).Error("KickOut")

				loop = false

				continue
			}
		}
	}

	err = impl.controller.UninstallServicer(servicer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("UninstallServicerFailed")

		return err
	}

	return nil
}

func (impl *serverImpl) serverReceiveRoutine(server customertalkpb.ServiceTalkService_ServiceServer,
	servicer defs.Servicer, userID uint64, chTerminal chan<- error, logger l.Wrapper) {
	var err error

	var request *customertalkpb.ServiceRequest

	defer func() {
		chTerminal <- err
	}()

	for {
		request, err = server.Recv()
		if err != nil {
			break
		}

		if request.GetAttachedTalks() != nil {
			err = impl.controller.ServicerQueryAttachedTalks(servicer)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("ServicerQueryAttachedTalksFailed")

				continue
			}
		} else if request.GetPendingTalks() != nil {
			err = impl.controller.ServicerQueryPendingTalks(servicer)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("ServicerQueryPendingTalks")

				continue
			}
		} else if reload := request.GetReload(); reload != nil {
			err = impl.controller.ServicerReloadTalk(servicer, reload.GetTalkId())
			if err != nil {
				logger.WithFields(l.ErrorField(err), l.StringField("talkID", reload.GetTalkId())).
					Error("ServicerReloadTalk")

				continue
			}
		} else if message := request.GetMessage(); message != nil {
			dbMessage := vo.TalkMessageWPb2Db(message.GetMessage())
			dbMessage.At = time.Now().Unix()
			dbMessage.CustomerMessage = false
			dbMessage.SenderID = userID

			err = impl.model.AddTalkMessage(server.Context(), message.GetTalkId(), dbMessage)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("AddTalkMessageFailed")

				break
			}

			var seqID uint64
			if message.GetMessage() != nil {
				seqID = message.GetMessage().GetSeqId()
			}

			err = impl.controller.ServicerMessageIncoming(servicer, seqID, message.GetTalkId(), dbMessage)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("CustomerMessageIncomingFailed")

				continue
			}
		} else if attach := request.GetAttach(); attach != nil {
			err = impl.controller.ServicerAttachTalk(servicer, attach.GetTalkId())

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("AttachTalkFailed")

				continue
			}
		} else if detach := request.GetDetach(); detach != nil {
			err = impl.controller.ServicerDetachTalk(servicer, detach.GetTalkId())

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("DetachTalkFailed")

				continue
			}
		} else {
			logger.Error("unknownMessage")
		}
	}
}
