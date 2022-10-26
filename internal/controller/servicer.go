package controller

import (
	"github.com/sbasestarter/customer-service-be/internal/defs"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/libeasygo/commerr"
)

func NewServicer(userID, uniqueID uint64, chSendMessage chan *customertalkpb.ServiceResponse) defs.Servicer {
	return &servicerImpl{
		userID:        userID,
		uniqueID:      uniqueID,
		chSendMessage: chSendMessage,
	}
}

type servicerImpl struct {
	userID        uint64
	uniqueID      uint64
	chSendMessage chan *customertalkpb.ServiceResponse
}

func (impl *servicerImpl) GetUserID() uint64 {
	return impl.userID
}

func (impl *servicerImpl) GetUniqueID() uint64 {
	return impl.uniqueID
}

func (impl *servicerImpl) SendMessage(msg *customertalkpb.ServiceResponse) error {
	select {
	case impl.chSendMessage <- msg:
	default:
		return commerr.ErrAborted
	}

	return nil
}

func (impl *servicerImpl) Remove(msg string) {
	_ = impl.SendMessage(&customertalkpb.ServiceResponse{
		Response: &customertalkpb.ServiceResponse_KickOut{
			KickOut: &customertalkpb.TalkKickOutMessage{
				Code:    -1,
				Message: msg,
			},
		},
	})
}
