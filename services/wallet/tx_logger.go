package main

import (
	"os"

	"./turtleha"
)

var cwd string

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
}

func main() {
	wallet := turtleha.NewService()
	wallet.RPCPassword = "<rpc-password>"
	wallet.MaxPollingFailures = 10

	err := wallet.Start()
	if err != nil {
		panic(err)
	}
}
