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

const port = 8070

var rpcPwd string
var walletDB *redis.Pool

func init() {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = ":6379"
	}
	rpcPwd = os.Getenv("RPC_PWD")
	if rpcPwd == "" {
		panic("Set the RPC_PWD env variable")
	}
	walletDB = newPool(redisHost)
	cleanupHook()
}

func main() {
	router := httprouter.New()
	router.GET("/status/:address", getStatus)
	router.GET("/address", getAddress)
	router.POST("/send_transaction", sendTransaction)
	log.Fatal(http.ListenAndServe(":8082", router))
}

// createWallets - creates a new wallet
func createWallet() (string, error) {
	response := map[string]interface{}{}
	walletdResponse := walletd.CreateAddress(
		rpcPwd,
		"localhost",
		port,
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
		port,
		address,
	)
	json.NewDecoder(walletdResponse).Decode(&temp)
	response.Data["balance"] = temp["result"]
	walletdResponse = walletd.GetStatus(
		rpcPwd,
		"localhost",
		port,
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
	walletdResponse := walletd.SendTransaction(
		rpcPwd,
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
	fmt.Println(amount, address, port, dest, paymentID)
	fmt.Printf("%v\n", walletdResponse)
	response.Status = "OK"
	json.NewDecoder(walletdResponse).Decode(&response.Data)
	json.NewEncoder(res).Encode(response)
}
