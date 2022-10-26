package controller

import (
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/libeasygo/commerr"
)

func NewCustomer(uniqueID uint64, talkID string, userID uint64, chSendMessage chan *customertalkpb.TalkResponse) defs.Customer {
	return &customerImpl{
		uniqueID:      uniqueID,
		talkID:        talkID,
		userID:        userID,
		chSendMessage: chSendMessage,
	}
}

type customerImpl struct {
	uniqueID      uint64
	talkID        string
	userID        uint64
	chSendMessage chan *customertalkpb.TalkResponse
}

func (impl *customerImpl) GetUniqueID() uint64 {
	return impl.uniqueID
}

func (impl *customerImpl) GetTalkID() string {
	return impl.talkID
}

func (impl *customerImpl) GetUserID() uint64 {
	return impl.userID
}

func (impl *customerImpl) SendMessage(msg *customertalkpb.TalkResponse) error {
	select {
	case impl.chSendMessage <- msg:
	default:
		return commerr.ErrAborted
	}

	return nil
}

func (impl *customerImpl) Remove(msg string) {
	_ = impl.SendMessage(&customertalkpb.TalkResponse{
		Talk: &customertalkpb.TalkResponse_KickOut{
			KickOut: &customertalkpb.TalkKickOutMessage{
				Code:    -1,
				Message: msg,
			},
		},
	})
}
