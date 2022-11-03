package impls

import (
	"context"
	"fmt"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-be/internal/vo"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
)

type MainRoutineRunner interface {
	Post(func())
}

func NewMD(mrRunner MainRoutineRunner, mdi defs.MDI, logger l.Wrapper) defs.MD {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	impl := &mdImpl{
		mrRunner:  mrRunner,
		mdi:       mdi,
		logger:    logger,
		customers: make(map[string]map[uint64]defs.Customer),
		servicers: make(map[uint64]map[uint64]defs.Servicer),
	}

	mdi.SetObserver(impl)

	return impl
}

type mdImpl struct {
	mrRunner MainRoutineRunner
	mdi      defs.MDI
	logger   l.Wrapper

	customers map[string]map[uint64]defs.Customer // talkID - customerN - customer
	servicers map[uint64]map[uint64]defs.Servicer // servicerID - servicerN - servicer
}

//
// defs.Observer
//

func (impl *mdImpl) OnMessageIncoming(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	impl.mrRunner.Post(func() {
		impl.sendResponseToCustomers(senderUniqueID, talkID, &customertalkpb.TalkResponse{
			Talk: &customertalkpb.TalkResponse_Message{
				Message: vo.TalkMessageDB2Pb(message),
			},
		})

		impl.sendResponseToServicersForTalk(senderUniqueID, talkID, &customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Message{
				Message: &customertalkpb.ServiceTalkMessageResponse{
					TalkId:  talkID,
					Message: vo.TalkMessageDB2Pb(message),
				},
			},
		})
	})
}

