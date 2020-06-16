package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/opencoff/go-srp"
)

var (
	srpEnv  *srp.SRP
	db      *sql.DB
	logFile *os.File
)

const nBits = 1024

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

	// database setup
	db, err = sql.Open(
		"postgres",
		"postgres://"+dbUser+":"+dbPwd+"@localhost/users?sslmode=disable",
	)
	if err != nil {
		panic(err)
	}
	if err = db.Ping(); err != nil {
		panic(err)
	}

	// services uri setup
	hostURI += hostPort

	// srp server for authentication
	srpEnv, err = srp.New(nBits)
	if err != nil {
		panic(err)
	}

	log.Println("You connected to your database.")
}
