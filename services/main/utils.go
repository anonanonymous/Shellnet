package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gomodule/redigo/redis"
)

type jsonResponse struct {
	Status string
	Data   map[string]interface{}
}

type userInfo struct {
	Username string
	Address  string
}

type pageInfo struct {
	Element string
}

// InternalServerError - handle internal server errors
func InternalServerError(res http.ResponseWriter, req *http.Request, err error) bool {
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// alreadyLoggedIn - checks if the user is already logged in
func alreadyLoggedIn(res http.ResponseWriter, req *http.Request) bool {
	usr := sessionGetKeys(req)
	if usr == nil {
		cookie := &http.Cookie{
			Name:   "session",
			Value:  "",
			MaxAge: -1,
		}
		http.SetCookie(res, cookie)
		return false
	}
	return true
}

// wrapper for redis HMSET
func sessionSetKeys(key, uname, addr string) error {
	conn := sessionDB.Get()
	_, err := conn.Do(
		"HMSET", key,
		"username", uname,
		"address", addr,
		"EX", 1512000) // 420 hours
	return err
}

// sessionGetKeys - retrieve info from cookie
func sessionGetKeys(req *http.Request) *userInfo {
	cookie, err := req.Cookie("session")
	if err != nil {
		return nil
	}
	conn := sessionDB.Get()
	reply, err := redis.Strings(conn.Do("HMGET", cookie.Value, "username", "address"))
	if err != nil || len(reply) != 2 {
		return nil
	}
	data := &userInfo{
		Username: sanitizer.Sanitize(reply[0]),
		Address:  reply[1],
	}
	return data
}

// wrapper for redis DEL KEY - used for logout
func sessionDelKey(key string) error {
	conn := sessionDB.Get()
	_, err := conn.Do("DEL", key)

	return err
}

// tryAuth - try to signup/login with a given username and password
func tryAuth(username, password, method string) *jsonResponse {
	resb, err := http.PostForm(usrURI+"/"+method,
		url.Values{
			"username": {username},
			"password": {password},
		})
	if err != nil {
		return &jsonResponse{Status: err.Error()}
	}
	response, err := decodeResponse(resb)
	return response
}

// walletCmd - executes a wallet command and returns the result
func walletCmd(cmd, address string) *jsonResponse {
	response := jsonResponse{}
	fmt.Println(walletURI + "/" + cmd + "/" + address)
	resb, err := http.Get(walletURI + "/" + cmd + "/" + address)
	if err != nil {
		return &jsonResponse{Status: err.Error()}
	}
	if err = json.NewDecoder(resb.Body).Decode(&response); err != nil {
		return &jsonResponse{Status: err.Error()}
	}
	response.Status = "OK"
	return &response
}

// decodeResponse - decodes the json data from a Response
func decodeResponse(resb *http.Response) (*jsonResponse, error) {
	var response jsonResponse
	err := json.NewDecoder(resb.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return &response, nil
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
