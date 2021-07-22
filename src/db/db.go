package db

type TrackingData struct {
	Locationid int64
	Longitude  float64
	Latitude   float64
	Timestamp  int64
}

type SessionId struct {
	Id        int64
	Timestamp int64
}

type MonitorFunc func(locationkey string) error

type Client interface {
	Register(username string, trackable bool) (int, error)
	GetTrackables() ([]string, error)
	StartSession(username string) (int, error)
	StopSession(username string) error
	StartTracking(trackeename string, username string) error
	StopTracking(trackeename string, username string) error
	ReportLocation(username string, longitude float64, latitude float64, timestamp int64) error
	GetSessionIds(username string) ([]SessionId, error)
	GetSessionData(sessionid int64) ([]TrackingData, error)
	MonitorLocation(trackeename string, username string, cb MonitorFunc) error
	GetLocation(locationkey string) (TrackingData, error)
}
