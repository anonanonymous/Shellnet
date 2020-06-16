package main

import (
	"encoding/json"
	"errors"
	"math"
)

var WALLET_HTTP = map[string]string{
	"status":       "GET",
	"send":         "POST",
	"delete":       "DELETE",
	"transactions": "GET",
	"keys":         "GET",
}

// RWallet - wallet service response
type RWallet struct {
	Status       *Status
	Transactions *[]Transaction
	Balance      *Balance
	Keys         *map[string]string
	TxHash       *string `json:"transactionHash"`
}

// Status - represents a status object
type Status struct {
	BlockCount       uint64 `json:"networkBlockCount"`
	WalletBlockCount uint64 `json:"walletBlockCount"`
	LocalBlockCount  uint64 `json:"localDaemonBlockCount"`
	PeerCount        uint64 `json:"peerCount"`
	Hashrate         uint64 `json:"hashrate"`
	IsViewWallet     bool   `json:"isViewWallet"`
	SubWalletCount   uint64 `json:"subWalletCount"`
}

// Transaction - represents a transaction object
type Transaction struct {
	Hash      string
	Amount    string
	PaymentID string
	ID        string
	Timestamp uint64
}

// Balance - represents a wallet balance
type Balance struct {
	Unlocked float64 `json:"unlocked"`
	Locked   float64 `json:"locked"`
	Address  string  `json:"address"`
}

// walletReq - helper for wallet api calls
func walletReq(route, address string, data jsMap, params ...string) (*RWallet, error) {
	var response struct {
		Status string
		Data   RWallet
	}

	method := WALLET_HTTP[route]
	route = walletPath(route, address, params...)
	b, err := sendRequest(
		method,
		walletURI+route,
		makeJSONString(data),
	)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(*b, &response); err != nil {
		return nil, err
	}
	println(string(*b))
	if response.Status != "OK" {
		return nil, errors.New(response.Status)
	}

	return &response.Data, nil
}

// walletPath - resolves the wallet path
func walletPath(route, address string, params ...string) (path string) {
	switch route {
	case "status", "keys", "send":
		path = "/" + address + "/" + route
	case "transactions":
		path = "/" + address + "/" + route + "/" + params[0]
	case "delete":
		path = "/" + address
	}
	return path
}

// walletStatusColor - green if synced, else orange
func walletStatusColor(s *Status) string {
	a := s.BlockCount
	b := s.WalletBlockCount
	if b >= a - 1 || (uint64(math.Abs(float64(a)-float64(b))) < 5 && b > 1) {
		return "green-input"
	}
	return "orange-input"
}
