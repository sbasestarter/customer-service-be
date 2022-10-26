package impls

import (
	"context"

	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sgostarter/libeasygo/commerr"
)

func NewModelEx(m defs.Model) defs.ModelEx {
	return &modelExImpl{
		m: m,
	}
}

type modelExImpl struct {
	m defs.Model
}

func (impl *modelExImpl) CreateTalk(ctx context.Context, talkInfo *defs.TalkInfoW) (talkID string, err error) {
	return impl.m.CreateTalk(ctx, talkInfo)
}

func (impl *modelExImpl) OpenTalk(ctx context.Context, talkID string) (err error) {
	return impl.m.OpenTalk(ctx, talkID)
}

func (impl *modelExImpl) CloseTalk(ctx context.Context, talkID string) error {
	return impl.m.CloseTalk(ctx, talkID)
}

func (impl *modelExImpl) AddTalkMessage(ctx context.Context, talkID string, message *defs.TalkMessageW) (err error) {
	return impl.m.AddTalkMessage(ctx, talkID, message)
}

func (impl *modelExImpl) GetTalkMessages(ctx context.Context, talkID string, offset, count int64) (messages []*defs.TalkMessageR, err error) {
	return impl.m.GetTalkMessages(ctx, talkID, offset, count)
}

func (impl *modelExImpl) TalkExists(ctx context.Context, talkID string) (exists bool, err error) {
	talkInfos, err := impl.m.QueryTalks(ctx, 0, 0, talkID, nil)
	if err != nil {
		return
	}

	if len(talkInfos) == 0 {
		return
	}

	exists = true

	return
}

func (impl *modelExImpl) QueryTalks(ctx context.Context, creatorID, serviceID uint64, talkID string,
	statuses []defs.TalkStatus) (talks []*defs.TalkInfoR, err error) {
	return impl.m.QueryTalks(ctx, creatorID, serviceID, talkID, statuses)
}

func (impl *modelExImpl) GetTalkInfo(ctx context.Context, talkID string) (talkInfo *defs.TalkInfoR, err error) {
	talkInfos, err := impl.m.QueryTalks(ctx, 0, 0, talkID, nil)
	if err != nil {
		return
	}

	if len(talkInfos) == 0 {
		err = commerr.ErrNotFound

		return
	}

	talkInfo = talkInfos[0]

	return
}

func (impl *modelExImpl) GetPendingTalkInfos(ctx context.Context) ([]*defs.TalkInfoR, error) {
	return impl.m.GetPendingTalkInfos(ctx)
}

func (impl *modelExImpl) UpdateTalkServiceID(ctx context.Context, talkID string, serviceID uint64) (err error) {
	return impl.m.UpdateTalkServiceID(ctx, talkID, serviceID)
}

func (impl *modelExImpl) GetServicerTalkInfos(ctx context.Context, servicerID uint64) ([]*defs.TalkInfoR, error) {
	talkInfos, err := impl.m.QueryTalks(ctx, 0, servicerID, "", []defs.TalkStatus{defs.TalkStatusOpened})
	if err != nil {
		return nil, err
	}

	return talkInfos, nil
}

func (impl *modelExImpl) GetTalkServicerID(ctx context.Context, talkID string) (servicerID uint64, err error) {
	talkInfo, err := impl.GetTalkInfo(ctx, talkID)
	if err != nil {
		return
	}

	servicerID = talkInfo.ServiceID

	return
}
