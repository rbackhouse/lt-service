package ltservice

import (
	"context"
	"io"

	"google.golang.org/grpc"

	pb "potpie.org/locationtracker/proto"

	"potpie.org/locationtracker/src/db"

	logger "github.com/sirupsen/logrus"
)

type service struct {
	dbclient db.Client
	sessions map[string]pb.LocationTracker_StartTrackingServer
}

func (this *service) Register(ctx context.Context, in *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	userid, err := this.dbclient.Register(in.GetUserName(), in.Trackable)
	if err != nil {
		return nil, err
	}
	return &pb.RegisterResponse{UserId: int64(userid)}, nil
}

func (this *service) GetTrackables(ctx context.Context, in *pb.GetTrackablesRequest) (*pb.GetTrackablesResponse, error) {
	trackables, err := this.dbclient.GetTrackables()
	if err != nil {
		return nil, err
	}
	return &pb.GetTrackablesResponse{UserName: trackables}, nil
}

func (this *service) StartSession(ctx context.Context, in *pb.StartSessionRequest) (*pb.StartSessionResponse, error) {
	_, err := this.dbclient.StartSession(in.GetUserName())
	if err != nil {
		return nil, err
	}
	return &pb.StartSessionResponse{}, nil
}

func (this *service) StopSession(ctx context.Context, in *pb.StopSessionRequest) (*pb.StopSessionResponse, error) {
	err := this.dbclient.StopSession(in.GetUserName())
	if err != nil {
		return nil, err
	}
	return &pb.StopSessionResponse{}, nil
}

func (this *service) StartTracking(in *pb.StartTrackingRequest, stream pb.LocationTracker_StartTrackingServer) error {
	_, ok := this.sessions[in.GetTrackeeName()+":"+in.GetUserName()]
	if ok {
		delete(this.sessions, in.GetTrackeeName()+":"+in.GetUserName())
	}
	this.sessions[in.GetTrackeeName()+":"+in.GetUserName()] = stream
	err := this.dbclient.StartTracking(in.GetTrackeeName(), in.GetUserName())
	if err != nil {
		return err
	}

	cb := func(locationkey string) error {
		td, err := this.dbclient.GetLocation(locationkey)
		if err != nil {
			return err
		}
		logger.Infof("Location: %+v", td)

		if err := stream.Send(&pb.TrackingData{TrackeeName: in.GetTrackeeName(), Longitude: td.Longitude, Latitude: td.Latitude, Timestamp: td.Timestamp}); err != nil {
			return err
		}
		return nil
	}

	logger.Infof("StartTracking 1: %s %s", in.GetTrackeeName(), in.GetUserName())
	err = this.dbclient.MonitorLocation(in.GetTrackeeName(), in.GetUserName(), cb)
	logger.Infof("StartTracking 2: %s %s", in.GetTrackeeName(), in.GetUserName())
	if err != nil {
		return err
	}
	return nil
}

func (this *service) StopTracking(ctx context.Context, in *pb.StopTrackingRequest) (*pb.StopTrackingResponse, error) {
	logger.Infof("StopTracking: %s %s", in.GetTrackeeName(), in.GetUserName())
	_, ok := this.sessions[in.GetTrackeeName()+":"+in.GetUserName()]
	if !ok {
		return nil, nil
	}
	delete(this.sessions, in.GetTrackeeName()+":"+in.GetUserName())
	err := this.dbclient.StopTracking(in.GetTrackeeName(), in.GetUserName())
	if err != nil {
		return nil, err
	}

	return &pb.StopTrackingResponse{}, nil
}

func (this *service) ReportLocation(stream pb.LocationTracker_ReportLocationServer) error {
	for {
		in, err := stream.Recv()

		if err == io.EOF {
			return stream.SendAndClose(&pb.ReportLocationResponse{})
		}
		err = this.dbclient.ReportLocation(in.GetTrackeeName(), in.GetLongitude(), in.GetLatitude(), in.GetTimestamp())
		if err != nil {
			return err
		}
	}
}

func (this *service) GetSessionIds(ctx context.Context, in *pb.SessionIdsRequest) (*pb.SessionIdsResponse, error) {
	ids, err := this.dbclient.GetSessionIds(in.GetUserName())
	if err != nil {
		return nil, err
	}
	results := []*pb.SessionId{}

	for _, id := range ids {
		results = append(results, &pb.SessionId{SessionId: id.Id, Timestamp: id.Timestamp})
	}
	return &pb.SessionIdsResponse{SessionId: results}, nil
}

func (this *service) GetSessionData(ctx context.Context, in *pb.SessionDataRequest) (*pb.SessionDataResponse, error) {
	data, err := this.dbclient.GetSessionData(in.SessionId)
	if err != nil {
		return nil, err
	}
	results := []*pb.TrackingData{}

	for _, d := range data {
		results = append(results, &pb.TrackingData{Longitude: d.Longitude, Latitude: d.Latitude, Timestamp: d.Timestamp})
	}
	return &pb.SessionDataResponse{TrackingData: results}, nil
}

func StartService(grpcServer *grpc.Server) pb.LocationTrackerServer {
	newService := &service{db.NewClient(), make(map[string]pb.LocationTracker_StartTrackingServer)}
	pb.RegisterLocationTrackerServer(grpcServer, newService)

	return newService
}
