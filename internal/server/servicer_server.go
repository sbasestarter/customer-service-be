package server

import (
	"fmt"
	"time"

	"github.com/godruoyi/go-snowflake"
	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-be/internal/controller"
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-be/internal/impls"
	"github.com/sbasestarter/customer-service-be/internal/model"
	"github.com/sbasestarter/customer-service-be/internal/vo"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
	"google.golang.org/grpc/codes"
)

func NewServicerServer(controller *controller.ServicerController, userTokenHelper defs.UserTokenHelper, logger l.Wrapper) customertalkpb.ServiceTalkServiceServer {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	m := impls.NewModelEx(model.NewMongoModel(&config.GetConfig().MongoConfig, logger))

	return &servicerServerImpl{
		logger:          logger,
		controller:      controller,
		userTokenHelper: userTokenHelper,
		model:           m,
	}
}

type servicerServerImpl struct {
	customertalkpb.UnimplementedServiceTalkServiceServer

	logger          l.Wrapper
	userTokenHelper defs.UserTokenHelper
	model           defs.ModelEx

	controller *controller.ServicerController
}

func (impl *servicerServerImpl) Service(server customertalkpb.ServiceTalkService_ServiceServer) error {
	if server == nil {
		impl.logger.Error("noServerStream")

		return gRpcMessageError(codes.InvalidArgument, "noServerStream")
	}

	_, userID, userName, err := impl.userTokenHelper.ExtractUserFromGRPCContext(server.Context(), false)
	if err != nil {
		return gRpcError(codes.Unauthenticated, err)
	}

	uniqueID := snowflake.ID()

	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "Service"),
		l.StringField("u", fmt.Sprintf("%d:%s", userID, userName)),
		l.UInt64Field("uniqueID", uniqueID))

	logger.Debug("enter")

	defer func() {
		logger.Debugf("leave")
	}()

	chSendMessage := make(chan *customertalkpb.ServiceResponse, 100)

	servicer := controller.NewServicer(userID, uniqueID, chSendMessage)

	err = impl.controller.InstallServicer(servicer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("InstallServicerFailed")

		return err
	}

	chTerminal := make(chan error, 2)

	go impl.serverReceiveRoutine(server, servicer, userID, userName, chTerminal, logger)

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

//
//
//

func (impl *servicerServerImpl) serverReceiveRoutine(server customertalkpb.ServiceTalkService_ServiceServer,
	servicer defs.Servicer, userID uint64, userName string, chTerminal chan<- error, logger l.Wrapper) {
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
			dbMessage.SenderUserName = userName

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
