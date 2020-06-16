package main

const (
	AUTH_ERROR   = 401
	WALLET_ERROR = 500
)

var ERRORS = map[int]string{
	AUTH_ERROR:   "Authentication Error",
	WALLET_ERROR: "Wallet Error",
}

// jsMap - alias for map[string]interface
type jsMap map[string]interface{}

// User - holds user details
type User struct {
	Username     string
	Address      string
	TwoFAEnabled bool
}

// TPage stores template data
type TPage struct {
	TUser   User
	TWallet RWallet
	TData   jsMap
}
