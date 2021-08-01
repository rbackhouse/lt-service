package wsservice

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"potpie.org/locationtracker/src/db"

	logger "github.com/sirupsen/logrus"
)

type RequestType int

const (
	START_TRACKING RequestType = iota
	STOP_TRACKING
	GET_TRACKABLES
	REGISTER
	GET_SESSION_IDS
	GET_SESSION_DATA
)

type ResponseType int

const (
	TRACKABLES ResponseType = iota
	REGISTER_ID
	SESSION_IDS
	SESSION_DATA
	TRACKING_DATA
)

type TrackingRequest struct {
	TrackeeName string
	UserName    string
}

type TrackingResponse struct {
	Type         ResponseType
	TrackingData db.TrackingData
}

type TrackablesResponse struct {
	Type       ResponseType
	Trackables []string
}

type RegisterRequest struct {
	UserName    string
	IsTrackable bool
}

type RegisterResponse struct {
	Type ResponseType
	Id   int
}

type SessionIdsRequest struct {
	UserName string
}

type SessionIdsResponse struct {
	Type ResponseType
	Ids  []db.SessionId
}

type SessionDataRequest struct {
	Id int64
}

type SessionDataResponse struct {
	Type ResponseType
	Data []db.TrackingData
}

type service struct {
	dbclient    db.Client
	connections map[string]net.Conn
}

func (this *service) StartTracking(trackeeName string, userName string, conn net.Conn) error {
	_, ok := this.connections[trackeeName+":"+userName]
	if ok {
		delete(this.connections, trackeeName+":"+userName)
	}
	this.connections[trackeeName+":"+userName] = conn
	err := this.dbclient.StartTracking(trackeeName, userName)
	if err != nil {
		return err
	}

	cb := func(locationkey string) error {
		td, err := this.dbclient.GetLocation(locationkey)
		if err != nil {
			return err
		}
		logger.Infof("Location: %+v", td)
		response := TrackingResponse{Type: TRACKING_DATA, TrackingData: td}
		msg, err := json.Marshal(response)
		if err != nil {
			return err
		}
		if err := wsutil.WriteServerMessage(conn, ws.OpText, msg); err != nil {
			return err
		}
		return nil
	}

	logger.Infof("StartTracking: %s %s", trackeeName, userName)
	err = this.dbclient.MonitorLocation(trackeeName, userName, cb)
	if err != nil {
		return err
	}
	return nil
}

func (this *service) StopTracking(trackeeName string, userName string, conn net.Conn) error {
	logger.Infof("StopTracking: %s %s", trackeeName, userName)
	_, ok := this.connections[trackeeName+":"+userName]
	if !ok {
		return nil
	}
	delete(this.connections, trackeeName+":"+userName)
	err := this.dbclient.StopTracking(trackeeName, userName)
	if err != nil {
		return err
	}

	return nil
}

func (this *service) GetTrackables(conn net.Conn) error {
	logger.Infof("GetTrackables")

	this.dbclient.GetTrackables()
	trackables, err := this.dbclient.GetTrackables()
	if err != nil {
		return err
	}
	response := TrackablesResponse{Type: TRACKABLES, Trackables: trackables}
	json, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if err := wsutil.WriteServerMessage(conn, ws.OpText, json); err != nil {
		return err
	}
	return nil
}

func (this *service) Register(userName string, isTrackage bool, conn net.Conn) error {
	logger.Infof("Register: %s %s", userName, isTrackage)

	id, err := this.dbclient.Register(userName, isTrackage)
	if err != nil {
		return err
	}
	response := RegisterResponse{Type: REGISTER_ID, Id: id}
	json, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if err := wsutil.WriteServerMessage(conn, ws.OpText, json); err != nil {
		return err
	}
	return nil
}

func (this *service) GetSessionIds(userName string, conn net.Conn) error {
	logger.Infof("GetSessionIds: %s", userName)

	ids, err := this.dbclient.GetSessionIds(userName)
	if err != nil {
		return err
	}
	response := SessionIdsResponse{Type: SESSION_IDS, Ids: ids}

	json, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if err := wsutil.WriteServerMessage(conn, ws.OpText, json); err != nil {
		return err
	}

	return nil
}

func (this *service) GetSessionData(id int64, conn net.Conn) error {
	logger.Infof("GetSessionData: %d", id)

	data, err := this.dbclient.GetSessionData(id)
	if err != nil {
		return err
	}
	response := SessionDataResponse{Type: SESSION_DATA, Data: data}

	json, err := json.Marshal(response)
	if err != nil {
		return err
	}
	if err := wsutil.WriteServerMessage(conn, ws.OpText, json); err != nil {
		return err
	}

	return nil
}

func (this *service) HandleMsg(conn net.Conn, msg []byte) {
	var objmap map[string]json.RawMessage
	err := json.Unmarshal([]byte(msg), &objmap)
	if err != nil {
		logger.Warn(err)
		return
	}

	var reqType RequestType
	err = json.Unmarshal(objmap["RequestType"], &reqType)
	if err != nil {
		logger.Warn(err)
		return
	}
	switch reqType {
	case START_TRACKING:
		var tr TrackingRequest
		err = json.Unmarshal(objmap["TrackingRequest"], &tr)
		if err != nil {
			logger.Warn(err)
			return
		}
		err = this.StartTracking(tr.TrackeeName, tr.UserName, conn)
		if err != nil {
			logger.Warn(err)
		}
		break
	case STOP_TRACKING:
		var tr TrackingRequest
		err = json.Unmarshal(objmap["TrackingRequest"], &tr)
		if err != nil {
			logger.Warn(err)
			return
		}
		err = this.StopTracking(tr.TrackeeName, tr.UserName, conn)
		if err != nil {
			logger.Warn(err)
		}
		break
	case GET_TRACKABLES:
		err = this.GetTrackables(conn)
		if err != nil {
			logger.Warn(err)
		}
		break
	case REGISTER:
		var rr RegisterRequest
		err = json.Unmarshal(objmap["RegisterRequest"], &rr)
		if err != nil {
			logger.Warn(err)
			return
		}
		err = this.Register(rr.UserName, rr.IsTrackable, conn)
		if err != nil {
			logger.Warn(err)
		}
		break
	case GET_SESSION_IDS:
		var sir SessionIdsRequest
		err = json.Unmarshal(objmap["SessionIdsRequest"], &sir)
		if err != nil {
			logger.Warn(err)
			return
		}
		err = this.GetSessionIds(sir.UserName, conn)
		if err != nil {
			logger.Warn(err)
		}
		break
	case GET_SESSION_DATA:
		var sdr SessionDataRequest
		err = json.Unmarshal(objmap["SessionDataRequest"], &sdr)
		if err != nil {
			logger.Warn(err)
			return
		}
		err = this.GetSessionData(sdr.Id, conn)
		if err != nil {
			logger.Warn(err)
		}
		break
	}
}

func StartService() http.HandlerFunc {
	newService := &service{db.NewClient(), make(map[string]net.Conn)}
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(request, writer)
		if err != nil {
			logger.Warn(err)
			return
		}
		go func() {
			defer conn.Close()

			for {
				msg, _, err := wsutil.ReadClientData(conn)
				if err != nil {
					logger.Warn(err)
					return
				} else {
					logger.Infof("Msg read : %s", string(msg))
					newService.HandleMsg(conn, msg)
				}
			}
		}()
	})

	return handler
}
