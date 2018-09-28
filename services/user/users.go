package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"

	"github.com/julienschmidt/httprouter"
	"github.com/opencoff/go-srp"
)

var (
	dbUser, dbPwd     string
	hostURI, hostPort string
	walletURI         string
	srpEnv            *srp.SRP
	db                *sql.DB
)

const nBits = 1024

func init() {
	var err error

	if dbUser = os.Getenv("DB_USER"); dbUser == "" {
		panic("Set the DB_USER env variable")
	}
	if dbPwd = os.Getenv("DB_PWD"); dbPwd == "" {
		panic("Set the DB_PWD env variable")
	}
	if hostURI = os.Getenv("HOST_URI"); hostURI == "" {
		hostURI = "http://localhost"
		println("Using default HOST_URI - http://localhost")
	}
	if hostPort = os.Getenv("HOST_PORT"); hostPort == "" {
		hostPort = ":8081"
		println("Using default HOST_PORT - 8081")
	}
	hostURI += hostPort

	if walletURI = os.Getenv("WALLET_URI"); walletURI == "" {
		panic("Set the WALLET_URI env variable")
	}
	srpEnv, err = srp.New(nBits)
	if err != nil {
		panic(err)
	}

	db, err = sql.Open("postgres", "postgres://"+dbUser+":"+dbPwd+"@localhost/users?sslmode=disable")
	if err != nil {
		panic(err)
	}
	if err = db.Ping(); err != nil {
		panic(err)
	}
	fmt.Println("You connected to your database.")
}

func main() {
	router := httprouter.New()
	router.POST("/signup", signup)
	router.POST("/login", login)
	router.GET("/delete/:username", deleteUser)
	log.Fatal(http.ListenAndServe(hostPort, router))
}

// signup - adds user to db
func signup(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// todo sanitize input
	encoder := json.NewEncoder(res)
	username := req.FormValue("username")
	password := req.FormValue("password")
	if isRegistered(username) {
		encoder.Encode(jsonResponse{Status: "Username taken"})
		return
	}

	v, err := srpEnv.Verifier([]byte(username), []byte(password))
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}
	ih, verif := v.Encode()
	resb, err := http.Get(walletURI + "/create")
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}
	response, err := decodeResponse(resb)
	address := response["Data"].(map[string]interface{})["address"].(string)
	_, err = db.Exec("INSERT INTO accounts (ih, verifier, username, address) VALUES ($1, $2, $3, $4);", ih, verif, username, address)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
	} else {
		encoder.Encode(jsonResponse{Status: "OK"})
	}
}

// login - verify username/password and sends back a sessionID
func login(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	encoder := json.NewEncoder(res)
	username := req.FormValue("username")
	password := req.FormValue("password")
	usr, err := getUser(username)
	if err != nil {
		encoder.Encode(jsonResponse{Status: "Username not found"})
		return
	}

	client, err := srpEnv.NewClient([]byte(username), []byte(password))
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}

	creds := client.Credentials()

	ih, A, err := srp.ServerBegin(creds)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}

	if usr.IH != ih {
		encoder.Encode(jsonResponse{Status: "IH's didn't match"})
		return
	}

	s, verif, err := srp.MakeSRPVerifier(usr.Verifier)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}

	srv, err := s.NewServer(verif, A)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}

	creds = srv.Credentials()

	cauth, err := client.Generate(creds)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}

	proof, ok := srv.ClientOk(cauth)
	if !ok {
		encoder.Encode(jsonResponse{Status: "Error: Incorrect Username/Password"})
		return
	}
	if !client.ServerOk(proof) {
		encoder.Encode(jsonResponse{Status: "Error: Incorrect Username/Password"})
		return
	}
	if 1 != subtle.ConstantTimeCompare(client.RawKey(), srv.RawKey()) {
		encoder.Encode(jsonResponse{Status: "Error: Incorrect Username/Password"})
		return
	}

	data := map[string]interface{}{
		"sessionID": hex.EncodeToString(A.Bytes()),
		"address":   usr.Address}
	encoder.Encode(jsonResponse{Status: "OK", Data: data})
}

// deleteUser - removes user from db, deletes user address from container
func deleteUser(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	encoder := json.NewEncoder(res)
	db.Exec("DELETE FROM accounts WHERE username = $1;", p.ByName("username"))
	encoder.Encode(jsonResponse{Status: "OK"})
}