func (impl *mdImpl) OnTalkClose(talkID string) {
	impl.mrRunner.Post(func() {
		impl.sendResponseToCustomers(0, talkID, &customertalkpb.TalkResponse{
			Talk: &customertalkpb.TalkResponse_Close{
				Close: &customertalkpb.TalkClose{},
			},
		})

		resp := &customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Close{
				Close: &customertalkpb.ServiceTalkClose{},
			},
		}
		impl.send4AllServicers(func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

func (impl *mdImpl) send4AllServicers(do func(defs.Servicer) error) {
	for servicerID, ss := range impl.servicers {
		for uniqueID, servicer := range ss {
			if err := do(servicer); err != nil {
				servicer.Remove("SendFailed")

				delete(ss, uniqueID)
			}
		}
		if len(ss) == 0 {
			delete(impl.servicers, servicerID)
		}
	}
}

func (impl *mdImpl) OnServicerAttachMessage(talkID string, servicerID uint64) {
	impl.mrRunner.Post(func() {
		talkInfo, err := impl.mdi.GetM().GetTalkInfo(context.TODO(), talkID)
		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

			return
		}

		resp := &customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Attach{
				Attach: &customertalkpb.ServiceAttachTalkResponse{
					Talk:              vo.TalkInfoRDb2Pb(talkInfo),
					AttachedServiceId: servicerID,
				},
			},
		}
		impl.send4AllServicers(func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

func (impl *mdImpl) OnServicerDetachMessage(talkID string, servicerID uint64) {
	talkInfo, err := impl.mdi.GetM().GetTalkInfo(context.TODO(), talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

		return
	}

	impl.mrRunner.Post(func() {
		resp := &customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Detach{
				Detach: &customertalkpb.ServiceDetachTalkResponse{
					Talk:              vo.TalkInfoRDb2Pb(talkInfo),
					DetachedServiceId: servicerID,
				},
			},
		}
		impl.send4AllServicers(func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

//
// defs.MD
//

func (impl *mdImpl) Load(ctx context.Context) (err error) {
	return impl.mdi.Load(ctx)
}

func (impl *mdImpl) InstallCustomer(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	ok, err := impl.mdi.GetM().TalkExists(ctx, customer.GetTalkID())
	if err != nil {
		impl.logger.WithFields(l.StringField("talkID", customer.GetTalkID()), l.ErrorField(err)).
			Error("TalkNotExistsFailed")

		customer.Remove("TalkNotExists:" + err.Error())

		return
	}

	if !ok {
		impl.logger.WithFields(l.StringField("talkID", customer.GetTalkID())).Error("TalkNotExists")

		customer.Remove("TalkNotExists")

		return
	}

	err = impl.mdi.AddTrackTalk(ctx, customer.GetTalkID())
	if err != nil {
		impl.logger.WithFields(l.StringField("talkID", customer.GetTalkID()), l.ErrorField(err)).
			Error("AddTrackTalkFailed")

		customer.Remove("AddTrackTalkFailed:" + err.Error())

		return
	}

	if _, ok = impl.customers[customer.GetTalkID()]; !ok {
		impl.customers[customer.GetTalkID()] = make(map[uint64]defs.Customer)
	}

	impl.customers[customer.GetTalkID()][customer.GetUniqueID()] = customer

	if customer.CreateTalkFlag() {
		impl.mdi.SendServiceDetachMessage(customer.GetTalkID(), 0)
	}

	go func(logger l.Wrapper) {
		messages, errG := impl.mdi.GetM().GetTalkMessages(context.TODO(), customer.GetTalkID(), 0, 0)
		if errG != nil {
			logger.WithFields(l.ErrorField(errG)).Error("GetTalkMessageFailed")

			return
		}

		var pbMessages []*customertalkpb.TalkMessage

		for _, message := range messages {
			pbMessages = append(pbMessages, vo.TalkMessageDB2Pb(&message.TalkMessageW))
		}

		if errS := customer.SendMessage(&customertalkpb.TalkResponse{
			Talk: &customertalkpb.TalkResponse_Messages{
				Messages: &customertalkpb.TalkMessages{
					Messages: pbMessages,
				},
			},
		}); errS != nil {
			logger.WithFields(l.ErrorField(errS)).Error("SendMessageFailed")
		}
	}(impl.logger.WithFields(l.StringField("customer", fmt.Sprintf("%s-%d", customer.GetTalkID(),
		customer.GetUniqueID()))))
}

func (impl *mdImpl) UninstallCustomer(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	impl.mdi.RemoveTrackTalk(ctx, customer.GetTalkID())

	if talkCustomers, ok := impl.customers[customer.GetTalkID()]; ok {
		delete(talkCustomers, customer.GetUniqueID())

		if len(talkCustomers) == 0 {
			delete(impl.customers, customer.GetTalkID())
		}
	}
}

func (impl *mdImpl) CustomerMessageIncoming(_ context.Context, customer defs.Customer, seqID uint64, message *defs.TalkMessageW) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	if message == nil {
		impl.logger.Error("noMessage")

		return
	}

	customersMap := impl.customers[customer.GetTalkID()]

	if err := customer.SendMessage(&customertalkpb.TalkResponse{
		Talk: &customertalkpb.TalkResponse_MessageConfirmed{
			MessageConfirmed: &customertalkpb.TalkMessageConfirmed{
				SeqId: seqID,
				At:    uint64(message.At),
			},
		},
	}); err != nil {
		impl.logger.WithFields(l.ErrorField(err), l.UInt64Field("id", customer.GetUniqueID())).
			Error("SendMessageFailed")

		customer.Remove("sendMessageFailed")

		delete(customersMap, customer.GetUniqueID())
	}

	if len(customersMap) == 0 {
		delete(impl.customers, customer.GetTalkID())
	}

	impl.mdi.SendMessage(customer.GetUniqueID(), customer.GetTalkID(), message)
}

func (impl *mdImpl) CustomerClose(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	if err := impl.mdi.GetM().CloseTalk(ctx, customer.GetTalkID()); err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("CloseTalkFailed")

		return
	}

	impl.mdi.SendTalkCloseMessage(customer.GetTalkID())
}

func (impl *mdImpl) InstallServicer(ctx context.Context, servicer defs.Servicer) {
	if servicer == nil {
		impl.logger.Error("noServicer")

		return
	}

	_ = impl.sendAttachedTalks(ctx, servicer)
	_ = impl.sendPendingTalks(ctx, servicer)

	if _, ok := impl.servicers[servicer.GetUserID()]; !ok {
		impl.servicers[servicer.GetUserID()] = make(map[uint64]defs.Servicer)
	}

	impl.servicers[servicer.GetUserID()][servicer.GetUniqueID()] = servicer
}

func (impl *mdImpl) UninstallServicer(_ context.Context, servicer defs.Servicer) {
	talkServicers, ok := impl.servicers[servicer.GetUserID()]
	if !ok {
		impl.logger.Warn("noUserService")

		return
	}

	delete(talkServicers, servicer.GetUniqueID())

	if len(talkServicers) == 0 {
		delete(impl.servicers, servicer.GetUserID())
	}
}

func (impl *mdImpl) ServicerAttachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
	if talkID == "" || servicer == nil {
		impl.logger.WithFields(l.StringField("talkID", talkID)).Error("noServicerOrTalkID")

		return
	}

	servicerID, err := impl.mdi.GetM().GetTalkServicerID(ctx, talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID > 0 {
		if servicerID == servicer.GetUserID() {
			impl.logger.WithFields(l.StringField("talkID", talkID),
				l.UInt64Field("userID", servicer.GetUserID())).Warn("talkAlreadyAttached")

			return
		}
	}

	err = impl.mdi.GetM().UpdateTalkServiceID(ctx, talkID, servicer.GetUserID())
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("UpdateTalkServiceID")
	}

	impl.mdi.SendServicerAttachMessage(talkID, servicer.GetUserID())

	talk, err := impl.getTalkInfoWithMessages(ctx, talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("getTalkInfoWithMessagesFailed")

		return
	}

	err = servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_Reload{
			Reload: &customertalkpb.ServiceTalkReloadResponse{
				Talk: talk,
			},
		},
	})
	if err != nil {
		impl.logger.Error("SendMessageFailed")
	}
}

func (impl *mdImpl) ServicerDetachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
	if talkID == "" || servicer == nil {
		impl.logger.WithFields(l.StringField("talkID", talkID)).Error("noServicerOrTalkID")

		return
	}

	servicerID, err := impl.mdi.GetM().GetTalkServicerID(ctx, talkID)
	if err != nil {
		impl.logger.WithFields(l.StringField("talkID", talkID)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID != servicer.GetUserID() {
		if err = servicer.SendMessage(&customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Notify{
				Notify: &customertalkpb.ServiceTalkNotifyResponse{
					Msg: "talkNotAttached",
				},
			},
		}); err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

			return
		}

		return
	}

	if err = impl.mdi.GetM().UpdateTalkServiceID(ctx, talkID, 0); err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("UpdateTalkServiceID")
	}

	impl.mdi.SendServiceDetachMessage(talkID, servicer.GetUserID())
}

func (impl *mdImpl) ServicerQueryAttachedTalks(ctx context.Context, servicer defs.Servicer) {
	_ = impl.sendAttachedTalks(ctx, servicer)
}

func (impl *mdImpl) ServicerQueryPendingTalks(ctx context.Context, servicer defs.Servicer) {
	_ = impl.sendPendingTalks(ctx, servicer)
}

func (impl *mdImpl) ServicerReloadTalk(ctx context.Context, servicer defs.Servicer, talkID string) {
	talk, err := impl.getTalkInfoWithMessages(ctx, talkID)
	if err != nil {
		return
	}

	if err = servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_Reload{
			Reload: &customertalkpb.ServiceTalkReloadResponse{
				Talk: talk,
			},
		},
	}); err != nil {
		impl.logger.Error("SendMessageFailed")
	}
}

