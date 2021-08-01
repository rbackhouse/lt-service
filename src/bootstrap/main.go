package main

import (
	"potpie.org/locationtracker/src/server"
	ltservice "potpie.org/locationtracker/src/service"
	wsservice "potpie.org/locationtracker/src/ws"

	"potpie.org/locationtracker/src/settings"
)

func main() {
	handler := wsservice.StartService()
	srv := server.NewServer(settings.GrpcUnaryInterceptor(nil))
	ltservice.StartService(srv.GrpcServer())
	srv.Start(handler)
}
