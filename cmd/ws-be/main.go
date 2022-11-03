package main

import (
	"context"
	"log"
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libservicetoolset/clienttoolset"
	"google.golang.org/grpc/metadata"
)

func wsReceive(conn *websocket.Conn, stream customertalkpb.ServiceTalkService_ServiceClient, logger l.Wrapper) {
	logger = logger.WithFields(l.StringField("func", "wsReceiveRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	for {
		messageType, message, err := conn.ReadMessage()
		if messageType == websocket.CloseMessage || err != nil {
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("ReadMessageFailed")
			}

			break
		}

		var request customertalkpb.ServiceRequest
		err = proto.Unmarshal(message, &request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalFailed")

			break
		}

		err = stream.SendMsg(&request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("SendGrpcMessageFailed")

			break
		}
	}
}

func gRPCReceiveRoutine(stream customertalkpb.ServiceTalkService_ServiceClient, conn *websocket.Conn, logger l.Wrapper) {
	logger = logger.WithFields(l.StringField("func", "gRPCReceiveRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")
	defer conn.Close()

	for {
		resp, err := stream.Recv()
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReceiveFailed")

			break
		}

		d, err := proto.Marshal(resp)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("MarshalFailed")

			break
		}

		err = conn.WriteMessage(websocket.BinaryMessage, d)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("WriteMessageFailed")

			break
		}
	}
}

func wS(gRpcClient customertalkpb.ServiceTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		Subprotocols: []string{"hey"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		logger = logger.WithFields(l.StringField(l.RoutineKey, "wS"))

		logger.Debug("enter")
		defer logger.Debug("leave")

		h := http.Header{}
		for _, sub := range websocket.Subprotocols(r) {
			if sub == "hey" {
				h.Set("Sec-Websocket-Protocol", "hey")
				break
			}
		}

		md := metadata.New(map[string]string{
			"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJJRCI6MTg0NTc0MDMwODQzMjY4MzAwOCwiVXNlck5hbWUiOiJ6anoiLCJleHAiOjE2NjkwOTUxODF9.3qqeSKcQxrr3CagAVQ79_sCSnBMmTM8u_k5jFHIjJUc",
		})

		stream, err := gRpcClient.Service(metadata.NewOutgoingContext(context.TODO(), md))
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("GRPCServiceFailed")

			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UpgradeFailed")

			return
		}

		defer c.Close()

		go gRPCReceiveRoutine(stream, c, logger)

		wsReceive(c, stream, logger)
	}
}

func main() {
	cfg := config.GetWSConfig()

	conn, err := clienttoolset.DialGRPC(cfg.GRPCClientConfig, nil)
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	defer conn.Close()

	gRpcClient := customertalkpb.NewServiceTalkServiceClient(conn)

	http.HandleFunc("/ws", wS(gRpcClient, cfg.Logger))

	log.Fatal(http.ListenAndServe(cfg.Listen, nil))
}
