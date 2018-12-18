package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"./turtlecoin-rpc-go/walletd"
	_ "github.com/lib/pq"

	"github.com/julienschmidt/httprouter"
)

var (
	dbUser, dbPwd     string
	hostURI, hostPort string
	rpcPort           int
	rpcPwd            string
	walletDB          *sql.DB
)

// Forking config.
var addressFormat = "^TRTL([a-zA-Z0-9]{95}|[a-zA-Z0-9]{183})$"
var divisor float64 = 100 // This is 100 for TRTL
var transactionFee = 10 // This is 10 for TRTL

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
}

func main() {
	router := httprouter.New()
	router.GET("/status/:address", getStatus)
	router.GET("/delete/:address", deleteAddress)
	router.GET("/create", newAddress)
	router.GET("/export_keys/:address", exportKeys)
	router.GET("/transactions/:address/:n", getTransactions)
	router.POST("/send_transaction", sendTransaction)
	log.Fatal(http.ListenAndServe(hostPort, router))
}

// createWallet - creates a new wallet
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

// newAddress - creates an address for a new user
func newAddress(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	encoder := json.NewEncoder(res)
	address, err := createWallet()
	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}
	walletDB.Exec("INSERT INTO addresses (address) VALUES ($1);", address)
	data := map[string]interface{}{"address": address}
	encoder.Encode(jsonResponse{Status: "OK", Data: data})
}

// deleteAddress - removes address from container
func deleteAddress(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	address := p.ByName("address")
	walletd.DeleteAddress(
		rpcPwd,
		"localhost",
		rpcPort,
		address,
	)
	walletDB.Exec(`DELETE FROM transactions
			WHERE addr_id = (SELECT id FROM addresses WHERE address = $1);`, address)
	walletDB.Exec("DELETE FROM addresses WHERE address = $1;", address)
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
	trtl := temp["result"].(map[string]interface{})["availableBalance"].(float64) / divisor
	temp["result"].(map[string]interface{})["availableBalance"] = trtl
	trtl = temp["result"].(map[string]interface{})["lockedAmount"].(float64)
	temp["result"].(map[string]interface{})["lockedAmount"] = trtl / divisor

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
	extra := "" // TODO - use for messages
	response := jsonResponse{}
	if matched, _ := regexp.MatchString(addressFormat, dest); !matched {
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
	amount *= divisor
	walletdResponse := walletd.SendTransaction(
		rpcPwd,
		"localhost",
		rpcPort,
		[]string{address},
		[]map[string]interface{}{
			{
				"amount":  int64(amount),
				"address": dest,
			},
		},
		transactionFee, // fee
		0, // unlock time
		3, // mixin
		extra,
		paymentID,
		"", // change address
	)
	json.NewDecoder(walletdResponse).Decode(&response.Data)
	if message, ok := response.Data["error"]; ok {
		response.Status = message.(map[string]interface{})["message"].(string)
	} else {
		response.Status = "OK"
	}
	json.NewEncoder(res).Encode(response)
}

// getTransactions - gets transaction history from the database
func getTransactions(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	encoder := json.NewEncoder(res)
	rows, err := walletDB.Query(`SELECT dest, hash, amount, pID, id FROM transactions
								 WHERE addr_id = (SELECT id FROM addresses WHERE address = $1) AND id > $2 ORDER BY id DESC LIMIT 15;`,
		p.ByName("address"), p.ByName("n"))

	if err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}

	var tmp string
	txs := make([]transaction, 0)
	for rows.Next() {
		tx := transaction{}
		err := rows.Scan(&tmp, &tx.Hash, &tx.Amount, &tx.PaymentID, &tx.ID)
		if err != nil {
			encoder.Encode(jsonResponse{Status: err.Error()})
			return
		}
		if tmp[0] != ' ' {
			tx.Destination = tmp
		}
		txs = append(txs, tx)
	}

	if err = rows.Err(); err != nil {
		encoder.Encode(jsonResponse{Status: err.Error()})
		return
	}
	encoder.Encode(jsonResponse{Status: "OK",
		Data: map[string]interface{}{"transactions": txs}})
}

// exportKeys - exports the spend and view key
func exportKeys(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	encoder := json.NewEncoder(res)
	address := p.ByName("address")
	response := jsonResponse{Status: "OK", Data: map[string]interface{}{}}
	key := map[string]interface{}{}
	walletdResponse := walletd.GetViewKey(
		rpcPwd,
		"localhost",
		rpcPort,
	)
	json.NewDecoder(walletdResponse).Decode(&key)
	response.Data["viewKey"] = key["result"].(map[string]interface{})["viewSecretKey"].(string)
	walletdResponse = walletd.GetSpendKeys(
		rpcPwd,
		"localhost",
		rpcPort,
		address,
	)
	json.NewDecoder(walletdResponse).Decode(&key)
	response.Data["spendPublicKey"] = key["result"].(map[string]interface{})["spendPublicKey"].(string)
	response.Data["spendSecretKey"] = key["result"].(map[string]interface{})["spendSecretKey"].(string)
	encoder.Encode(response)
}
