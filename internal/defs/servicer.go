package defs

import "github.com/sbasestarter/customer-service-proto/gens/customertalkpb"

type Servicer interface {
	GetUserID() uint64
	GetUniqueID() uint64
	SendMessage(msg *customertalkpb.ServiceResponse) error
	Remove(msg string)
}
