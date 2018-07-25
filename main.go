package main

import (
	"encoding/json"
	"errors"
	"fmt"
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

type jsonResponse struct {
	Status string
	Cookie *http.Cookie
	User   userInfo
}

type status struct {
	Result struct {
		BlockCount      int    `json:"blockCount"`
		KnownBlockCount int    `json:"knownBlockCount"`
		LastBlockHash   string `json:"lastBlockHash"`
		PeerCount       int    `json:"peerCount"`
	} `json:"result"`
}

type balance struct {
	Result struct {
		AvailableBalance int `json:"availableBalance"`
		LockedAmount     int `json:"lockedAmount"`
	} `json:"result"`
}

type transaction struct {
	Result struct {
		TransactionHash string `json:"transactionHash"`
	} `json:"result"`
}

type walletInfo struct {
	Balance balance
	Status  status
}

type userInfo struct {
	Username string
	Address  string
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
	sanitizer = bluemonday.StrictPolicy()
	templates = template.Must(template.ParseGlob("templates/*.html"))
}

func main() {
	router := httprouter.New()
	router.GET("/", index)
	router.GET("/signup", signupPage)
	router.GET("/account", accountPage)
	router.POST("/signup", signupHandler)
	router.GET("/login", loginPage)
	router.GET("/logout", logoutHandler)
	router.POST("/delete", deleteHandler)
	router.POST("/login", loginHandler)
	router.POST("/send_transaction", sendHandler)
	router.GET("/wallet", getWalletInfo)
	router.Handler(http.MethodGet, "/assets/*filepath", http.StripPrefix("/assets", http.FileServer(http.Dir("./assets"))))
	log.Fatal(http.ListenAndServe(":8080", router))
}

func index(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var err error
	if alreadyLoggedIn(req) {
		cookie, _ := req.Cookie("session")
		username, err := sessionGetKey(cookie.Value)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		resb, err := http.Get(host + ":8081/user/" + username)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resb.Body.Close()
		bs, err := ioutil.ReadAll(resb.Body)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		response := jsonResponse{}
		json.Unmarshal(bs, &response)
		response.User.Username = sanitizer.Sanitize(response.User.Username)
		data := struct {
			User userInfo
		}{User: response.User}
		err = templates.ExecuteTemplate(res, "index.html", data)
	} else {
		err = templates.ExecuteTemplate(res, "index.html", nil)
	}
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func accountPage(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	cookie, _ := req.Cookie("session")
	username, err := sessionGetKey(cookie.Value)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	resb, err := http.Get(host + ":8081/user/" + username)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resb.Body.Close()
	bs, err := ioutil.ReadAll(resb.Body)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	response := jsonResponse{}
	json.Unmarshal(bs, &response)
	data := struct {
		User userInfo
	}{User: response.User}
	err = templates.ExecuteTemplate(res, "account.html", data)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func signupPage(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	data := struct {
		PageAttr pageInfo
	}{pageInfo{Element: "signup"}}
	err := templates.ExecuteTemplate(res, "login.html", data)
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
	data := struct {
		PageAttr pageInfo
	}{pageInfo{Element: "login"}}
	err := templates.ExecuteTemplate(res, "login.html", data)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}

// handle logins
func loginHandler(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080/account", http.StatusSeeOther)
		return
	}
	username := req.FormValue("username")
	password := req.FormValue("password")

	response, err := tryAuth(username, password, "login")
	if err != nil {
		http.Error(res, err.Error(), http.StatusForbidden)
		return
	}

	err = sessionSetKey(response.Cookie.Value, username)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = startWallet(username, password, response.Cookie.Value)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(res, response.Cookie)
	http.Redirect(res, req, host+":8080/account", http.StatusSeeOther)
}

func deleteHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	resb, err := http.Get(host + ":8081/delete")
	if err != nil {
		return
	}
	defer resb.Body.Close()
	bs, err := ioutil.ReadAll(resb.Body)
	if err != nil {
		return
	}
	var response jsonResponse
	json.Unmarshal(bs, &response)
	if response.Status != "OK" {
		http.Error(res, response.Status, http.StatusForbidden)
		return
	}
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		MaxAge: -1,
	}
	http.SetCookie(res, cookie)
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

func signupHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080/account", http.StatusSeeOther)
		return
	}
	username := req.FormValue("username")
	password := req.FormValue("password")
	if len(username) < 1 || len(password) < 1 {
		InternalServerError(res, req, errors.New("Incorrect Username/Password format"))
		return
	}
	_, err := tryAuth(username, password, "signup")
	if err != nil {
		http.Error(res, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(res, req, host+":8080/login", http.StatusSeeOther)
}

func getWalletInfo(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	wallet := walletInfo{}
	cookie, _ := req.Cookie("session")

	json.Unmarshal(walletCmd("status", cookie.Value), &wallet.Status)
	json.Unmarshal(walletCmd("balance", cookie.Value), &wallet.Balance)
	json.NewEncoder(res).Encode(wallet)
}

func sendHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(req) {
		http.Redirect(res, req, host+":8080", http.StatusSeeOther)
		return
	}
	cookie, _ := req.Cookie("session")
	username, err := sessionGetKey(cookie.Value)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	resb, err := http.Get(host + ":8081/user/" + username)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resb.Body.Close()
	bs, err := ioutil.ReadAll(resb.Body)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	response := jsonResponse{}
	json.Unmarshal(bs, &response)
	resb, err = http.PostForm(host+":8082/send_transaction",
		url.Values{
			"amount":      {req.FormValue("amount")},
			"address":     {response.User.Address},
			"destination": {req.FormValue("destination")},
			"payment_id":  {req.FormValue("payment_id")},
		})
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resb.Body.Close()
	bs, err = ioutil.ReadAll(resb.Body)
	tx := transaction{}
	json.Unmarshal(bs, &tx)
	fmt.Println(tx.Result.TransactionHash)
}
