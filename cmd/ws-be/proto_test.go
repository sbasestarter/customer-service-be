package main

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/stretchr/testify/assert"
)

func TestProto(t *testing.T) {
	r := &customertalkpb.TalkInfo{
		TalkId: "1",
		Title:  "title1",
	}

	d, err := proto.Marshal(r)
	assert.Nil(t, err)
	t.Log(d)
}
