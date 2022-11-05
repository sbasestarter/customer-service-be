package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-be/internal/vo"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
)

func NewServicerMD(mdi defs.ServicerMDI, logger l.Wrapper) defs.ServicerMD {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	impl := &servicerMDImpl{
		mdi:       mdi,
		logger:    logger,
		servicers: make(map[uint64]map[uint64]defs.Servicer),
	}

	mdi.SetServicerObserver(impl)

	return impl
}

type servicerMDImpl struct {
	mrRunner defs.MainRoutineRunner
	mdi      defs.ServicerMDI
	logger   l.Wrapper

	servicers map[uint64]map[uint64]defs.Servicer // servicerID - servicerN - servicer
}

//
// defs.ServicerObserver
//

func (impl *servicerMDImpl) OnMessageIncoming(senderUniqueID uint64, talkID string, message *defs.TalkMessageW) {
	impl.mrRunner.Post(func() {
		impl.sendResponseToServicersForTalk(0, talkID, &customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Message{
				Message: &customertalkpb.ServiceTalkMessageResponse{
					TalkId:  talkID,
					Message: vo.TalkMessageDB2Pb(message),
				},
			},
		})
	})
}

func (impl *servicerMDImpl) OnTalkCreate(talkID string) {
	impl.mrRunner.Post(func() {
		talkInfo, err := impl.mdi.GetM().GetTalkInfo(context.TODO(), talkID)
		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

			return
		}

		resp := &customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Detach{ // FIXME use create message?
				Detach: &customertalkpb.ServiceDetachTalkResponse{
					Talk: vo.TalkInfoRDb2Pb(talkInfo),
				},
			},
		}
		impl.send4AllServicers(func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

func (impl *servicerMDImpl) OnTalkClose(talkID string) {
	impl.mrRunner.Post(func() {
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

func (impl *servicerMDImpl) OnServicerAttachMessage(talkID string, servicerID uint64) {
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

		talkWithMessages, err := impl.getTalkInfoWithMessages(context.TODO(), talkID)
		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("getTalkInfoWithMessagesFailed")

			return
		}

		resp = &customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Reload{
				Reload: &customertalkpb.ServiceTalkReloadResponse{
					Talk: talkWithMessages,
				},
			},
		}

		impl.send4AllOneServicer(servicerID, func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

func (impl *servicerMDImpl) OnServicerDetachMessage(talkID string, servicerID uint64) {
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
// defs.ServicerMD
//

func (impl *servicerMDImpl) Setup(mr defs.MainRoutineRunner) {
	impl.mrRunner = mr
}

func (impl *servicerMDImpl) InstallServicer(ctx context.Context, servicer defs.Servicer) {
	if servicer == nil {
		impl.logger.Error("noServicer")

		return
	}

	talkIDs, _ := impl.sendAttachedTalks(ctx, servicer)
	for _, talkID := range talkIDs {
		_ = impl.mdi.AddTrackTalk(ctx, talkID)
	}

	_ = impl.sendPendingTalks(ctx, servicer)

	if _, ok := impl.servicers[servicer.GetUserID()]; !ok {
		impl.servicers[servicer.GetUserID()] = make(map[uint64]defs.Servicer)
	}

	impl.servicers[servicer.GetUserID()][servicer.GetUniqueID()] = servicer
}

func (impl *servicerMDImpl) UninstallServicer(ctx context.Context, servicer defs.Servicer) {
	talkServicers, ok := impl.servicers[servicer.GetUserID()]
	if !ok {
		impl.logger.Warn("noUserService")

		return
	}

	delete(talkServicers, servicer.GetUniqueID())

	if len(talkServicers) == 0 {
		delete(impl.servicers, servicer.GetUserID())

		talkInfos, _ := impl.mdi.GetM().GetServicerTalkInfos(ctx, servicer.GetUserID())
		for _, info := range talkInfos {
			impl.mdi.RemoveTrackTalk(context.TODO(), info.TalkID)
		}
	}
}

func (impl *servicerMDImpl) ServicerAttachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
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
}

func (impl *servicerMDImpl) ServicerDetachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
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

func (impl *servicerMDImpl) ServicerQueryAttachedTalks(ctx context.Context, servicer defs.Servicer) {
	_, _ = impl.sendAttachedTalks(ctx, servicer)
}

func (impl *servicerMDImpl) ServicerQueryPendingTalks(ctx context.Context, servicer defs.Servicer) {
	_ = impl.sendPendingTalks(ctx, servicer)
}

func (impl *servicerMDImpl) ServicerReloadTalk(ctx context.Context, servicer defs.Servicer, talkID string) {
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
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}
}

func (impl *servicerMDImpl) ServiceMessage(ctx context.Context, servicer defs.Servicer, talkID string, seqID uint64, message *defs.TalkMessageW) {
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

	impl.mdi.SendMessage(servicer.GetUniqueID(), talkID, message)
}

//
//
//

func (impl *servicerMDImpl) sendAttachedTalks(ctx context.Context, servicer defs.Servicer) (talkIDs []string, err error) {
	talkInfos, err := impl.mdi.GetM().GetServicerTalkInfos(ctx, servicer.GetUserID())
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetServicerTalkInfosFailed")

		return
	}

	talks := make([]*customertalkpb.ServiceTalkInfoAndMessages, 0, len(talkInfos))

	for _, talkInfo := range talkInfos {
		talkMessages, _ := impl.mdi.GetM().GetTalkMessages(ctx, talkInfo.TalkID, 0, 0)
		talkIDs = append(talkIDs, talkInfo.TalkID)

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

func (impl *servicerMDImpl) sendPendingTalks(ctx context.Context, servicer defs.Servicer) (err error) {
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

func (impl *servicerMDImpl) getTalkInfoWithMessages(ctx context.Context, talkID string) (*customertalkpb.ServiceTalkInfoAndMessages, error) {
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

func (impl *servicerMDImpl) sendResponseToServicersForTalk(excludedUniqueID uint64, talkID string, resp *customertalkpb.ServiceResponse) {
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

func (impl *servicerMDImpl) send4AllServicers(do func(defs.Servicer) error) {
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

func (impl *servicerMDImpl) send4AllOneServicer(servicerID uint64, do func(defs.Servicer) error) {
	servicers, ok := impl.servicers[servicerID]
	if !ok {
		return
	}

	for uniqueID, servicer := range servicers {
		if err := do(servicer); err != nil {
			servicer.Remove("SendFailed")

			delete(servicers, uniqueID)
		}
	}

	if len(servicers) == 0 {
		delete(impl.servicers, servicerID)
	}
}
