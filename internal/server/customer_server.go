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
	"github.com/sbasestarter/customer-service-be/internal/vo"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
	"google.golang.org/grpc/codes"
)

func NewCustomerServer(controller *controller.CustomerController, userTokenHelper defs.UserTokenHelper, logger l.Wrapper) customertalkpb.CustomerTalkServiceServer {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	m := impls.NewModelEx(model.NewMongoModel(&config.GetConfig().MongoConfig, logger))

	return &customerServerImpl{
		logger:          logger,
		controller:      controller,
		userTokenHelper: userTokenHelper,
		model:           m,
	}
}

type customerServerImpl struct {
	customertalkpb.UnimplementedCustomerTalkServiceServer

	logger          l.Wrapper
	userTokenHelper defs.UserTokenHelper
	model           defs.ModelEx

	controller *controller.CustomerController
}

func (impl *customerServerImpl) QueryTalks(ctx context.Context, request *customertalkpb.QueryTalksRequest) (*customertalkpb.QueryTalksResponse, error) {
	_, userID, _, err := impl.userTokenHelper.ExtractUserFromGRPCContext(ctx, false)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("ExtractUserInfoFromGRPCContextFailed")

		return nil, gRpcError(codes.Unauthenticated, nil)
	}

	talkInfos, err := impl.model.QueryTalks(ctx, userID, 0, "", vo.TaskStatusesMapPb2Db(request.GetStatuses()))
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("QueryTalksFailed")

		return nil, gRpcError(codes.Internal, err)
	}

	return &customertalkpb.QueryTalksResponse{Talks: vo.TalkInfoRsDB2Pb(talkInfos)}, nil
}

func (impl *customerServerImpl) Talk(server customertalkpb.CustomerTalkService_TalkServer) error {
	if server == nil {
		impl.logger.Error("noServerStream")

		return gRpcMessageError(codes.InvalidArgument, "noServerStream")
	}

	_, userID, userName, err := impl.userTokenHelper.ExtractUserFromGRPCContext(server.Context(), false)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("ExtractUserInfoFromGRPCContextFailed")

		return gRpcError(codes.Unauthenticated, nil)
	}

	uniqueID := snowflake.ID()

	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "Talk"),
		l.StringField("u", fmt.Sprintf("%d:%s", userID, userName)),
		l.UInt64Field("uniqueID", uniqueID))

	logger.Debug("enter")

	defer func() {
		logger.Debugf("leave")
	}()

	request, err := server.Recv()
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("ReceiveOpMessageFailed")

		return gRpcError(codes.Unknown, err)
	}

	talkID, createTalkFlag, err := impl.handleTalkStart(server.Context(), userID, userName, request)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("handleTalkStartFailed")

		return err
	}

	logger = logger.WithFields(l.StringField("talkID", talkID))

	chSendMessage := make(chan *customertalkpb.TalkResponse, 100)

	customer := controller.NewCustomer(uniqueID, talkID, createTalkFlag, userID, chSendMessage)

	err = impl.controller.InstallCustomer(customer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("InstallCustomerFailed")

		return err
	}

	chTerminal := make(chan error, 2)

	go impl.customerReceiveRoutine(server, customer, userID, userName, chTerminal, logger)

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

//
//
//

func (impl *customerServerImpl) handleTalkStart(ctx context.Context, userID uint64, userName string,
	request *customertalkpb.TalkRequest) (talkID string, talkCreateFlag bool, err error) {
	if request == nil {
		err = gRpcMessageError(codes.InvalidArgument, "noRequest")

		return
	}

	if request.GetCreate() != nil {
		if request.GetCreate().GetTitle() == "" {
			err = gRpcMessageError(codes.InvalidArgument, "noTitle")

			return
		}

		talkID, err = impl.model.CreateTalk(ctx, &defs.TalkInfoW{
			Status:          defs.TalkStatusOpened,
			Title:           request.GetCreate().GetTitle(),
			StartAt:         time.Now().Unix(),
			CreatorID:       userID,
			CreatorUserName: userName,
		})

		talkCreateFlag = true

		return
	}

	if request.GetOpen() != nil {
		talkID = request.GetOpen().GetTalkId()

		err = impl.model.OpenTalk(ctx, talkID)

		return
	}

	err = gRpcMessageError(codes.InvalidArgument, "invalidRequest")

	return
}

func (impl *customerServerImpl) customerReceiveRoutine(server customertalkpb.CustomerTalkService_TalkServer,
	customer defs.Customer, userID uint64, userName string, chTerminal chan<- error, logger l.Wrapper) {
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
			dbMessage.SenderUserName = userName

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
