package impls

import (
	"context"
	"fmt"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-be/internal/vo"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libeasygo/commerr"
)

type mainData struct {
	m      defs.ModelEx
	logger l.Wrapper

	pendingTalkInfos    map[string]*defs.TalkInfoR // talkID - talkInfo
	processingTalkInfos map[string]*defs.TalkInfoR // talkID - talkInfo

	processingTalkRecord map[uint64][]string // servicerID - talkIDs

	customers map[string]map[uint64]defs.Customer // talkID - customerN - customer
	servicers map[uint64]map[uint64]defs.Servicer // servicerID - servicerN - servicer
}

func NewMainData(m defs.ModelEx, logger l.Wrapper) defs.MD {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	return &mainData{
		m:                    m,
		logger:               logger,
		pendingTalkInfos:     make(map[string]*defs.TalkInfoR),
		processingTalkInfos:  make(map[string]*defs.TalkInfoR),
		processingTalkRecord: make(map[uint64][]string),
		customers:            make(map[string]map[uint64]defs.Customer),
		servicers:            make(map[uint64]map[uint64]defs.Servicer),
	}
}

func (md *mainData) Load(ctx context.Context) (err error) {
	talkInfos, err := md.m.QueryTalks(ctx, 0, 0, "", []defs.TalkStatus{defs.TalkStatusOpened})
	if err != nil {
		md.logger.WithFields(l.ErrorField(err)).Fatal("QueryTalksFailed")

		return
	}

	for idx := 0; idx < len(talkInfos); idx++ {
		info := talkInfos[idx]

		if info.ServiceID == 0 {
			md.pendingTalkInfos[info.TalkID] = info

			continue
		}

		md.processingTalkInfos[info.TalkID] = info
		md.processingTalkRecord[info.ServiceID] = append(md.processingTalkRecord[info.ServiceID], info.TalkID)
	}

	return
}

func (md *mainData) InstallCustomer(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		md.logger.Error("noCustomer")

		return
	}

	if _, ok := md.customers[customer.GetTalkID()]; !ok {
		md.customers[customer.GetTalkID()] = make(map[uint64]defs.Customer)
	}

	md.customers[customer.GetTalkID()][customer.GetUniqueID()] = customer

	_, ok := md.processingTalkInfos[customer.GetTalkID()]
	if !ok {
		_, ok = md.pendingTalkInfos[customer.GetTalkID()]
		if !ok {
			talkInfo, _ := md.m.GetTalkInfo(ctx, customer.GetTalkID())
			if talkInfo != nil {
				md.pendingTalkInfos[customer.GetTalkID()] = talkInfo
			} else {
				md.logger.WithFields(l.StringField("talkID", customer.GetTalkID())).
					Error("GetTalkInfoFailed")
			}
		}
	}

	go func(logger l.Wrapper) {
		messages, err := md.m.GetTalkMessages(context.TODO(), customer.GetTalkID(), 0, 0)
		if err != nil {
			logger.Error("GetTalkMessageFailed")

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
	}(md.logger.WithFields(l.StringField("customer", fmt.Sprintf("%s-%d", customer.GetTalkID(),
		customer.GetUniqueID()))))
}

func (md *mainData) UninstallCustomer(_ context.Context, customer defs.Customer) {
	if customer == nil {
		md.logger.Error("noCustomer")

		return
	}

	if talkCustomers, ok := md.customers[customer.GetTalkID()]; ok {
		delete(talkCustomers, customer.GetUniqueID())

		if len(talkCustomers) == 0 {
			delete(md.customers, customer.GetTalkID())
		}
	}
}

