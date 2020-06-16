package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/julienschmidt/httprouter"
)

// InitHandlers - registers the http handlers
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
	r.Handler("GET", "/captcha/*captchaID", captcha.Server(
		captcha.StdWidth,
		captcha.StdHeight,
	))
	r.Handler(http.MethodGet, "/assets/*filepath", http.StripPrefix(
		"/assets",
		http.FileServer(http.Dir("./assets")),
	))
}

// index displays homepage - method: GET
func index(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var err error
	var data interface{}

	if u, err := alreadyLoggedIn(res, req); err == nil {
		data = TPage{
			TUser: *u,
		}
	}

	err = templates.ExecuteTemplate(res, "index.html", data)
	if err != nil {
		log.Println(err)
	}
}

// accountPage - shows account and wallet info
func accountPage(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var user *User

	if u, err := alreadyLoggedIn(res, req); err == nil {
		user = u
	} else {
		errorTemplate(res, "Could not find user session", AUTH_ERROR)
		return
	}

	params := req.URL.Query()
	walletInfo, err := walletReq("status", user.Address, nil)
	if err != nil {
		log.Println(err)
		errorTemplate(res, ERRORS[WALLET_ERROR], AUTH_ERROR)
		return
	}

	color := walletStatusColor(walletInfo.Status)
	txs, err := walletReq("transactions", user.Address, nil, "")
	if err != nil {
		log.Println(err)
		errorTemplate(res, ERRORS[WALLET_ERROR], AUTH_ERROR)
		return
	}

	pd := TPage{
		TUser:   *user,
		TWallet: *walletInfo,
		TData: jsMap{
			"uri":          hostURI,
			"transactions": txs.Transactions,
			"wallet_icon":  color,
			"url_params":   params,
		},
	}

	// transaction hash/error  modal data
	if txHash, err := req.Cookie("txHash"); err == nil {
		pd.TData["txHash"] = txHash.Value
		http.SetCookie(res, &http.Cookie{
			Name:   "txHash",
			Path:   "/account",
			MaxAge: -1,
		})
	}
	if cookie, err := req.Cookie("error"); err == nil {
		pd.TData["error"] = cookie.Value
		http.SetCookie(res, &http.Cookie{
			Name:   "error",
			Path:   "/account",
			MaxAge: -1,
		})
	}

	err = templates.ExecuteTemplate(res, "account.html", pd)
	if err != nil {
		log.Println(err)
	}
}

// signupPage - displays signup page - method: GET
func signupPage(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if _, err := alreadyLoggedIn(res, req); err == nil {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}

	pd := TPage{
		TData: jsMap{
			"uri":       hostURI,
			"element":   "signup",
			"captchaID": captcha.New(),
		},
	}

	err := templates.ExecuteTemplate(res, "login.html", pd)
	if err != nil {
		log.Println(err)
	}
}

// loginPage - displays login page - method: GET
func loginPage(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if _, err := alreadyLoggedIn(res, req); err == nil {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}

	pd := TPage{
		TData: jsMap{
			"uri":       hostURI,
			"element":   "login",
			"captchaID": captcha.New(),
		},
	}

	err := templates.ExecuteTemplate(res, "login.html", pd)
	if err != nil {
		log.Println(err)
	}
}

// loginHandler handles logins method: POST
func loginHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	if _, err := alreadyLoggedIn(res, req); err == nil {
		http.Redirect(res, req, hostURI+"/account", http.StatusForbidden)
		return
	}
	if !captcha.VerifyString(req.FormValue("captchaID"), req.FormValue("captchaSolution")) {
		errorTemplate(res, "Wrong captcha solution!", http.StatusForbidden)
		return
	}

	username := req.FormValue("username")
	password := req.FormValue("password")
	resp, err := userReq("login", "", jsMap{
		"username": username,
		"password": password,
	})
	if err != nil {
		log.Println("loginHandler: ", err)
		errorTemplate(res, err.Error(), http.StatusForbidden)
		return
	}

	cookie := &http.Cookie{
		Name:     "session",
		Value:    (*resp)["sessionID"],
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Now().Add(time.Hour * sessionDuration),
	}
	address := (*resp)["address"]

	redisSAdd(address, []string{cookie.Value})
	err = redisHMSet(cookie.Value, jsMap{
		"username": username,
		"address":  address,
	})
	if err != nil {
		errorTemplate(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.SetCookie(res, cookie)
	http.Redirect(res, req, hostURI+"/account", http.StatusSeeOther)
}

// deleteHandler - deletes user from database and deletes wallet
func deleteHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var user *User

	if u, err := alreadyLoggedIn(res, req); err == nil {
		user = u
	} else {
		errorTemplate(res, "Could not find user session", http.StatusForbidden)
		return
	}

	go walletReq("delete", user.Address, nil)
	go userReq("delete", user.Username, nil)

	sessions, _ := redisSMembers(user.Address)
	for _, sess := range sessions {
		redisSRem(user.Address, sess)
		redisDel(sess)
	}
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
	if _, err := alreadyLoggedIn(res, req); err != nil {
		errorTemplate(res, "Could not find user session", http.StatusForbidden)
		return
	}

	cookie, _ := req.Cookie("session")
	go redisDel(cookie.Value)

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
	var message string

	if _, err := alreadyLoggedIn(res, req); err == nil {
		http.Redirect(res, req, hostURI, http.StatusSeeOther)
		return
	}

	username := req.FormValue("username")
	password := req.FormValue("password")
	verifyPassword := req.FormValue("verify_password")

	// uggo
	if len(username) < 1 || len(password) < 1 || len(username) > 64 {
		message = "Incorrect Username/Password format"
	} else if password != verifyPassword {
		message = "Passwords do not match"
	} else if response, err := userReq("signup", "", jsMap{
		"username": username,
		"password": password,
	}); err != nil {
		log.Println((*response)["status"])
		message = "Could not create account. Try again"
	}

	if message != "" {
		errorTemplate(res, message, http.StatusInternalServerError)
	} else {
		message = "Account created. Please log in " + username
		authMessage(res, message, "login", "success")
	}
}

