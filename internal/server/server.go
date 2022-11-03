package server

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func gRpcError(c codes.Code, err error) error {
	var errMsg string
	if err != nil {
		errMsg = err.Error()
	}

	return status.Error(c, errMsg)
}

func gRpcMessageError(c codes.Code, msg string) error {
	return status.Error(c, msg)
}
