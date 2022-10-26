package server

import (
	"testing"

	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"google.golang.org/protobuf/encoding/protojson"
)

type utPrint struct {
	t  *testing.T
	id string
}

// nolint
func (print *utPrint) Print(msg string) {
	print.t.Logf("%s: %s", print.id, msg)
}

// nolint
func TestDestruct(t *testing.T) {
	print := &utPrint{
		t:  t,
		id: "1",
	}

	print.Print("enter")
	defer print.Print("leave")
	defer func() {
		print.Print("leave2")
	}()

	print = &utPrint{
		t:  t,
		id: "2",
	}

	print.Print("hoho")
}

func TestRequest(t *testing.T) {
	r := &customertalkpb.TalkRequest{
		Talk: &customertalkpb.TalkRequest_Create{
			Create: &customertalkpb.TalkCreateRequest{
				Title: "abcd",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	r = &customertalkpb.TalkRequest{
		Talk: &customertalkpb.TalkRequest_Open{
			Open: &customertalkpb.TalkOpenRequest{
				TalkId: "635391d445a949658512eb4c",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	r = &customertalkpb.TalkRequest{
		Talk: &customertalkpb.TalkRequest_Message{
			Message: &customertalkpb.TalkMessageW{
				SeqId: 10,
				Message: &customertalkpb.TalkMessageW_Text{
					Text: "1234",
				},
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	r = &customertalkpb.TalkRequest{
		Talk: &customertalkpb.TalkRequest_Close{
			Close: &customertalkpb.TalkClose{},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	sr := &customertalkpb.ServiceRequest{
		Request: &customertalkpb.ServiceRequest_Attach{
			Attach: &customertalkpb.ServiceAttachRequest{
				TalkId: "6358df94b02c89cb932c884f",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(sr))

	sr = &customertalkpb.ServiceRequest{
		Request: &customertalkpb.ServiceRequest_Message{
			Message: &customertalkpb.ServicePostMessage{
				TalkId: "6358df94b02c89cb932c884f",
				Message: &customertalkpb.TalkMessageW{
					SeqId: 100,
					Message: &customertalkpb.TalkMessageW_Text{
						Text: "Who are you?",
					},
				},
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(sr))

	sr = &customertalkpb.ServiceRequest{
		Request: &customertalkpb.ServiceRequest_Detach{
			Detach: &customertalkpb.ServiceDetachRequest{
				TalkId: "6358df94b02c89cb932c884f",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(sr))
}
