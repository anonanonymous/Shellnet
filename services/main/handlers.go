package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/julienschmidt/httprouter"
)

// InitHandlers - sets up the http handlers
func InitHandlers(r *httprouter.Router) {
	r.GET("/", limit(index, ratelimiter))
	r.GET("/tos", limit(terms, ratelimiter))
	r.GET("/login", limit(loginPage, ratelimiter))
	r.POST("/login", limit(loginHandler, strictRL))
	r.GET("/logout", limit(logoutHandler, ratelimiter))
	r.GET("/signup", limit(signupPage, ratelimiter))
	r.POST("/signup", limit(signupHandler, strictRL))
	r.GET("/account", limit(accountPage, ratelimiter))
	r.GET("/account/keys", limit(walletKeys, ratelimiter))
	r.POST("/account/delete", limit(deleteHandler, ratelimiter))
	r.GET("/account/wallet_info", limit(getWalletInfo, ratelimiter))
	r.POST("/account/export_keys", limit(keyHandler, ratelimiter))
	r.POST("/account/send_transaction", limit(sendHandler, ratelimiter))
	r.Handler(http.MethodGet, "/captcha/*name",
		captcha.Server(captcha.StdWidth, captcha.StdHeight))
	r.Handler(http.MethodGet, "/assets/*filepath", http.StripPrefix("/assets",
		http.FileServer(http.Dir("./assets"))))
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
		CaptchaID string
		PageAttr  pageInfo
	}{CaptchaID: captcha.New(), PageAttr: pg}
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
		CaptchaID string
		PageAttr  pageInfo
	}{CaptchaID: captcha.New(), PageAttr: pg}
	err := templates.ExecuteTemplate(res, "login.html", data)
	InternalServerError(res, req, err)
}

// loginHandler handles logins, redirects to account page on succeess - method: POST
func loginHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if alreadyLoggedIn(res, req) {
		http.Redirect(res, req, hostURI+"/account", http.StatusSeeOther)
		return
	}
	if !captcha.VerifyString(req.FormValue("captchaId"), req.FormValue("captchaSolution")) {
		http.Error(res, "Wrong captcha solution!", http.StatusForbidden)
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
		InternalServerError(res, req, authMessage(res, message, "login", "success"))
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

// terms - shows the terms of service
func terms(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	err := templates.ExecuteTemplate(res, "terms.html", nil)
	InternalServerError(res, req, err)
}
