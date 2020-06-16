package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/dchest/captcha"
)

// sign - returns a HMAC signature for a message
func sign(message, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// checkServerError - handle internal server errors
func checkServerError(res http.ResponseWriter, req *http.Request, err error) bool {
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return true
	}
	return false
}

// authMessage - displays the specified page with a message
func authMessage(res http.ResponseWriter, message, page, spec string) error {
	data := TPage{
		TData: jsMap{
			"uri":       hostURI,
			"element":   page,
			"message":   jsMap{spec: message},
			"captchaID": captcha.New(),
		},
	}

	if spec == "error" {
		res.WriteHeader(http.StatusUnauthorized)
	} else {
		res.WriteHeader(http.StatusOK)
	}

	return templates.ExecuteTemplate(res, "login.html", data)
}

// alreadyLoggedIn - checks if the user is already logged in
func alreadyLoggedIn(res http.ResponseWriter, req *http.Request) (*User, error) {
	user, err := getUserFromSession(req)

	if err != nil {
		cookie := &http.Cookie{
			Name:   "session",
			Value:  "",
			MaxAge: -1,
		}
		http.SetCookie(res, cookie)
		return nil, err
	}

	return user, nil
}

// getUserFromSession - gets user details
func getUserFromSession(req *http.Request) (user *User, err error) {
	defer func() {
		if r := recover(); r != nil {
			user = nil
			err = errors.New(ERRORS[AUTH_ERROR])
		}
	}()

	c, err := req.Cookie("session")
	if err != nil {
		return nil, err
	}

	resp, err := redisHGetAll(c.Value)
	if err != nil || (*resp)["username"] == "" {
		return nil, errors.New("Session not found")
	}

	user = &User{
		Username:     (*resp)["username"],
		Address:      (*resp)["address"],
		TwoFAEnabled: (*resp)["two_fa_enabled"] != "",
	}

	return user, nil
}

// makeJSONString - converts a jsMap to a string
func makeJSONString(dict jsMap) string {
	result, err := json.Marshal(dict)
	if err != nil {
		return ""
	}
	return string(result)
}
