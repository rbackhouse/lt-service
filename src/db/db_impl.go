package db

import (
	"fmt"
	"time"

	"potpie.org/locationtracker/src/settings"

	"github.com/gomodule/redigo/redis"
	logger "github.com/sirupsen/logrus"
)

type client struct {
	pool *redis.Pool
}

func NewClient() Client {
	s := settings.NewSettings()

	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", s.RedisUrl) },
	}
	return &client{
		pool: pool,
	}
}

func (c *client) Register(username string, trackable bool) (int, error) {
	conn := c.pool.Get()
	defer conn.Close()

	existing, err := redis.Bool(conn.Do("HEXISTS", "users", username))
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}
	if existing == true {
		logger.Warnf("User %s is already registered", username)
		return -1, fmt.Errorf("User %s is already registered", username)
	}
	userid, err := redis.Int(conn.Do("INCR", "next_user_id"))
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}
	userkey := fmt.Sprintf("user:%d", userid)
	_, err = conn.Do("HSET", "users", username, userid)
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}

	_, err = conn.Do("HSET", userkey, "username", username, "trackable", trackable)
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}

	if trackable {
		_, err = conn.Do("RPUSH", "trackables", userid)
	}

	return userid, nil
}

func (c *client) StartSession(username string) (int, error) {
	conn := c.pool.Get()
	defer conn.Close()

	userid, err := getUserId(conn, username)
	if err != nil {
		return -1, err
	}
	userkey := fmt.Sprintf("user:%d", userid)

	sessionid, err := redis.Int(conn.Do("INCR", "next_session_id"))
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}
	_, err = redis.Int(conn.Do("HSET", userkey, "currentsession", sessionid))
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}
	sessionskey := fmt.Sprintf("sessions:%d", userid)

	_, err = conn.Do("ZADD", sessionskey, time.Now().Unix(), sessionid)
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}
	logger.Infof("Start Session %s %s %d", username, sessionskey, sessionid)

	return sessionid, nil
}

func (c *client) StopSession(username string) error {
	conn := c.pool.Get()
	defer conn.Close()

	userid, err := getUserId(conn, username)
	if err != nil {
		return err
	}
	userkey := fmt.Sprintf("user:%d", userid)

	_, err = conn.Do("HDEL", userkey, "currentsession")
	if err != nil {
		logger.Fatal(err)
		return err
	}
	logger.Infof("Stop Session %s", username)

	return nil
}

func (c *client) StartTracking(trackeename string, username string) error {
	conn := c.pool.Get()
	defer conn.Close()

	trackeeid, err := getUserId(conn, trackeename)
	if err != nil {
		return err
	}
	userkey := fmt.Sprintf("user:%d", trackeeid)
	trackable, err := redis.Bool(conn.Do("HGET", userkey, "trackable"))

	if trackable {
		trackedkey := fmt.Sprintf("tracked:%d", trackeeid)
		userid, err := conn.Do("HGET", "users", username)
		if err != nil {
			logger.Fatal(err)
			return err
		}

		_, err = conn.Do("ZADD", trackedkey, time.Now().Unix(), userid)
		if err != nil {
			logger.Fatal(err)
			return err
		}
	} else {
		return fmt.Errorf("User %s is not trackable", trackeename)
	}

	return nil
}

func (c *client) StopTracking(trackeename string, username string) error {
	conn := c.pool.Get()
	defer conn.Close()

	trackeeid, err := getUserId(conn, username)
	if err != nil {
		return err
	}
	userkey := fmt.Sprintf("user:%d", trackeeid)
	trackable, err := redis.Bool(conn.Do("HGET", userkey, "trackable"))

	if trackable {
		trackedkey := fmt.Sprintf("tracked:%d", trackeeid)
		userid, err := conn.Do("HGET", "users", username)
		if err != nil {
			logger.Fatal(err)
			return err
		}
		_, err = conn.Do("ZREM", trackedkey, userid)
		if err != nil {
			logger.Fatal(err)
			return err
		}
	} else {
		return fmt.Errorf("User %s is not trackable", trackeename)
	}

	return nil
}