func (impl *mdImpl) ServiceMessage(ctx context.Context, servicer defs.Servicer, talkID string, seqID uint64, message *defs.TalkMessageW) {
	if servicer == nil || talkID == "" || message == nil {
		impl.logger.Error("nilParameters")

		return
	}

	servicerID, err := impl.mdi.GetM().GetTalkServicerID(ctx, talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID != servicer.GetUserID() {
		impl.logger.WithFields(l.StringField("talkID", talkID), l.UInt64Field("curServicerID", servicer.GetUserID()),
			l.UInt64Field("talkServicerID", servicerID)).Error("invalidServicerID")

		return
	}

	servicersMap := impl.servicers[servicer.GetUserID()]

	if err = servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_MessageConfirmed{
			MessageConfirmed: &customertalkpb.ServiceMessageConfirmed{
				SeqId: seqID,
				At:    uint64(message.At),
			},
		},
	}); err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

		servicer.Remove("SendMessageFailed")

		delete(servicersMap, servicer.GetUniqueID())
	}

	if len(servicersMap) == 0 {
		delete(impl.servicers, servicer.GetUserID())
	}

	impl.mdi.SendMessage(servicer.GetUniqueID(), talkID, message)
}

//
//
//

func (impl *mdImpl) sendAttachedTalks(ctx context.Context, servicer defs.Servicer) (err error) {
	talkInfos, err := impl.mdi.GetM().GetServicerTalkInfos(ctx, servicer.GetUserID())
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetServicerTalkInfosFailed")

		return
	}

	talks := make([]*customertalkpb.ServiceTalkInfoAndMessages, 0, len(talkInfos))

	for _, talkInfo := range talkInfos {
		talkMessages, _ := impl.mdi.GetM().GetTalkMessages(ctx, talkInfo.TalkID, 0, 0)
		if len(talkMessages) <= 0 {
			continue
		}

		talks = append(talks, &customertalkpb.ServiceTalkInfoAndMessages{
			TalkInfo: vo.TalkInfoRDb2Pb(talkInfo),
			Messages: vo.TalkMessagesRDb2Pb(talkMessages),
		})
	}

	err = servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_Talks{
			Talks: &customertalkpb.ServiceAttachedTalksResponse{
				Talks: talks,
			},
		},
	})
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}

	return
}

