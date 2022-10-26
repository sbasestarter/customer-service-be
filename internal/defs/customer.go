package defs

import "github.com/sbasestarter/customer-service-proto/gens/customertalkpb"

type Customer interface {
	GetUniqueID() uint64
	GetTalkID() string
	GetUserID() uint64
	SendMessage(msg *customertalkpb.TalkResponse) error
	Remove(msg string)
}
