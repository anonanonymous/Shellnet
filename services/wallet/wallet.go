package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"./turtlecoin-rpc-go/walletd"
	"github.com/gomodule/redigo/redis"

	"github.com/julienschmidt/httprouter"
)

var cwd string
var wallet string
var sessionDB redis.Conn

type jsonResponse struct {
	Status string
	Data   string
}

var ports = map[int]string{}

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	wallet = cwd + "/walletd"
	sessionDB, err = redis.Dial("tcp", ":6379")
	if err != nil {
		panic(err)
	}
}

func main() {
	router := httprouter.New()
	router.POST("/start", startWallet)
	//router.GET("/stop", stopWallet)
	router.GET("/balance/:session_id", getBalance)
	router.GET("/address/:session_id", getAddress)
	router.GET("/status/:session_id", getStatus)
	router.POST("/create", createWallet)
	router.POST("/send_transaction/:session_id", sendTransaction)
	log.Fatal(http.ListenAndServe(":8082", router))
}

func createWallet(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	encoder := json.NewEncoder(res)
	fname := req.FormValue("filename")
	pwd := req.FormValue("password")
	bytes, err := exec.Command(wallet, "-g", "-w", "wallets/"+fname, "-p", pwd).Output()
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
	} else {
		re := regexp.MustCompile("(TRTL)[a-zA-Z0-9]{95}")
		address := re.FindString(string(bytes))
		encoder.Encode(jsonResponse{Status: "OK", Data: address})
	}
}

/* start running the users wallet node*/
func startWallet(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	encoder := json.NewEncoder(res)
	fname := req.FormValue("filename")
	pwd := req.FormValue("password")
	rpc := req.FormValue("rpc_password")

	port := ""
	for i := 8090; i < 65000; i++ {
		if _, err := sessionGetKey(strconv.Itoa(i)); err != nil {
			sessionSetKey(strconv.Itoa(i), "active")
			port = strconv.Itoa(i)
			break
		}
	}
	conf := []byte(
		"container-file=wallets/" + fname +
			"\ncontainer-password=" + pwd +
			"\nrpc-password=" + rpc +
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
	cmd := exec.Command(wallet, "-c", "/tmp/"+fname, "-d")
	err = cmd.Run()
	os.Remove("/tmp/" + fname)
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
	} else {
		encoder.Encode(jsonResponse{Status: "OK"})
	}
	sessionSetKey(fname+rpc, port)
}

func getBalance(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	sessID := p.ByName("session_id")
	fname, _ := sessionGetKey(sessID)
	temp, _ := sessionGetKey(fname + sessID)
	port, _ := strconv.Atoi(temp)
	walletdResponse := walletd.GetBalance(
		sessID,
		"localhost",
		port,
		"",
	)
	walletdResponse.WriteTo(res)
}

func getStatus(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	sessID := p.ByName("session_id")
	fname, _ := sessionGetKey(sessID)
	temp, _ := sessionGetKey(fname + sessID)
	port, _ := strconv.Atoi(temp)
	walletdResponse := walletd.GetStatus(
		sessID,
		"localhost",
		port,
	)
	walletdResponse.WriteTo(res)
}

func getAddress(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	sessID := p.ByName("session_id")
	fname, _ := sessionGetKey(sessID)
	temp, _ := sessionGetKey(fname + sessID)
	port, _ := strconv.Atoi(temp)
	walletdResponse := walletd.GetAddresses(
		sessID,
		"localhost",
		port,
	)
	walletdResponse.WriteTo(res)
}

func sendTransaction(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	dest := req.FormValue("destination")
	amountStr := req.FormValue("amount")
	paymentID := req.FormValue("payment_id")
	address := req.FormValue("address")
	if matched, _ := regexp.MatchString("^(TRTL)[a-zA-Z0-9]{95}$", dest); !matched {
		panic("Incorrect Address Format")
		return
	}
	if matched, _ := regexp.MatchString("^[0-9]+\\.{0,1}[0-9]{0,2}$", amountStr); !matched {
		panic("Incorrect Amount Format")
		return
	}
	if matched, _ := regexp.MatchString("^[a-fA-F0-9]{64}$", paymentID); !matched {
		panic("Incorrect Payment Id Format")
		return
	}
	amount, _ := strconv.Atoi(amountStr)
	sessID := p.ByName("session_id")
	fname, _ := sessionGetKey(sessID)
	temp, _ := sessionGetKey(fname + sessID)
	port, _ := strconv.Atoi(temp)
	walletdResponse := walletd.SendTransaction(
		sessID,
		"localhost",
		port,
		[]string{address},
		[]map[string]interface{}{
			{
				"amount":  amount,
				"address": dest,
			},
		},
		10,
		0,  // unlock time
		7,  // mixin
		"", // extra
		paymentID,
		"", // change address
	)
	walletdResponse.WriteTo(res)
}

/*
func getWalletPid(name string) []string {
	cmd := "ps ax | grep /tmp/"+name+" cut -f1 -d' '"
	output, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return
	}
	pids := strings.Split(string(output), "\n")
	return pids
}
*/
