package main

import (
	"potpie.org/locationtracker/src/server"
	ltservice "potpie.org/locationtracker/src/service"
	"potpie.org/locationtracker/src/settings"
)

func main() {
	srv := server.NewServer(settings.GrpcUnaryInterceptor(nil))
	ltservice.StartService(srv.GrpcServer())
	srv.Start()
}
