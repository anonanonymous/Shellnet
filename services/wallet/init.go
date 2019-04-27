package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
)

var (
	dbUser, dbPwd     string
	hostURI, hostPort string
	rpcPort           int
	rpcPwd            string
	walletDB          *sql.DB
)

const (
	// Forking config.
	addressFormat          = "^TRTL([a-zA-Z0-9]{95}|[a-zA-Z0-9]{183})$"
	divisor        float64 = 100 // This is 100 for TRTL
	transactionFee         = 10  // This is 10 for TRTL
)

func init() {
	var err error

	if dbUser = os.Getenv("DB_USER"); dbUser == "" {
		panic("Set the DB_USER env variable")
	}
	if dbPwd = os.Getenv("DB_PWD"); dbPwd == "" {
		panic("Set the DB_PWD env variable")
	}

	walletDB, err = sql.Open("postgres", "postgres://"+dbUser+":"+dbPwd+"@localhost/tx_history?sslmode=disable")
	if err != nil {
		panic(err)
	}
	if err = walletDB.Ping(); err != nil {
		panic(err)
	}

	fmt.Println("You connected to your database.")
	if hostURI = os.Getenv("HOST_URI"); hostURI == "" {
		hostURI = "http://localhost"
		println("Using default HOST_URI - http://localhost")
	}
	if hostPort = os.Getenv("HOST_PORT"); hostPort == "" {
		hostPort = ":8082"
		println("Using default HOST_PORT - 8082")
	}

	hostURI += hostPort
	if rpcPwd = os.Getenv("RPC_PWD"); rpcPwd == "" {
		panic("Set the RPC_PWD env variable")
	}
	if rpcPort, err = strconv.Atoi(os.Getenv("RPC_PORT")); rpcPort == 0 || err != nil {
		rpcPort = 8070
		println("Using default RPC_PORT - 8070")
	}

	wallet := NewService()
	wallet.RPCPassword = rpcPwd
	go wallet.Start()
}
