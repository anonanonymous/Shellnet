package main

import (
	"encoding/json"
	"net/http"
)

type Transaction struct {
	Hash      string
	Amount    string
	PaymentID string
	ID        string
	Timestamp uint64
}

// jsMap - alias for map[string]interface{}
type jsMap map[string]interface{}

// writeJSON - writes json to the responseWriter
func writeJSON(res http.ResponseWriter, code int, data jsMap) {
	res.WriteHeader(code)
	json.NewEncoder(res).Encode(data)
}
