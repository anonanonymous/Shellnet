package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
)

// InternalServerError - handle internal server errors
func InternalServerError(res http.ResponseWriter, req *http.Request, err error) bool {
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// check if user is already logged in
func alreadyLoggedIn(req *http.Request) bool {
	c, err := req.Cookie("session")
	if err != nil {
		return false
	}
	_, err = sessionGetKey(c.Value)
	return err == nil
}

// wrapper for redis SET KEY VAL
func sessionSetKey(key, val string) error {
	_, err := sessionDB.Do("SET", key, val, "EX", 1512000)
	if err != nil {
		return err
	}
	return nil
}

// retrieve username from sessionid
func sessionGetKey(key string) (string, error) {
	reply, err := sessionDB.Do("GET", key)
	if err != nil {
		return "", err
	}
	if reply == nil {
		return "", errors.New("Key not found")
	}
	username := string(reply.([]byte))
	return username, nil
}

// wrapper for redis DEL KEY - used for logout
func sessionDelKey(key string) error {
	_, err := sessionDB.Do("DEL", key)
	if err != nil {
		return err
	}
	return nil
}

// try to signup/login with a given username and password
func tryAuth(username, password, method string) (*jsonResponse, error) {
	resb, err := http.PostForm(host+":8081/"+method,
		url.Values{
			"username": {username},
			"password": {password},
		})
	if err != nil {
		return nil, err
	}
	defer resb.Body.Close()

	bs, err := ioutil.ReadAll(resb.Body)
	if err != nil {
		return nil, err
	}

	var response jsonResponse
	err = json.Unmarshal(bs, &response)
	if err != nil {
		return nil, err
	} else if response.Status != "OK" {
		return nil, errors.New(response.Status)
	}
	return &response, nil
}

// start running the users wallet daemon
func startWallet(username, password, rpcPassword string) (*jsonResponse, error) {
	resb, err := http.PostForm(host+":8082/start",
		url.Values{
			"filename":     {username},
			"password":     {password},
			"rpc_password": {rpcPassword},
		})
	if err != nil {
		return nil, err
	}
	defer resb.Body.Close()

	bs, err := ioutil.ReadAll(resb.Body)
	if err != nil {
		return nil, err
	}

	var response jsonResponse
	err = json.Unmarshal(bs, &response)

	if err != nil {
		return nil, err
	} else if response.Status != "OK" {
		return nil, errors.New(response.Status)
	}
	return &response, nil
}

// executes a wallet command and returns the result
func walletCmd(cmd, sessID string) []byte {
	resb, err := http.Get(host + ":8082/" + cmd + "/" + sessID)
	if err != nil {
		return nil
	}
	defer resb.Body.Close()
	bs, err := ioutil.ReadAll(resb.Body)
	if err != nil {
		return nil
	}
	return bs
}
