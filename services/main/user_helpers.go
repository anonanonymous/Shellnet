package main

import (
	"encoding/json"
	"errors"
)

// RUser - user service response
type RUser struct {
	SessionID string `json:"sessionID"`
	Address   string `json:"address"`
}

// UserHTTP - defines http methods for user service
var UserHTTP = map[string]string{
	"signup": "POST",
	"login":  "POST",
	"update": "PUT",
	"delete": "DELETE",
}

// userReq - helper for user api calls
func userReq(route, username string, data jsMap) (*map[string]string, error) {
	var response map[string]string

	method := UserHTTP[route]
	route = userPath(route, username)
	b, err := sendRequest(method, userURI+route, makeJSONString(data))
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(*b, &response); err != nil {
		return nil, err
	}
	if response["status"] != "OK" {
		return nil, errors.New(response["status"])
	}

	return &response, nil
}

// userPath - resolves the user path
func userPath(route, username string) (path string) {
	switch route {
	case "login", "signup":
		path = "/" + route
	case "update", "delete":
		path = "/" + route + "/" + username
	}
	return path
}
