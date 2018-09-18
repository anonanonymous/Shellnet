package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
)

type jsonResponse struct {
	Status string
	Data   map[string]interface{}
}

type user struct {
	ID       int
	IH       string
	Username string
	Verifier string
	Address  string
}

// decodeResponse - decodes the json data from a Response
func decodeResponse(resb *http.Response) (map[string]interface{}, error) {
	response := map[string]interface{}{}
	err := json.NewDecoder(resb.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// InternalServerError - handle internal server errors
func InternalServerError(res http.ResponseWriter, req *http.Request, err error) bool {
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// isRegistered - check if username is already present in the database
func isRegistered(username string) bool {
	row := db.QueryRow("SELECT username FROM accounts WHERE username = $1;", username)
	err := row.Scan()
	return err != sql.ErrNoRows
}

// getUser - retrieves a user from the database
func getUser(username string) (*user, error) {
	row := db.QueryRow("SELECT ih, verifier, username, id, address FROM accounts WHERE username = $1;", username)
	usr := user{}
	err := row.Scan(&usr.IH, &usr.Verifier, &usr.Username, &usr.ID, &usr.Address)
	if err != nil {
		fmt.Println("err: ", err)
		return nil, err
	}
	return &usr, nil
}
