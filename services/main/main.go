package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/gomodule/redigo/redis"

	"github.com/julienschmidt/httprouter"
)

var (
	hostURI, hostPort string
	usrURI            string
	walletURI         string
	sessionDB         *redis.Pool
	templates         *template.Template
)

func init() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = ":6379"
	}
	if hostURI = os.Getenv("HOST_URI"); hostURI == "" {
		panic("Set the HOST_URI env variable")
	}
	if hostPort = os.Getenv("HOST_PORT"); hostPort == "" {
		hostPort = ":8080"
		println("Using default HOST_PORT - 8080")
	}
        // Removed this in favor of local nginx routing to 8080. 
	// To keep routing to specific port, include port in run.sh HOST_URI.
	//hostURI += hostPort

	if usrURI = os.Getenv("USER_URI"); usrURI == "" {
		panic("Set the USER_URI env variable")
	}
	if walletURI = os.Getenv("WALLET_URI"); walletURI == "" {
		panic("Set the WALLET_URI env variable")
	}

	templates = template.Must(template.ParseGlob("templates/*.html"))
	sessionDB = newPool(redisHost)
	cleanupHook()
}

func main() {
	router := httprouter.New()
	router.GET("/", index)
	router.GET("/login", loginPage)
	router.POST("/login", loginHandler)
	router.GET("/logout", logoutHandler)
	router.GET("/signup", signupPage)
	router.POST("/signup", signupHandler)
	router.GET("/account", accountPage)
	router.GET("/account/keys", walletKeys)
	router.POST("/account/delete", deleteHandler)
	router.GET("/account/wallet_info", getWalletInfo)
	router.POST("/account/export_keys", keyHandler)
	router.POST("/account/send_transaction", sendHandler)
	router.Handler(http.MethodGet, "/assets/*filepath", http.StripPrefix("/assets",
		http.FileServer(http.Dir("./assets"))))
	log.Fatal(http.ListenAndServe(hostPort, router))
}

// index displays homepage - method: GET
func index(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var err error
	if alreadyLoggedIn(res, req) {
		usr := sessionGetKeys(req, "session")
		if usr == nil {
			http.Error(res, "Couldn't find user session", http.StatusInternalServerError)
			return
		}
		data := struct {
			User userInfo
		}{User: *usr}
		err = templates.ExecuteTemplate(res, "index.html", data)
	} else {
		err = templates.ExecuteTemplate(res, "index.html", nil)
	}
	InternalServerError(res, req, err)
}

// accountPage - shows wallet info and stufffs
func accountPage(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	usr := sessionGetKeys(req, "session")
	if usr == nil {
		http.Error(res, "Couldn't find user session", http.StatusInternalServerError)
		return
	}
	walletResponse := walletCmd("status", usr.Address)
	if walletResponse.Status != "OK" {
		http.Error(res, "Error loading wallet status", http.StatusInternalServerError)
		return
	}
	walletIcon := walletStatusColor(walletResponse)

	pg := pageInfo{
		URI:      hostURI,
		Messages: map[string]interface{}{"wallet_icon": walletIcon},
	}
	if txHash, err := req.Cookie("transactionHash"); err == nil {
		pg.Messages["txHash"] = txHash.Value
		http.SetCookie(res, &http.Cookie{Name: "transactionHash", Path: "/account", MaxAge: -1})
	}

	txs := walletCmd("transactions/"+usr.Address, "0")
	data := struct {
		User         userInfo
		Wallet       map[string]interface{}
		PageAttr     pageInfo
		Transactions map[string]interface{}
	}{User: *usr, Wallet: walletResponse.Data, PageAttr: pg, Transactions: txs.Data}
	InternalServerError(res, req, templates.ExecuteTemplate(res, "account.html", data))
}

// signupPage - displays signup page - method: GET
func signupPage(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	pg := pageInfo{
		URI:     hostURI,
		Element: "signup",
	}
	data := struct {
		PageAttr pageInfo
	}{PageAttr: pg}
	err := templates.ExecuteTemplate(res, "login.html", data)
	InternalServerError(res, req, err)
}

// loginPage - displays login page - method: GET
func loginPage(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	pg := pageInfo{
		URI:     hostURI,
		Element: "login",
	}
	data := struct {
		PageAttr pageInfo
	}{PageAttr: pg}
	err := templates.ExecuteTemplate(res, "login.html", data)
	InternalServerError(res, req, err)
}

// loginHandler handles logins, redirects to account page on succeess - method: POST
func loginHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI+"/account", http.StatusSeeOther)
		return
	}
	username := req.FormValue("username")
	password := req.FormValue("password")

	response := tryAuth(username, password, "login")
	if response.Status != "OK" {
		InternalServerError(res, req, authMessage(res, response.Status, "login", "error"))
		return
	}
	cookie := &http.Cookie{
		Name:     "session",
		Value:    response.Data["sessionID"].(string),
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(time.Hour * 420),
	}
	address := response.Data["address"].(string)
	err := sessionSetKeys(cookie.Value, username, address)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(res, cookie)
	http.Redirect(res, req, hostURI+"/account", http.StatusSeeOther)
}

