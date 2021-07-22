package server

import (
	"google.golang.org/grpc"
)

type Server interface {
	Start()
	GrpcServer() *grpc.Server
}