func (impl *mdImpl) sendPendingTalks(ctx context.Context, servicer defs.Servicer) (err error) {
	talkInfos, err := impl.mdi.GetM().GetPendingTalkInfos(ctx)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetPendingTalkInfosFailed")

		return
	}

	err = servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_PendingTalks{
			PendingTalks: &customertalkpb.ServicePendingTalksResponse{
				Talks: vo.TalkInfoRsDB2Pb(talkInfos),
			},
		},
	})
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}

	return
}

func (impl *mdImpl) getTalkInfoWithMessages(ctx context.Context, talkID string) (*customertalkpb.ServiceTalkInfoAndMessages, error) {
	talkInfo, err := impl.mdi.GetM().GetTalkInfo(ctx, talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

		return nil, err
	}

	talkMessages, err := impl.mdi.GetM().GetTalkMessages(ctx, talkID, 0, 0)
	if err != nil {
		impl.logger.Error("NoTalkIDMessage")

		return nil, err
	}

	return &customertalkpb.ServiceTalkInfoAndMessages{
		TalkInfo: vo.TalkInfoRDb2Pb(talkInfo),
		Messages: vo.TalkMessagesRDb2Pb(talkMessages),
	}, nil
}

func (impl *mdImpl) sendResponseToCustomers(excludedUniqueID uint64, talkID string, resp *customertalkpb.TalkResponse) {
	customersMap := impl.customers[talkID]

	for _, talkCustomer := range customersMap {
		if talkCustomer.GetUniqueID() == excludedUniqueID {
			continue
		}

		if err := talkCustomer.SendMessage(resp); err != nil {
			impl.logger.WithFields(l.ErrorField(err), l.UInt64Field("id", talkCustomer.GetUniqueID())).
				Error("SendMessageFailed")

			talkCustomer.Remove("sendMessageFailed")

			delete(customersMap, talkCustomer.GetUniqueID())
		}
	}

	if len(customersMap) == 0 {
		delete(impl.customers, talkID)
	}
}

func (impl *mdImpl) sendResponseToServicersForTalk(excludedUniqueID uint64, talkID string, resp *customertalkpb.ServiceResponse) {
	servicerID, err := impl.mdi.GetM().GetTalkServicerID(context.TODO(), talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID == 0 {
		return
	}

	servicersMap := impl.servicers[servicerID]

	for _, servicer := range servicersMap {
		if servicer.GetUniqueID() == excludedUniqueID {
			continue
		}

		if err = servicer.SendMessage(resp); err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

			servicer.Remove("SendMessageFailed")

			delete(servicersMap, servicer.GetUniqueID())
		}
	}

	if len(servicersMap) == 0 {
		delete(impl.servicers, servicerID)
	}
}