func (md *mainData) CustomerMessageIncoming(ctx context.Context, customer defs.Customer,
	seqID uint64, message *defs.TalkMessageW) {
	if customer == nil {
		md.logger.Error("noCustomer")

		return
	}

	if message == nil {
		md.logger.Error("noMessage")

		return
	}

	var customerExists bool

	customersMap := md.customers[customer.GetTalkID()]

	pbMessageResponse := &customertalkpb.TalkResponse{
		Talk: &customertalkpb.TalkResponse_Message{
			Message: vo.TalkMessageDB2Pb(message),
		},
	}

	for _, talkCustomer := range customersMap {
		pbResponse := pbMessageResponse

		if talkCustomer == customer {
			customerExists = true

			pbResponse = &customertalkpb.TalkResponse{
				Talk: &customertalkpb.TalkResponse_MessageConfirmed{
					MessageConfirmed: &customertalkpb.TalkMessageConfirmed{
						SeqId: seqID,
						At:    uint64(message.At),
					},
				},
			}
		}

		if err := talkCustomer.SendMessage(pbResponse); err != nil {
			md.logger.WithFields(l.ErrorField(err), l.UInt64Field("id", talkCustomer.GetUniqueID())).
				Error("SendMessageFailed")

			talkCustomer.Remove("sendMessageFailed")

			delete(customersMap, talkCustomer.GetUniqueID())
		}
	}

	if talkInfo, exists := md.processingTalkInfos[customer.GetTalkID()]; exists && talkInfo.ServiceID > 0 {
		servicersMap := md.servicers[talkInfo.ServiceID]

		for _, servicer := range servicersMap {
			if err := servicer.SendMessage(&customertalkpb.ServiceResponse{
				Response: &customertalkpb.ServiceResponse_Message{
					Message: &customertalkpb.ServiceTalkMessageResponse{
						TalkId:  customer.GetTalkID(),
						Message: vo.TalkMessageDB2Pb(message),
					},
				},
			}); err != nil {
				md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

				servicer.Remove("SendMessageFailed")

				delete(servicersMap, servicer.GetUniqueID())
			}
		}

		if len(servicersMap) == 0 {
			delete(md.servicers, talkInfo.ServiceID)
		}
	}

	if !customerExists {
		md.logger.WithFields(l.UInt64Field("id", customer.GetUniqueID())).Warn("customerNotExists")

		customer.Remove("customerNotExists")

		delete(customersMap, customer.GetUniqueID())
	}

	if len(customersMap) == 0 {
		delete(md.customers, customer.GetTalkID())
	}
}

func (md *mainData) CustomerClose(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		md.logger.Error("noCustomer")

		return
	}

	if err := md.m.CloseTalk(ctx, customer.GetTalkID()); err != nil {
		md.logger.WithFields(l.ErrorField(err)).Error("CloseTalkFailed")

		return
	}

	if talkInfo, ok := md.processingTalkInfos[customer.GetTalkID()]; ok {
		if talkInfo.ServiceID > 0 {
			talkRecords := md.processingTalkRecord[talkInfo.ServiceID]

			for idx := 0; idx < len(talkRecords); idx++ {
				if talkRecords[idx] == customer.GetTalkID() {
					talkRecords = append(talkRecords[:idx], talkRecords[idx+1:]...)

					break
				}
			}

			md.processingTalkRecord[talkInfo.ServiceID] = talkRecords
		}

		delete(md.processingTalkInfos, customer.GetTalkID())
	}

	if _, ok := md.pendingTalkInfos[customer.GetTalkID()]; ok {
		delete(md.processingTalkInfos, customer.GetTalkID())
	}

	for _, c := range md.customers[customer.GetTalkID()] {
		if err := c.SendMessage(&customertalkpb.TalkResponse{
			Talk: &customertalkpb.TalkResponse_Close{},
		}); err != nil {
			md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
		}
	}

	if talkInfo, ok := md.processingTalkInfos[customer.GetTalkID()]; ok && talkInfo.ServiceID > 0 {
		for _, servicer := range md.servicers[talkInfo.ServiceID] {
			if err := servicer.SendMessage(&customertalkpb.ServiceResponse{
				Response: &customertalkpb.ServiceResponse_Close{
					Close: &customertalkpb.ServiceTalkClose{},
				},
			}); err != nil {
				md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
			}
		}
	}
}