// getWalletInfo - gets wallet info
func getWalletInfo(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var user *User

	if u, err := alreadyLoggedIn(res, req); err == nil {
		user = u
	} else {
		errorTemplate(res, "Could not find user session", http.StatusForbidden)
		return
	}
	println(user.Address)
	response, err := walletReq("status", user.Address, nil)
	if err != nil {
		errorTemplate(res, ERRORS[WALLET_ERROR], WALLET_ERROR)
		return
	}

	if err == nil {
		json.NewEncoder(res).Encode(jsMap{
			"balance": *response.Balance,
			"status":  *response.Status,
		})
	}
}

// sendHandler - sends a transaction
func sendHandler(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var message string
	var user *User
	var name = "txHash"

	if u, err := alreadyLoggedIn(res, req); err == nil {
		user = u
	} else {
		errorTemplate(res, "Could not find user session", http.StatusForbidden)
		return
	}

	resp, err := walletReq("send", user.Address, jsMap{
		"amount":      req.FormValue("amount"),
		"address":     user.Address,
		"destination": strings.TrimSpace(req.FormValue("destination")),
		"payment_id":  req.FormValue("payment_id"),
	})
	if err != nil {
		name = "error"
		message = "Sending transaction failed"
		log.Println(user.Username, err)
	} else {
		message = *resp.TxHash
	}

	c := &http.Cookie{
		Name:  name,
		Path:  "/account",
		Value: message,
	}

	http.SetCookie(res, c)
	http.Redirect(res, req, hostURI+"/account", http.StatusSeeOther)
}

// keyHandler - shows the wallet keys of a user
func keyHandler(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var user *User

	if u, err := alreadyLoggedIn(res, req); err == nil {
		user = u
	} else {
		errorTemplate(res, "Could not find user session", http.StatusForbidden)
		return
	}

	password := req.FormValue("password")
	response, err := userReq("login", "", jsMap{
		"username": user.Username,
		"password": password,
	})
	if err != nil {
		log.Println(err)
		errorTemplate(res, "Authentication Failed", http.StatusForbidden)
		return
	}

	c := &http.Cookie{
		Name:     "key",
		Value:    (*response)["sessionID"],
		HttpOnly: true,
		Path:     "/account/keys",
	}

	http.SetCookie(res, c)
	err = redisHMSet((*response)["sessionID"], jsMap{
		"username":       user.Username,
		"address":        user.Address,
		"two_fa_enabled": user.TwoFAEnabled,
	})
	if err != nil {
		log.Println(err)
		errorTemplate(res, ERRORS[WALLET_ERROR], WALLET_ERROR)
		return
	}

	http.Redirect(res, req, hostURI+"/account/keys", http.StatusSeeOther)
}

// walletKeys - shows the wallet keys
func walletKeys(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var user *User

	if u, err := alreadyLoggedIn(res, req); err == nil {
		user = u
	} else {
		errorTemplate(res, "Could not find user session", http.StatusForbidden)
		return
	}

	cookie, _ := req.Cookie("key")
	go redisDel(cookie.Value)
	http.SetCookie(res, &http.Cookie{
		Name:   "key",
		Path:   "/account/keys",
		MaxAge: -1,
	})

	keys, err := walletReq("keys", user.Address, nil)
	if err != nil {
		log.Println("walletKeys:", err)
		errorTemplate(res, ERRORS[AUTH_ERROR], AUTH_ERROR)
		return
	}

	data := TPage{
		TUser: *user,
		TData: jsMap{
			"publicSpendKey":  (*keys.Keys)["publicSpendKey"],
			"privateSpendKey": (*keys.Keys)["privateSpendKey"],
			"viewKey":         (*keys.Keys)["viewKey"],
		},
	}
	err = templates.ExecuteTemplate(res, "keys.html", data)
	if err != nil {
		log.Println(err)
	}
}

// terms - shows the terms of service
func terms(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	templates.ExecuteTemplate(res, "terms.html", nil)
}
