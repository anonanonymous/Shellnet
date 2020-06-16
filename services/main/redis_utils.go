package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gomodule/redigo/redis"
)

/*
// wrapper for redis HMSET for auth
func sessionSetKeys(key, uname, addr string) error {
	conn := sessionDB.Get()
	defer conn.Close()
	_, err := conn.Do(
		"HMSET", key,
		"username", uname,
		"address", addr,
		"EX", 1512000) // 420 hours
	return err
}
*/
// wrapper for redis SADD
func redisSAdd(key string, members []string) error {
	args := []interface{}{key}
	for _, v := range members {
		args = append(args, v)
	}
	conn := sessionDB.Get()
	defer conn.Close()
	_, err := conn.Do(
		"SADD", args...,
	)
	return err
}

// wrapper for redis SREM
func redisSRem(key, val string) error {
	conn := sessionDB.Get()
	defer conn.Close()
	_, err := conn.Do(
		"SREM", key, val,
	)
	return err
}

// wrapper for redis SMEMBERS
func redisSMembers(key string) ([]string, error) {
	conn := sessionDB.Get()

	defer conn.Close()
	reply, err := redis.Strings(conn.Do(
		"SMEMBERS", key,
	))
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// wrapper for redis EXPIRE
func redisExpire(name string, val int64) error {
	conn := sessionDB.Get()
	defer conn.Close()
	_, err := conn.Do(
		"EXPIRE", name, val,
	)
	return err
}

// wrapper for redis DEL
func redisDel(key string) error {
	conn := sessionDB.Get()
	defer conn.Close()
	_, err := conn.Do(
		"DEL", key,
	)
	return err
}

// wrapper for redis HMSET
func redisHMSet(name string, pairs map[string]interface{}) error {
	args := []interface{}{name}
	for f, v := range pairs {
		args = append(args, f, v)
	}
	conn := sessionDB.Get()
	defer conn.Close()
	_, err := conn.Do(
		"HMSET", args...,
	)
	return err
}

// wrapper for redis HGETALL
func redisHGetAll(key string) (*map[string]string, error) {
	conn := sessionDB.Get()
	defer conn.Close()
	res, err := redis.StringMap(conn.Do(
		"HGETALL", key,
	))
	return &res, err
}

// wrapper for redis HINCRBY
func redisHIncrBy(key, field string, val int64) error {
	args := []interface{}{key, field, val}
	conn := sessionDB.Get()
	defer conn.Close()
	_, err := conn.Do(
		"HINCRBY", args...,
	)
	return err
}

// wrapper for redis HMGET
func redisHMGet(key string, fields []string) ([]string, error) {
	args := []interface{}{key}
	for _, f := range fields {
		args = append(args, f)
	}
	conn := sessionDB.Get()
	defer conn.Close()
	res, err := redis.Strings(conn.Do(
		"HMGET", args...,
	))
	return res, err
}

// newPool - creates and initializes a redis pool
func newPool(server string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,

		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", server)
			if err != nil {
				return nil, err
			}
			return c, err
		},

		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

// cleanipHook - close redis pool on exit
func cleanupHook() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)
	signal.Notify(c, syscall.SIGKILL)
	go func() {
		<-c
		sessionDB.Close()
		os.Exit(0)
	}()
}