func (md *mainData) InstallServicer(ctx context.Context, servicer defs.Servicer) {
	if servicer == nil {
		md.logger.Error("noServicer")

		return
	}

	if talkIDs, ok := md.processingTalkRecord[servicer.GetUserID()]; ok && len(talkIDs) > 0 {
		md.sendAttachedTalks(ctx, servicer, md.processingTalkInfos, talkIDs, md.logger)
	}

	md.sendPendingTalks(servicer, md.pendingTalkInfos, md.logger)

	if _, ok := md.servicers[servicer.GetUserID()]; !ok {
		md.servicers[servicer.GetUserID()] = make(map[uint64]defs.Servicer)
	}

	md.servicers[servicer.GetUserID()][servicer.GetUniqueID()] = servicer
}

func (md *mainData) UninstallServicer(ctx context.Context, servicer defs.Servicer) {
	talkServicers, ok := md.servicers[servicer.GetUserID()]
	if !ok {
		md.logger.Warn("noUserService")

		return
	}

	delete(talkServicers, servicer.GetUniqueID())

	if len(talkServicers) == 0 {
		delete(md.servicers, servicer.GetUserID())
	}
}

// ServicerAttachTalk .
// nolint: funlen
func (md *mainData) ServicerAttachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
	if talkID == "" || servicer == nil {
		md.logger.WithFields(l.StringField("talkID", talkID)).Error("noServicerOrTalkID")

		return
	}

	talkInfo, ok := md.processingTalkInfos[talkID]
	if ok {
		if talkInfo.ServiceID == servicer.GetUserID() {
			md.logger.WithFields(l.StringField("talkID", talkID),
				l.UInt64Field("userID", servicer.GetUserID())).Warn("talkAlreadyAttached")

			return
		}

		if _, ok = md.servicers[talkInfo.ServiceID]; ok {
			if err := servicer.SendMessage(&customertalkpb.ServiceResponse{
				Response: &customertalkpb.ServiceResponse_Notify{
					Notify: &customertalkpb.ServiceTalkNotifyResponse{
						Msg: "Reject",
					},
				},
			}); err != nil {
				md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

				servicer.Remove("SendMessageFailed")
			}

			return
		}
	}

	var talkPending2ProcessingFlag bool

	talkInfo, ok = md.pendingTalkInfos[talkID]
	if !ok {
		talkInfo, ok = md.processingTalkInfos[talkID]
		if !ok {
			if err := servicer.SendMessage(&customertalkpb.ServiceResponse{
				Response: &customertalkpb.ServiceResponse_Notify{
					Notify: &customertalkpb.ServiceTalkNotifyResponse{
						Msg: "NoTalkID",
					},
				},
			}); err != nil {
				md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

				servicer.Remove("SendMessageFailed")
			}

			return
		}

		talkRecords := md.processingTalkRecord[talkInfo.ServiceID]

		for idx := 0; idx < len(talkRecords); idx++ {
			if talkRecords[idx] == talkID {
				talkRecords = append(talkRecords[:idx], talkRecords[idx+1:]...)

				break
			}
		}

		md.processingTalkRecord[talkInfo.ServiceID] = talkRecords
	} else {
		delete(md.pendingTalkInfos, talkID)
		md.processingTalkInfos[talkID] = talkInfo

		talkPending2ProcessingFlag = true
	}

	md.processingTalkInfos[talkID].ServiceID = servicer.GetUserID()

	if err := md.m.UpdateTalkServiceID(ctx, talkID, servicer.GetUserID()); err != nil {
		md.logger.WithFields(l.ErrorField(err)).Error("UpdateTalkServiceID")
	}

	md.processingTalkRecord[servicer.GetUserID()] = append(md.processingTalkRecord[servicer.GetUserID()], talkID)

	if talkPending2ProcessingFlag {
		for _, ss := range md.servicers {
			for _, s := range ss {
				if err := s.SendMessage(&customertalkpb.ServiceResponse{
					Response: &customertalkpb.ServiceResponse_Attach{
						Attach: &customertalkpb.ServiceAttachTalkResponse{
							Talk:              vo.TalkInfoRDb2Pb(talkInfo),
							AttachedServiceId: servicer.GetUserID(),
						},
					},
				}); err != nil {
					md.logger.Error("SendMessageFailed")
				}
			}
		}
	}

	talk, err := md.getTalkInfoWithMessages(ctx, talkID)
	if err != nil {
		md.logger.WithFields(l.ErrorField(err)).Error("getTalkInfoWithMessages")

		return
	}

	if err = servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_Reload{
			Reload: &customertalkpb.ServiceTalkReloadResponse{
				Talk: talk,
			},
		},
	}); err != nil {
		md.logger.Error("SendMessageFailed")
	}
}

