package main

import (
	"database/sql"
	"log"
	"os"

	walletapi "github.com/anonanonymous/wallet-api-go"
)

var (
	viewKey   string
	walletDB  *sql.DB
	walletAPI *walletapi.WalletAPI
	logFile   *os.File
)

func init() {
	var err error

	// logging setup
	logFile, err = os.OpenFile(
		"service.log",
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		panic(err)
	}

	// Database setup
	walletDB, err = sql.Open(
		"postgres",
		"postgres://"+dbUser+":"+dbPwd+"@localhost/tx_history?sslmode=disable",
	)
	if err != nil {
		panic(err)
	}
	if err = walletDB.Ping(); err != nil {
		panic(err)
	}
	log.Println("You connected to your database.")

	// http://localhost  :8082 -> http://localhost:8082
	hostURI += hostPort

	// Wallet configuration
	walletAPI = walletapi.InitWalletAPI(walletAPIKey, walletAPIHost, walletAPIPort)
	viewKey, err = walletAPI.ViewKey()
	if err != nil {
		panic(err)
	}
}
