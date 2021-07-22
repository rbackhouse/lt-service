package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	reuseport "github.com/kavu/go_reuseport"

	"potpie.org/locationtracker/src/settings"

	logger "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type server struct {
	grpcPort   int
	grpcServer *grpc.Server
}

func (server *server) Start() {
	server.handleGracefulShutdown()

	addr := fmt.Sprintf(":%d", server.grpcPort)
	logger.Infof("Listening for gRPC on '%s'", addr)
	lis, err := reuseport.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("Failed to listen for gRPC: %v", err)
	}
	server.grpcServer.Serve(lis)
}

func NewServer(opts ...settings.Option) Server {
	s := settings.NewSettings()
	for _, opt := range opts {
		opt(&s)
	}

	ret := new(server)
	ret.grpcServer = grpc.NewServer(s.GrpcUnaryInterceptor)

	ret.grpcPort = s.GrpcPort

	return ret
}

func (server *server) handleGracefulShutdown() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigs

		logger.Infof("HTTP server received %v, shutting down gracefully", sig)
		os.Exit(0)
	}()
}

func (server *server) GrpcServer() *grpc.Server {
	return server.grpcServer
}
