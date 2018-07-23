package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/julienschmidt/httprouter"
)

var cwd string
var walletd string

type jsonResponse struct {
	Status string
}

var ports = map[int]string{}

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	walletd = cwd + "/walletd"
}

func main() {
	router := httprouter.New()
	router.POST("/start", startWallet)
	router.POST("/create", createWallet)
	log.Fatal(http.ListenAndServe(":8082", router))
}

func createWallet(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	encoder := json.NewEncoder(res)
	fname := req.FormValue("filename")
	pwd := req.FormValue("password")
	err := exec.Command(walletd, "-g", "-w", "wallets/"+fname, "-p", pwd).Run()
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
	} else {
		encoder.Encode(jsonResponse{Status: "OK"})
	}
}

/* start running the users wallet node*/
func startWallet(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	encoder := json.NewEncoder(res)
	fname := req.FormValue("filename")
	pwd := req.FormValue("password")
	var port string
	// sponge
	for i := 8070; i < 65000; i++ {
		if _, ok := ports[i]; !ok {
			ports[i] = strconv.Itoa(i)
			port = ports[i]
			break
		}
	}
	conf := []byte(
		"container-file=wallets/" + fname +
			"\ncontainer-password=" + pwd +
			"\nrpc-password=" + pwd +
			"\nbind-port=" + port +
			"\ndaemon-address=us-west.turtlenode.io",
	)
	fmt.Println(fname, port, pwd, "\n", string(conf))
	err := ioutil.WriteFile("/tmp/"+fname, conf, 0644)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		os.Remove("/tmp/" + fname)
		return
	}
	cmd := exec.Command(walletd, "-c", "/tmp/"+fname, "-d")
	err = cmd.Run()
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
	} else {
		encoder.Encode((jsonResponse{Status: "OK"}))
	}
	os.Remove("/tmp/" + fname)
}
