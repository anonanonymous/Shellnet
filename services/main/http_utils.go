package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
)

// httpsRedirect - redirects http to https
func httpsRedirect(res http.ResponseWriter, req *http.Request) {
	newURI := "https://" + req.Host + req.RequestURI
	http.Redirect(res, req, newURI, http.StatusPermanentRedirect)
}

// limit - rate limiter middleware
func limit(h httprouter.Handle, rl *stdlib.Middleware) httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
		context, err := rl.Limiter.Get(req.Context(), rl.Limiter.GetIPKey(req))
		if err != nil {
			rl.OnError(res, req, err)
			return
		}

		res.Header().Add("X-RateLimit-Limit", strconv.FormatInt(context.Limit, 10))
		res.Header().Add("X-RateLimit-Remaining", strconv.FormatInt(context.Remaining, 10))
		res.Header().Add("X-RateLimit-Reset", strconv.FormatInt(context.Reset, 10))

		if context.Reached {
			rl.OnLimitReached(res, req)
			return
		}
		res.Header().Set("Access-Control-Allow-Origin", "*")
		req.Body = http.MaxBytesReader(res, req.Body, 2048) // limit post size
		h(res, req, p)
	}
}

// sendRequest - helper function for sending data with an hmac signature
func sendRequest(method, uri, data string) (*[]byte, error) {
	var rawData []byte

	req, err := http.NewRequest(method, uri, bytes.NewBufferString(data))
	if err != nil {
		return nil, err
	}

	req.Header.Add("HMAC-SIGNATURE", sign(data, apiKEY))
	req.Header.Add("Content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	req.Close = true
	defer resp.Body.Close()

	rawData, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &rawData, nil
}

// errorTemplate - displays the error page
func errorTemplate(res http.ResponseWriter, err string, code int) {
	data := struct {
		Error    string
		HTTPCode int
	}{
		Error:    err,
		HTTPCode: code,
	}
	res.WriteHeader(code)
	templates.ExecuteTemplate(res, "error.html", data)
}
