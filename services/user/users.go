package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	_ "github.com/lib/pq"

	"github.com/gomodule/redigo/redis"

	"github.com/julienschmidt/httprouter"
	"github.com/opencoff/go-srp"
)

const host = "http://192.168.1.70"
const port = ":8081"
const dbuser = "dsanon"
const dbpwd = "86a8c07323ea5e56dc8e8ed70191a04cea0c2daa7030993d01d8ba3e64076bc2"

var sessionDB redis.Conn
var userDB = map[string]user{}
var nBits = 1024
var srpEnv *srp.SRP
var db *sql.DB

type user struct {
	ID       int
	IH       string
	Username string
	Verifier string
}

type jsonResponse struct {
	Status string
	Cookie *http.Cookie
}

func init() {
	var err error
	srpEnv, err = srp.New(nBits)
	if err != nil {
		panic(err)
	}

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
	router.GET("/logout", logout)
	router.POST("/signup", signup)
	router.POST("/login", login)
	router.DELETE("/users/:username", deleteUser)
	log.Fatal(http.ListenAndServe(port, router))
}

// adds user to db
func signup(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	// todo sanitize input sanitize(s string)
	encoder := json.NewEncoder(res)
	username := req.FormValue("username")
	password := req.FormValue("password")
	//  ./walletd -g -w '#!<uname>' -p '<pwd>'
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
	resb, err := http.PostForm(host+":8082/create",
		url.Values{
			"filename": {username},
			"password": {password},
		})
	if InternalServerError(res, req, err) {
		return
	}
	defer resb.Body.Close()
	bs, err := ioutil.ReadAll(resb.Body)
	if InternalServerError(res, req, err) {
		return
	}

	var response jsonResponse
	err = json.Unmarshal(bs, &response)
	if err != nil || response.Status != "OK" {
		http.Error(res, response.Status, http.StatusInternalServerError)
		return
	}
	// sponge
	fmt.Printf("v: %s, ih: %s\n", verif, ih)

	_, err = db.Exec("INSERT INTO accounts (ih, verifier, username) VALUES ($1, $2, $3);", ih, verif, username)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}
	encoder.Encode(jsonResponse{Status: "OK"})
}

// login a user
func login(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	encoder := json.NewEncoder(res)
	//todo verify that input is utf-8
	username := req.FormValue("username")
	password := req.FormValue("password")
	usr, err := getUser(username)
	if err != nil {
		err = encoder.Encode(jsonResponse{Status: "Username not found"})
		if err != nil {
			fmt.Println(err.Error())
		}
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
		encoder.Encode(jsonResponse{Status: "ID's didn't match"})
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

	cookie := &http.Cookie{
		Name:     "session",
		Value:    hex.EncodeToString(A.Bytes()),
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(time.Hour * 420),
	}

	encoder.Encode(jsonResponse{Status: "OK", Cookie: cookie})
}

// remove user session from session db and remove user cookie
func logout(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	cookie, _ := req.Cookie("session")
	err := sessionDelKey(cookie.Value)
	if InternalServerError(res, req, err) {
		return
	}

	cookie = &http.Cookie{
		Name:   "session",
		Value:  "",
		MaxAge: -1,
	}
	http.SetCookie(res, cookie)

	http.Redirect(res, req, host+":8080", http.StatusSeeOther)
}

// delete user from db
func deleteUser(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if !alreadyLoggedIn(req) {
		http.Error(res, "Not logged in", http.StatusForbidden)
		return
	}
	cookie, _ := req.Cookie("session")
	username, err := sessionGetKey(cookie.Value)
	if err != nil {
		http.Error(res, "Error with cookie", http.StatusForbidden)
		return
	}

	sessionDelKey(cookie.Value)
	db.Exec("DELETE FROM accounts WHERE username = $1;", username)

	http.Redirect(res, req, host+":8080", http.StatusSeeOther)
}
