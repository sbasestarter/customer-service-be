package main

import (
	"context"
	"encoding/json"
	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libservicetoolset/clienttoolset"
	"google.golang.org/grpc/metadata"
	"io"
	"log"
	"net/http"
)

const (
	httpTokenHeaderKey = "token"
)

func wsReceive(conn *websocket.Conn, stream customertalkpb.CustomerTalkService_TalkClient, logger l.Wrapper) {
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

func gRPCReceiveRoutine(stream customertalkpb.CustomerTalkService_TalkClient, conn *websocket.Conn, logger l.Wrapper) {
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

func wS(gRpcClient customertalkpb.CustomerTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
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

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UpgradeFailed")

			return
		}

		defer c.Close()

		_, msg, err := c.ReadMessage()
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReadTokenMessageFailed")

			return
		}

		kv := make(map[string]string)
		err = json.Unmarshal(msg, &kv)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalTokenFailed")

			return
		}

		md := metadata.New(map[string]string{
			"token": kv["token"],
		})

		stream, err := gRpcClient.Talk(metadata.NewOutgoingContext(context.TODO(), md))
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("GRPCServiceFailed")

			return
		}

		go gRPCReceiveRoutine(stream, c, logger)

		wsReceive(c, stream, logger)
	}
}

func checkHandler(gRpcClient customertalkpb.CustomerTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		md := metadata.New(map[string]string{
			"token": r.Header.Get(httpTokenHeaderKey),
		})

		resp, err := gRpcClient.CheckToken(metadata.NewOutgoingContext(context.TODO(), md), &customertalkpb.CheckTokenRequest{})
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("CheckTokenFailed")

			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		d, _ := proto.Marshal(resp)
		_, _ = w.Write(d)
	}
}

func createHandler(gRpcClient customertalkpb.CustomerTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		d, err := io.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReadAllFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		var request customertalkpb.CreateTokenRequest
		err = proto.Unmarshal(d, &request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		resp, err := gRpcClient.CreateToken(context.TODO(), &request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("CheckTokenFailed")

			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		d, _ = proto.Marshal(resp)
		_, _ = w.Write(d)
	}
}

func listTalkHandler(gRpcClient customertalkpb.CustomerTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		md := metadata.New(map[string]string{
			"token": r.Header.Get(httpTokenHeaderKey),
		})

		resp, err := gRpcClient.QueryTalks(metadata.NewOutgoingContext(context.TODO(), md), &customertalkpb.QueryTalksRequest{
			Statuses: []customertalkpb.TalkStatus{
				customertalkpb.TalkStatus_TALK_STATUS_OPENED,
			},
		})
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("CheckTokenFailed")

			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		d, _ := proto.Marshal(resp)
		_, _ = w.Write(d)
	}
}

func main() {
	cfg := config.GetWSConfig()

	conn, err := clienttoolset.DialGRPC(cfg.CustomerGRPCClientConfig, nil)
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	defer conn.Close()

	gRpcClient := customertalkpb.NewCustomerTalkServiceClient(conn)

	http.HandleFunc("/checkToken", checkHandler(gRpcClient, cfg.Logger))
	http.HandleFunc("/createToken", createHandler(gRpcClient, cfg.Logger))
	http.HandleFunc("/listTalk", listTalkHandler(gRpcClient, cfg.Logger))
	http.HandleFunc("/ws", wS(gRpcClient, cfg.Logger))

	log.Fatal(http.ListenAndServe(cfg.CustomerListen, nil))
}