// deleteHandler - deletes user from database and deletes wallet
func deleteHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	usr := sessionGetKeys(req, "session")
	if usr == nil {
		http.Error(res, "Couldn't find user session", http.StatusInternalServerError)
		return
	}
	go walletCmd("delete", usr.Address)
	go http.Get(usrURI + "/delete/" + usr.Username)
	cookie := &http.Cookie{
		Name:   "session",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(res, cookie)
	http.Redirect(res, req, hostURI, http.StatusSeeOther)
}

// logoutHandler - removes the user cookie from redis - method: GET
func logoutHandler(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if !alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	cookie, _ := req.Cookie("session")
	go sessionDelKey(cookie.Value)

	cookie = &http.Cookie{
		Name:   "session",
		Value:  "",
		MaxAge: -1,
	}

	http.SetCookie(res, cookie)
	http.Redirect(res, req, hostURI, http.StatusSeeOther)
}

// signupHandler tries to add a new user - method: POST
func signupHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	var message string
	username := req.FormValue("username")
	password := req.FormValue("password")
	verifyPassword := req.FormValue("verify_password")

	if len(username) < 1 || len(password) < 1 || len(username) > 64 {
		message = "Incorrect Username/Password format"
	} else if password != verifyPassword {
		message = "Passwords do not match"
	} else if response := tryAuth(username, password, "signup"); response.Status != "OK" {
		message = "Could not create account. Try again"
	}

	if message != "" {
		InternalServerError(res, req, authMessage(res, message, "signup", "error"))
	} else {
		message = "Account Created, Please Log In"
        http.Redirect(res, req, hostURI, http.StatusSeeOther)
		//InternalServerError(res, req, authMessage(res, message, "login", "success"))
	}
}

// getWalletInfo - gets wallet info
func getWalletInfo(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	usr := sessionGetKeys(req, "session")
	if usr == nil {
		http.Error(res, "Couldn't find user session", http.StatusInternalServerError)
		return
	}
	response := walletCmd("status", usr.Address)
	if response.Status == "OK" {
		json.NewEncoder(res).Encode(response)
	}
}

// sendHandler - sends a transaction
func sendHandler(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	if !alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	var message string
	usr := sessionGetKeys(req, "session")
	if usr == nil {
		http.Error(res, "Couldn't find user session", http.StatusInternalServerError)
		return
	}

	resb, err := http.PostForm(walletURI+"/send_transaction",
		url.Values{
			"amount":      {req.FormValue("amount")},
			"address":     {usr.Address},
			"destination": {strings.TrimSpace(req.FormValue("destination"))},
			"payment_id":  {req.FormValue("payment_id")},
		})
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	response := jsonResponse{}
	json.NewDecoder(resb.Body).Decode(&response)
	if response.Status != "OK" {
		message = "Error!: " + response.Status
	} else {
		message = response.Data["result"].(map[string]interface{})["transactionHash"].(string)
	}
	c := &http.Cookie{
		Name:  "transactionHash",
		Path:  "/account",
		Value: message,
	}
	http.SetCookie(res, c)
	http.Redirect(res, req, hostURI+"/account", http.StatusSeeOther)
}

// keyHandler - shows the wallet keys of a user
func keyHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	usr := sessionGetKeys(req, "session")
	password := req.FormValue("password")
	response := tryAuth(usr.Username, password, "login")
	if response.Status != "OK" {
		http.Error(res, "Authentication Failed", http.StatusInternalServerError)
		return
	}
	c := &http.Cookie{
		Name:     "key",
		Value:    response.Data["sessionID"].(string),
		HttpOnly: true,
		Path:     "/account/keys",
	}
	http.SetCookie(res, c)
	sessionSetKeys(response.Data["sessionID"].(string), usr.Username, usr.Address)
	http.Redirect(res, req, hostURI+"/account/keys", http.StatusSeeOther)
}

// walletKeys - shows the wallet keys
func walletKeys(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if !alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}
	usr := sessionGetKeys(req, "key")
	if usr == nil {
		http.Error(res, "No hackermans allowed", http.StatusInternalServerError)
		return
	}
	cookie, _ := req.Cookie("key")
	go sessionDelKey(cookie.Value)
	http.SetCookie(res, &http.Cookie{Name: "key", Path: "/account/keys", MaxAge: -1})

	keys := walletCmd("export_keys", usr.Address)

	data := struct {
		User userInfo
		Keys map[string]interface{}
	}{User: userInfo{Username: usr.Username}, Keys: keys.Data}
	err := templates.ExecuteTemplate(res, "keys.html", data)
	InternalServerError(res, req, err)
}
