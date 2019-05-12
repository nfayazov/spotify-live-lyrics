package main

import (
	"encoding/json"
	"github.com/gomodule/redigo/redis"
)

const sessionPrefix string = "session"
const statePrefix 	string = "state"
const lyricPrefix	string = "lyric"

func newPool(address string) *redis.Pool {
	return &redis.Pool{
		MaxIdle: 80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", address)
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
}

func setSession(sessionId string, s session) error {
	json, err := json.Marshal(s)
	if err != nil {
		return err
	}

	_, err = conn.Do("SETEX", sessionPrefix+":"+sessionId, sessionLength, json)
	if err != nil {
		return err
	}

	return nil
}

func getSessionFromRedis(sessionId string) (*session, error) {
	tmp, err := redis.String(conn.Do("GET", sessionPrefix+":"+sessionId))
	if err != nil {
		return nil, err
	}

	s := session{}
	err = json.Unmarshal([]byte(tmp), &s)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

func deleteSession(sessionId string) error {
	_, err := conn.Do("DEL", sessionPrefix+sessionId)
	if err != nil {
		return err
	}
	return nil
}