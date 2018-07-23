package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
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

func isRegistered(username string) bool {
	row := db.QueryRow("SELECT * FROM accounts WHERE username = $1;", username)
	err := row.Scan()
	return err != sql.ErrNoRows
}

func getUser(username string) (*user, error) {
	row := db.QueryRow("SELECT * FROM accounts WHERE username = $1;", username)
	usr := user{}
	err := row.Scan(&usr.IH, &usr.Verifier, &usr.Username, &usr.ID)
	if err != nil {
		fmt.Println("err: ", err)
		return nil, err
	}
	fmt.Println(usr)
	return &usr, nil
}

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

func sessionSetKey(key, val string) error {
	_, err := sessionDB.Do("SET", key, val)
	if err != nil {
		return err
	}
	return nil
}

func sessionDelKey(key string) error {
	_, err := sessionDB.Do("DEL", key)
	if err != nil {
		return err
	}
	return nil
}
