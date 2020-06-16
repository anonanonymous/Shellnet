package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// sendRequest - helper function for sending data with an hmac signature
func sendRequest(method, uri, data string) (*map[string]interface{}, *[]byte, error) {
	var rawData []byte
	var body map[string]interface{}

	req, err := http.NewRequest(method, uri, bytes.NewBufferString(data))
	if err != nil {
		return nil, nil, err
	}

	//req.Header.Add("HMAC-SIGNATURE", sign(data, apiKEY))
	req.Header.Add("Content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	req.Close = true
	defer resp.Body.Close()

	rawData, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if err = json.Unmarshal(rawData, &body); err != nil {
		return nil, nil, err
	}

	return &body, &rawData, nil
}
