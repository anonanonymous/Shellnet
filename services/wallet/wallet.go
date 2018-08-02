package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"./turtlecoin-rpc-go/walletd"
	"github.com/gomodule/redigo/redis"

	"github.com/julienschmidt/httprouter"
)

var (
	hostURI, hostPort string
	rpcPort           int
	rpcPwd            string
	walletDB          *redis.Pool
)

func init() {
	var err error
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = ":6379"
	}
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
	walletDB = newPool(redisHost)
	cleanupHook()
}

func main() {
	router := httprouter.New()
	router.GET("/status/:address", getStatus)
	router.GET("/address", getAddress)
	router.POST("/send_transaction", sendTransaction)
	log.Fatal(http.ListenAndServe(hostPort, router))
}

// createWallets - creates a new wallet
func createWallet() (string, error) {
	response := map[string]interface{}{}
	walletdResponse := walletd.CreateAddress(
		rpcPwd,
		"localhost",
		rpcPort,
	)
	json.NewDecoder(walletdResponse).Decode(&response)
	address := response["result"].(map[string]interface{})["address"].(string)
	return address, nil
}

// getAddress - gets an address for a new user
func getAddress(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	encoder := json.NewEncoder(res)
	address, err := createWallet()
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}
	data := map[string]interface{}{"address": address}
	encoder.Encode(jsonResponse{Status: "OK", Data: data})
}

// getStatus - gets the balance and status of a wallet
func getStatus(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	address := p.ByName("address")
	response := jsonResponse{Data: map[string]interface{}{}}
	temp := make(map[string]interface{}, 1)
	walletdResponse := walletd.GetBalance(
		rpcPwd,
		"localhost",
		rpcPort,
		address,
	)
	json.NewDecoder(walletdResponse).Decode(&temp)
	trtl := temp["result"].(map[string]interface{})["availableBalance"].(float64) / 100
	temp["result"].(map[string]interface{})["availableBalance"] = trtl
	trtl = temp["result"].(map[string]interface{})["lockedAmount"].(float64) / 100
	temp["result"].(map[string]interface{})["lockedAmount"] = trtl

	response.Data["balance"] = temp["result"]
	walletdResponse = walletd.GetStatus(
		rpcPwd,
		"localhost",
		rpcPort,
	)
	json.NewDecoder(walletdResponse).Decode(&temp)
	response.Data["status"] = temp["result"]
	json.NewEncoder(res).Encode(response)
}

// sendTransaction - sends a transaction from address to dest
func sendTransaction(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	dest := req.FormValue("destination")
	amountStr := req.FormValue("amount")
	paymentID := req.FormValue("payment_id")
	address := req.FormValue("address")
	extra := ""
	response := jsonResponse{}
	if matched, _ := regexp.MatchString("^(TRTL)[a-zA-Z0-9]{95}$", dest); !matched {
		json.NewEncoder(res).Encode(jsonResponse{Status: "Incorrect Address Format"})
		return
	}
	if matched, _ := regexp.MatchString("^[0-9]+\\.{0,1}[0-9]{0,2}$", amountStr); !matched {
		json.NewEncoder(res).Encode(jsonResponse{Status: "Incorrect Amount Format"})
		return
	}
	if matched, _ := regexp.MatchString("^[a-fA-F0-9]{64}$", paymentID); !matched && paymentID != "" {
		json.NewEncoder(res).Encode(jsonResponse{Status: "Incorrect Payment ID Format"})
		return
	}
	amount, _ := strconv.ParseFloat(amountStr, 10)
	amount *= 100
	walletdResponse := walletd.SendTransaction(
		rpcPwd,
		"localhost",
		rpcPort,
		[]string{address},
		[]map[string]interface{}{
			{
				"amount":  amount,
				"address": dest,
			},
		},
		10,
		0, // unlock time
		7, // mixin
		extra,
		paymentID,
		"", // change address
	)
	fmt.Println(amount, address, rpcPort, dest, paymentID)
	json.NewDecoder(walletdResponse).Decode(&response.Data)
	if message, ok := response.Data["error"]; ok {
		response.Status = message.(map[string]interface{})["message"].(string)
	} else {
		response.Status = "OK"
	}
	json.NewEncoder(res).Encode(response)
}
