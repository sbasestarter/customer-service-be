package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/sbasestarter/customer-service-be/config"
	"github.com/sbasestarter/customer-service-proto/gens/customertalkpb"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libservicetoolset/clienttoolset"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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
			if s, ok := status.FromError(err); ok {
				if s.Code() == codes.Unauthenticated {
					_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(s.Code()), s.Message()))
				}
			}

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

		stream, err := gRpcClient.Service(metadata.NewOutgoingContext(context.TODO(), md))
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("GRPCServiceFailed")

			return
		}

		go gRPCReceiveRoutine(stream, c, logger)

		wsReceive(c, stream, logger)
	}
}

type loginData struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type loginDataResponse struct {
	Token    string `json:"token"`
	UserName string `json:"user_name"`
}

func loginHandler(gRpcClient customertalkpb.ServicerUserServicerClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		d, err := io.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReadAllFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		var loginD loginData
		err = json.Unmarshal(d, &loginD)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalFailed")
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		resp, err := gRpcClient.Login(r.Context(), &customertalkpb.LoginRequest{
			UserName: loginD.UserName,
			Password: loginD.Password,
		})
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("LoginFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		ldResp := &loginDataResponse{
			Token:    resp.Token,
			UserName: resp.UserName,
		}

		d, _ = json.Marshal(ldResp)

		_, _ = w.Write(d)
	}
}

func main() {
	cfg := config.GetWSConfig()

	//
	//
	//

	talkConn, err := clienttoolset.DialGRPC(cfg.ServicerGRPCClientConfig, []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024 * 1024 * 1024)),
	})
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	defer talkConn.Close()

	gRpcTalkClient := customertalkpb.NewServiceTalkServiceClient(talkConn)

	//
	//
	//
	userConn, err := clienttoolset.DialGRPC(cfg.ServicerUserGRPCClientConfig, nil)
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	defer userConn.Close()

	gRpcUserClient := customertalkpb.NewServicerUserServicerClient(userConn)

	//
	//
	//

	http.HandleFunc("/login", loginHandler(gRpcUserClient, cfg.Logger))
	http.HandleFunc("/ws", wS(gRpcTalkClient, cfg.Logger))

	log.Fatal(http.ListenAndServe(cfg.ServicerListen, nil))
}
