package main

import (
	json2 "encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
)

const sessionPrefix string = "session:"

func newPool() *redis.Pool {
	return &redis.Pool{
		MaxIdle: 80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ":6379")
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
}

func setSession(sessionId string, s session) error {
	json, err := json2.Marshal(s)
	if err != nil {
		return err
	}

	_, err = conn.Do("SET", sessionPrefix+sessionId, json)
	if err != nil {
		return err
	}

	return nil
}

func getSessionFromRedis(sessionId string) (*session, error) {
	tmp, err := redis.String(conn.Do("GET", sessionPrefix+sessionId))
	if err != nil {
		return nil, err
	}

	s := session{}
	err = json2.Unmarshal([]byte(tmp), &s)
	if err != nil {
		return nil, err
	}

	return &s, nil

}

func pingRedis() {
	pool := newPool()
	conn := pool.Get()
	defer conn.Close()
	err := ping(conn)
	if err != nil {
		fmt.Println(err)
	}
}

func ping(c redis.Conn) error {
	// Send PING command to Redis
	pong, err := c.Do("PING")
	if err != nil {
		return err
	}

	// PING command returns a Redis "Simple String"
	// Use redis.String to convert the interface type to string
	s, err := redis.String(pong, err)
	if err != nil {
		return err
	}

	fmt.Printf("PING Response = %s\n", s)
	// Output: PONG

	return nil
}