package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"text/template"

	_ "github.com/lib/pq"

	"github.com/microcosm-cc/bluemonday"

	"github.com/gomodule/redigo/redis"

	"github.com/julienschmidt/httprouter"
)

const host = "http://192.168.1.70"
const dbuser = "dsanon"
const dbpwd = "86a8c07323ea5e56dc8e8ed70191a04cea0c2daa7030993d01d8ba3e64076bc2"

var sessionDB redis.Conn
var templates *template.Template
var sanitizer *bluemonday.Policy
var db *sql.DB

type jsonResponse struct {
	Status string
	Cookie *http.Cookie
}

type userInfo struct {
	Username string
}

type pageInfo struct {
	Element string
}

func init() {
	var err error
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
	sanitizer = bluemonday.StrictPolicy()
	templates = template.Must(template.ParseGlob("templates/*.html"))
}

func main() {
	router := httprouter.New()
	router.GET("/", index)
	router.GET("/signup", signupPage)
	router.POST("/signup", signupHandler)
	router.GET("/login", loginPage)
	router.GET("/logout", logoutHandler)
	router.POST("/login", loginHandler)
	router.Handler(http.MethodGet, "/assets/*filepath", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	log.Fatal(http.ListenAndServe(":8080", router))
}

func index(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var err error
	if alreadyLoggedIn(req) {
		usr := getUserInfo(req)
		data := struct {
			User userInfo
		}{User: usr}
		err = templates.ExecuteTemplate(res, "index.html", data)
	} else {
		err = templates.ExecuteTemplate(res, "index.html", nil)
	}
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func signupPage(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	err := templates.ExecuteTemplate(res, "signup.html", nil)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

func loginPage(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	err := templates.ExecuteTemplate(res, "login.html", nil)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handle logins
func loginHandler(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	username := req.FormValue("username")
	password := req.FormValue("password")
	resb, err := http.PostForm(host+":8081/login",
		url.Values{
			"username": {username},
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

	err = sessionSetKey(response.Cookie.Value, username)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	resb, err = http.PostForm(host+":8082/start",
		url.Values{
			"filename": {username},
			"password": {password},
		})
	if InternalServerError(res, req, err) {
		return
	}
	defer resb.Body.Close()
	bs, err = ioutil.ReadAll(resb.Body)
	if InternalServerError(res, req, err) {
		return
	}

	err = json.Unmarshal(bs, &response)
	if err != nil || response.Status != "OK" {
		http.Error(res, response.Status, http.StatusInternalServerError)
		return
	}

	http.SetCookie(res, response.Cookie)
	http.Redirect(res, req, host+":8080", http.StatusSeeOther)
}

func logoutHandler(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if !alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
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

func signupHandler(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	username := req.FormValue("username")
	password := req.FormValue("password")
	if len(username) < 1 || len(password) < 1 {
		InternalServerError(res, req, errors.New("Incorrect Username/Password format"))
		return
	}
	resb, err := http.PostForm(host+":8081/signup",
		url.Values{"username": {username}, "password": {password}})
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
		http.Error(res, response.Status, http.StatusForbidden)
		return
	}
	http.Redirect(res, req, host+":8080/login", http.StatusSeeOther)
}

func getUserInfo(req *http.Request) userInfo {
	info := userInfo{}
	if c, err := req.Cookie("session"); err == nil {
		info.Username, _ = sessionGetKey(c.Value)
	}
	return info
}