func (c *client) ReportLocation(username string, longitude float64, latitude float64, timestamp int64) error {
	conn := c.pool.Get()
	defer conn.Close()

	userid, err := getUserId(conn, username)
	if err != nil {
		return err
	}
	userkey := fmt.Sprintf("user:%d", userid)

	locationid, err := redis.Int(conn.Do("INCR", "next_location_id"))
	if err != nil {
		logger.Fatal(err)
		return err
	}
	locationkey := fmt.Sprintf("location:%d", locationid)

	//logger.Infof("Location %s %d %f:%f %d", username, locationid, latitude, longitude, timestamp)

	_, err = conn.Do("HSET", locationkey, "latitude", latitude, "longitude", longitude, "timestamp", timestamp)
	if err != nil {
		logger.Fatal(err)
		return err
	}

	channel := fmt.Sprintf("channel:%s", username)

	_, err = conn.Do("PUBLISH", channel, locationkey)
	if err != nil {
		logger.Fatal(err)
		return err
	}

	currentsession, err := redis.Int(conn.Do("HGET", userkey, "currentsession"))
	if err != nil {
		logger.Fatal(err)
		return err
	}

	sessionkey := fmt.Sprintf("session:%d", currentsession)

	_, err = conn.Do("RPUSH", sessionkey, locationid)
	if err != nil {
		logger.Fatal(err)
		return err
	}

	return nil
}

func (c *client) GetTrackables() ([]string, error) {
	conn := c.pool.Get()
	defer conn.Close()

	trackables, err := redis.Ints(conn.Do("LRANGE", "trackables", 0, -1))
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}
	results := []string{}

	for _, userid := range trackables {
		userkey := fmt.Sprintf("user:%d", userid)
		username, err := redis.String(conn.Do("HGET", userkey, "username"))
		if err != nil {
			logger.Fatal(err)
			return nil, err
		}
		results = append(results, username)
	}
	return results, nil
}

func (c *client) GetSessionIds(username string) ([]SessionId, error) {
	conn := c.pool.Get()
	defer conn.Close()
	userid, err := getUserId(conn, username)
	if err != nil {
		return nil, err
	}

	sessionskey := fmt.Sprintf("sessions:%d", userid)

	sessions, err := redis.Int64s(conn.Do("ZRANGE", sessionskey, 0, -1))
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}

	results := []SessionId{}

	for _, id := range sessions {
		timestamp, err := redis.Int64(conn.Do("ZSCORE", sessionskey, id))
		if err != nil {
			logger.Fatal(err)
			return nil, err
		}
		logger.Infof("Location: %+v", SessionId{id, timestamp})

		results = append(results, SessionId{id, timestamp})
	}
	return results, nil
}

func (c *client) GetSessionData(sessionid int64) ([]TrackingData, error) {
	conn := c.pool.Get()
	defer conn.Close()

	sessionkey := fmt.Sprintf("session:%d", sessionid)
	locations, err := redis.Ints(conn.Do("LRANGE", sessionkey, 0, -1))
	if err != nil {
		logger.Fatal(err)
		return nil, err
	}
	results := []TrackingData{}
	for _, locationid := range locations {
		locationkey := fmt.Sprintf("location:%d", locationid)
		location, err := redis.Float64s(conn.Do("HMGET", locationkey, "latitude", "longitude", "timestamp"))
		if err != nil {
			logger.Fatal(err)
			return nil, err
		}
		results = append(results, TrackingData{int64(locationid), location[1], location[0], int64(location[2])})
	}
	return results, nil
}

func (c *client) MonitorLocation(trackeename string, username string, cb MonitorFunc) error {
	conn := c.pool.Get()
	defer conn.Close()

	channel := fmt.Sprintf("channel:%s", trackeename)

	psc := redis.PubSubConn{Conn: conn}
	psc.Subscribe(channel)
	for {
		switch v := psc.Receive().(type) {
		case redis.Message:
			locationkey := string(v.Data)
			logger.Infof("%s: message: %s", v.Channel, locationkey)
			cb(locationkey)
		case redis.Subscription:
			logger.Infof("%s: %s %d\n", v.Channel, v.Kind, v.Count)
		case error:
			return v
		}
	}
}

func (c *client) GetLocation(locationkey string) (TrackingData, error) {
	conn := c.pool.Get()
	defer conn.Close()

	location, err := redis.Float64s(conn.Do("HMGET", locationkey, "latitude", "longitude", "timestamp"))

	if err != nil {
		logger.Fatal(err)
		return TrackingData{}, err
	}

	return TrackingData{int64(0), location[1], location[0], int64(location[2])}, nil
}

func getUserId(conn redis.Conn, username string) (int, error) {
	id, err := conn.Do("HGET", "users", username)
	if err != nil {
		logger.Fatal(err)
		return -1, err
	}
	if id == nil {
		return -1, fmt.Errorf("Username %s does not exist", username)
	}
	userid, err := redis.Int(id, nil)
	return userid, nil
}
