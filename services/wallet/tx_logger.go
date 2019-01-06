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
	wallet.Path = cwd + "/turtle-service"
	wallet.RPCPassword = "<rpc-password>"
	wallet.LogLevel = "3"
	wallet.DaemonAddress = "<daemon url>"
	wallet.MaxPollingFailures = 10
	wallet.ContainerPassword = "<container password>"
	wallet.ContainerFile = cwd + "/<wallet file>"

	err := wallet.Start()
	if err != nil {
		panic(err)
	}
}
