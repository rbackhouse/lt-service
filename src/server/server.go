package server

import (
	"net/http"

	"google.golang.org/grpc"
)

type Server interface {
	Start(handler http.HandlerFunc)
	GrpcServer() *grpc.Server
}
