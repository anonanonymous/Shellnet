package main

import (
	"encoding/json"
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
	URI      string
	Element  string
	Messages map[string]interface{}
}

// InternalServerError - handle internal server errors
func InternalServerError(res http.ResponseWriter, req *http.Request, err error) bool {
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// authMessage - displays the specified page with a message
func authMessage(res http.ResponseWriter, message, page, spec string) error {
	pg := pageInfo{
		URI:      hostURI,
		Element:  page,
		Messages: map[string]interface{}{spec: message},
	}
	data := struct {
		PageAttr pageInfo
	}{PageAttr: pg}
	if spec == "error" {
		res.WriteHeader(http.StatusUnauthorized)
	} else {
		res.WriteHeader(http.StatusFound)
	}
	return templates.ExecuteTemplate(res, "login.html", data)
}

// alreadyLoggedIn - checks if the user is already logged in
func alreadyLoggedIn(res http.ResponseWriter, req *http.Request) bool {
	usr := sessionGetKeys(req, "session")
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

// sessionGetKeys - retrieve info from cookie
func sessionGetKeys(req *http.Request, name string) *userInfo {
	cookie, err := req.Cookie(name)
	if err != nil {
		return nil
	}
	conn := sessionDB.Get()
	defer conn.Close()
	reply, err := redis.Strings(conn.Do("HMGET", cookie.Value, "username", "address"))
	if err != nil || len(reply) != 2 || reply[0] == "" {
		return nil
	}
	data := &userInfo{
		Username: reply[0],
		Address:  reply[1],
	}
	return data
}

// sessionDelKey - wrapper for redis DEL KEY - used for logout
func sessionDelKey(key string) error {
	conn := sessionDB.Get()
	defer conn.Close()
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
func walletCmd(cmd, param string) *jsonResponse {
	response := jsonResponse{}
	resb, err := http.Get(walletURI + "/" + cmd + "/" + param)
	if err != nil {
		return &jsonResponse{Status: err.Error()}
	}
	if err = json.NewDecoder(resb.Body).Decode(&response); err != nil {
		return &jsonResponse{Status: err.Error()}
	}
	response.Status = "OK"
	return &response
}

// walletStatusColor - green if synced, else orange
func walletStatusColor(res *jsonResponse) string {
	a := res.Data["status"].(map[string]interface{})["knownBlockCount"].(float64)
	b := res.Data["status"].(map[string]interface{})["blockCount"].(float64)
	if a-b < 5 && b > 0 {
		return "green-input"
	}
	return "orange-input"
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
