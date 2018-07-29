package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/microcosm-cc/bluemonday"

	_ "github.com/lib/pq"

	"github.com/gomodule/redigo/redis"

	"github.com/julienschmidt/httprouter"
	"github.com/opencoff/go-srp"
)

const host = "http://192.168.1.70"
const port = ":8081"
const dbuser = "dsanon"
const dbpwd = "86a8c07323ea5e56dc8e8ed70191a04cea0c2daa7030993d01d8ba3e64076bc2"

var sanitizer *bluemonday.Policy
var sessionDB redis.Conn
var nBits = 1024
var srpEnv *srp.SRP
var db *sql.DB

type user struct {
	ID       int
	IH       string
	Username string
	Verifier string
	Address  string
}

func init() {
	var err error
	srpEnv, err = srp.New(nBits)
	if err != nil {
		panic(err)
	}
	sanitizer = bluemonday.StrictPolicy()

	sessionDB, err = redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}

	db, err = sql.Open("postgres", "postgres://"+dbuser+":"+dbpwd+"@localhost/users?sslmode=disable")
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
	router.DELETE("/user/:username", deleteUser)
	log.Fatal(http.ListenAndServe(port, router))
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
	// sponge
	fmt.Printf("v: %s, ih: %s\n", verif, ih)
	resb, err := http.Get(host + ":8082/address")
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}
	response, err := decodeResponse(resb)
	address := response["Data"].(map[string]interface{})["address"].(string)
	_, err = db.Exec("INSERT INTO accounts (ih, verifier, username, address) VALUES ($1, $2, $3, 4);", ih, verif, username, address)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
	} else {
		encoder.Encode(jsonResponse{Status: "OK"})
	}
}

// login - verify username/password and sends back a cookie
func login(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	encoder := json.NewEncoder(res)
	//todo verify that input is utf-8
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
		encoder.Encode(jsonResponse{Status: "Client proof failed"})
		return
	}
	if !client.ServerOk(proof) {
		encoder.Encode(jsonResponse{Status: "Server proof failed"})
		return
	}
	if 1 != subtle.ConstantTimeCompare(client.RawKey(), srv.RawKey()) {
		encoder.Encode(jsonResponse{Status: "Keys didn't match"})
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