func (md *mainData) ServicerDetachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
	if talkID == "" || servicer == nil {
		md.logger.WithFields(l.StringField("talkID", talkID)).Error("noServicerOrTalkID")

		return
	}

	var talkAttached bool

	for _, processingTalkID := range md.processingTalkRecord[servicer.GetUserID()] {
		if processingTalkID == talkID {
			talkAttached = true

			break
		}
	}

	if !talkAttached {
		if err := servicer.SendMessage(&customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Notify{
				Notify: &customertalkpb.ServiceTalkNotifyResponse{
					Msg: "talkNotAttached",
				},
			},
		}); err != nil {
			md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

			return
		}

		return
	}

	talkRecords := md.processingTalkRecord[servicer.GetUserID()]

	for idx := 0; idx < len(talkRecords); idx++ {
		if talkRecords[idx] == talkID {
			talkRecords = append(talkRecords[:idx], talkRecords[idx+1:]...)

			break
		}
	}

	md.processingTalkRecord[servicer.GetUserID()] = talkRecords

	md.pendingTalkInfos[talkID] = md.processingTalkInfos[talkID]
	delete(md.processingTalkInfos, talkID)
	md.pendingTalkInfos[talkID].ServiceID = 0

	if err := md.m.UpdateTalkServiceID(ctx, talkID, 0); err != nil {
		md.logger.WithFields(l.ErrorField(err)).Error("UpdateTalkServiceID")
	}

	for _, s := range md.servicers[servicer.GetUserID()] {
		if err := s.SendMessage(&customertalkpb.ServiceResponse{
			Response: &customertalkpb.ServiceResponse_Detach{
				Detach: &customertalkpb.ServiceDetachTalkResponse{
					TalkId:            talkID,
					DetachedServiceId: servicer.GetUserID(),
				},
			},
		}); err != nil {
			md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

			return
		}
	}
}

func (md *mainData) ServicerQueryAttachedTalks(ctx context.Context, servicer defs.Servicer) {
	md.sendAttachedTalks(ctx, servicer, md.processingTalkInfos, md.processingTalkRecord[servicer.GetUserID()], md.logger)
}

func (md *mainData) ServicerQueryPendingTalks(_ context.Context, servicer defs.Servicer) {
	md.sendPendingTalks(servicer, md.pendingTalkInfos, md.logger)
}

func (md *mainData) ServicerReloadTalk(ctx context.Context, servicer defs.Servicer, talkID string) {
	talk, err := md.getTalkInfoWithMessages(ctx, talkID)
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
		md.logger.Error("SendMessageFailed")
	}
}

func (md *mainData) ServiceMessage(_ context.Context, servicer defs.Servicer, talkID string, seqID uint64, message *defs.TalkMessageW) {
	if servicer == nil || talkID == "" || message == nil {
		md.logger.Error("nilParameters")

		return
	}

	talkInfo, exists := md.processingTalkInfos[talkID]
	if !exists {
		md.logger.WithFields(l.StringField("talkID", talkID)).Error("talkIDNotExists")

		return
	}

	if talkInfo.ServiceID != servicer.GetUserID() {
		md.logger.WithFields(l.StringField("talkID", talkID), l.UInt64Field("serverID", talkInfo.ServiceID),
			l.UInt64Field("curServerID", servicer.GetUserID())).Error("otherServicerServing")

		return
	}

	var servicerExists bool

	servicersMap := md.servicers[servicer.GetUserID()]

	pbResp := &customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_Message{
			Message: &customertalkpb.ServiceTalkMessageResponse{
				TalkId:  talkID,
				Message: vo.TalkMessageDB2Pb(message),
			},
		},
	}

	for _, s := range servicersMap {
		resp := pbResp

		if s == servicer {
			servicerExists = true

			resp = &customertalkpb.ServiceResponse{
				Response: &customertalkpb.ServiceResponse_MessageConfirmed{
					MessageConfirmed: &customertalkpb.ServiceMessageConfirmed{
						SeqId: seqID,
						At:    uint64(message.At),
					},
				},
			}
		}

		if err := s.SendMessage(resp); err != nil {
			md.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

			s.Remove("SendMessageFailed")

			delete(servicersMap, s.GetUniqueID())
		}
	}

	if customersMap, ok := md.customers[talkID]; ok {
		pbMessageResponse := &customertalkpb.TalkResponse{
			Talk: &customertalkpb.TalkResponse_Message{
				Message: vo.TalkMessageDB2Pb(message),
			},
		}

		for _, customer := range customersMap {
			if err := customer.SendMessage(pbMessageResponse); err != nil {
				md.logger.WithFields(l.ErrorField(err), l.UInt64Field("id", customer.GetUniqueID())).
					Error("SendMessageFailed")

				customer.Remove("sendMessageFailed")

				delete(customersMap, customer.GetUniqueID())
			}
		}

		if len(customersMap) == 0 {
			delete(md.customers, talkID)
		}
	}

	if !servicerExists {
		servicer.Remove("noServicer")

		delete(servicersMap, servicer.GetUniqueID())
	}

	if len(servicersMap) == 0 {
		delete(md.servicers, servicer.GetUserID())
	}
}

func (md *mainData) getTalkInfoWithMessages(ctx context.Context, talkID string) (*customertalkpb.ServiceTalkInfoAndMessages, error) {
	talkInfo, ok := md.processingTalkInfos[talkID]
	if !ok {
		return nil, commerr.ErrNotFound
	}

	talkMessages, err := md.m.GetTalkMessages(ctx, talkID, 0, 0)
	if err != nil {
		md.logger.Error("NoTalkIDMessage")

		return nil, err
	}

	return &customertalkpb.ServiceTalkInfoAndMessages{
		TalkInfo: vo.TalkInfoRDb2Pb(talkInfo),
		Messages: vo.TalkMessagesRDb2Pb(talkMessages),
	}, nil
}

func (md *mainData) sendAttachedTalks(ctx context.Context, servicer defs.Servicer, processingTalkInfos map[string]*defs.TalkInfoR,
	talkIDs []string, logger l.Wrapper) {
	talks := make([]*customertalkpb.ServiceTalkInfoAndMessages, 0, len(talkIDs))

	for _, talkID := range talkIDs {
		talkInfo, ok := processingTalkInfos[talkID]
		if !ok {
			logger.Error("NoTalkIDOnPendingTalkInfos")

			continue
		}

		talkMessages, err := md.m.GetTalkMessages(ctx, talkID, 0, 0)
		if err != nil {
			logger.Error("NoTalkIDMessage")

			continue
		}

		talks = append(talks, &customertalkpb.ServiceTalkInfoAndMessages{
			TalkInfo: vo.TalkInfoRDb2Pb(talkInfo),
			Messages: vo.TalkMessagesRDb2Pb(talkMessages),
		})
	}

	err := servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_Talks{
			Talks: &customertalkpb.ServiceAttachedTalksResponse{
				Talks: talks,
			},
		},
	})
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}
}

func (md *mainData) sendPendingTalks(servicer defs.Servicer, pendingTalkInfos map[string]*defs.TalkInfoR, logger l.Wrapper) {
	talkInfos := make([]*defs.TalkInfoR, 0, len(pendingTalkInfos))

	for _, talkInfo := range pendingTalkInfos {
		talkInfos = append(talkInfos, talkInfo)
	}

	err := servicer.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_PendingTalks{
			PendingTalks: &customertalkpb.ServicePendingTalksResponse{
				Talks: vo.TalkInfoRsDB2Pb(talkInfos),
			},
		},
	})
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}
}
