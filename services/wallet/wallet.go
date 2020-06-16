package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	_ "github.com/lib/pq"

	"github.com/julienschmidt/httprouter"
)

func main() {
	defer logFile.Close()
	log.SetOutput(logFile)

	router := httprouter.New()

	router.GET("/:address/status", getStatus)
	router.GET("/:address/keys", getKeys)
	router.GET("/:address/transactions", getTransactions)
	router.GET("/:address/transactions/:offset", getTransactions)
	//router.GET("/api/status", serviceStatus)
	router.POST("/", createAddress)
	router.POST("/:address/send", sendTransaction)

	router.DELETE("/:address", deleteAddress)

	log.Println("Info: Starting Service on:", hostURI)
	log.Fatal(http.ListenAndServe(hostPort, router))
}

// createAddress - creates an address for a new user
func createAddress(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	address, err := walletAPI.CreateAddress()
	if err != nil {
		writeJSON(res, 400, jsMap{"status": err.Error()})
		return
	}

	go walletAPI.Save()
	_, err = walletDB.Exec(`
		INSERT INTO addresses (address)
		VALUES ($1);`, address["address"],
	)
	if err != nil {
		log.Println("createAddress:", err)
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	log.Println("createAddress: new subwallet", address["address"])
	writeJSON(res, 201, jsMap{
		"status": "OK",
		"data":   address["address"],
	})
}

// deleteAddress - removes address from container
func deleteAddress(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var address = p.ByName("address")
	var addrID int64

	// get the id
	row := walletDB.QueryRow(`
		SELECT id
		FROM addresses
		WHERE address = $1;`, address,
	)
	if err := row.Scan(&addrID); err != nil {
		writeJSON(res, 400, jsMap{"status": err.Error()})
		return
	}

	walletAPI.DeleteAddress(address)
	_, err := walletDB.Exec(`
		DELETE FROM transactions
		WHERE addr_id = $1;`, addrID,
	)
	if err != nil {
		log.Println("deleteAddress:", err)
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	go walletAPI.Save()
	_, err = walletDB.Exec(`
		DELETE FROM addresses
		WHERE id = $1;`, addrID,
	)
	if err != nil {
		log.Println("deleteAddress:", err)
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	log.Println("deleteAddress: deleted address", address)
	writeJSON(res, 200, jsMap{"status": "OK"})
}

// getStatus - gets the balance and status of a wallet
func getStatus(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var address = p.ByName("address")

	println(address)
	balance, err := walletAPI.GetAddressBalance(address)
	if err != nil {
		writeJSON(res, 400, jsMap{"status": err.Error()})
		return
	}

	resp, err := walletAPI.Status()
	if err != nil {
		writeJSON(res, 400, jsMap{"status": err.Error()})
		return
	}

	writeJSON(res, 200, jsMap{
		"status": "OK",
		"data": jsMap{
			"status": *resp,
			"balance": jsMap{
				"address":  balance.Address,
				"locked":   float64(balance.Locked) / divisor,
				"unlocked": float64(balance.Unlocked) / divisor,
			},
		},
	})
}

// sendTransaction - sends a transaction from address to dest
func sendTransaction(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var response jsMap
	var tx struct {
		Amount               string
		Address, Destination string
		PaymentID            string `json:"payment_id"`
	}

	err := json.NewDecoder(req.Body).Decode(&tx)
	if err != nil {
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	_, err = walletAPI.ValidateAddress(tx.Destination)
	if err != nil {
		writeJSON(res, 400, jsMap{"status": "Incorrect Address Format"})
		return
	}
	if matched, _ := regexp.MatchString("^[0-9]+\\.{0,1}[0-9]{0,2}$", tx.Amount); !matched {
		writeJSON(res, 400, jsMap{"status": "Incorrect Amount Format"})
		return
	}
	if matched, _ := regexp.MatchString("^[a-fA-F0-9]{64}$", tx.PaymentID); !matched && tx.PaymentID != "" {
		writeJSON(res, 400, jsMap{"status": "Incorrect Payment ID Format"})
		return
	}

	iAmount, _ := strconv.ParseFloat(tx.Amount, 64)
	iAmount *= divisor

	txHash, err := walletAPI.SendTransactionAdvanced(
		[]map[string]interface{}{
			{
				"address": tx.Destination,
				"amount":  iAmount,
			},
		},
		nil, nil, []string{tx.Address}, tx.PaymentID, tx.Address, nil,
	)
	if err != nil {
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	response = jsMap{
		"status": "OK",
		"data": jsMap{
			"transactionHash": txHash,
		},
	}

	writeJSON(res, 200, response)
}

// getTransactions - gets transaction history from the database
func getTransactions(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var lastHeight, addrID uint64
	var ts time.Time
	var address = p.ByName("address")

	// get the last block scanned
	row := walletDB.QueryRow(`
			SELECT blockHeight, id
			FROM addresses
			WHERE address = $1;`, address,
	)
	if err := row.Scan(&lastHeight, &addrID); err != nil {
		writeJSON(res, 400, jsMap{"status": err.Error()})
		return
	}

	// get all transactions since last scan
	wallet, _ := walletAPI.Status()
	// uggo
	if lastHeight < wallet.BlockCount {
		transactions, err := walletAPI.GetAddressTransactionsInRange(
			address,
			lastHeight,
			wallet.BlockCount,
		)
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}

		// put the latest transactions into db
		for _, tx := range *transactions {
			for _, transfer := range tx.Transfers {
				if transfer.Address != address {
					continue
				}
				_, err = walletDB.Exec(`
					INSERT INTO transactions (addr_id, amount, hash, paymentID, _timestamp)
					VALUES ($1, $2, $3, $4, $5);`,
					addrID,
					float64(transfer.Amount)/divisor,
					tx.Hash,
					tx.PaymentID,
					time.Unix(int64(tx.Timestamp), 0),
				)
				if err != nil {
					log.Println(err)
				}
			}
		}
		// update block scan height
		_, err = walletDB.Exec(`
		UPDATE addresses
		SET blockHeight = $1
		WHERE id = $2;`, wallet.BlockCount, addrID,
		)
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}
	}

	// query database for transactions
	txs := []Transaction{}
	rows, err := walletDB.Query(`
		SELECT hash, amount, paymentID, _timestamp, id
		FROM transactions
		WHERE addr_id = $1
		ORDER BY id DESC LIMIT 15;`,
		addrID,
	)
	if err != nil {
		log.Println(err)
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	defer rows.Close()
	for rows.Next() {
		tx := Transaction{}
		err := rows.Scan(
			&tx.Hash,
			&tx.Amount,
			&tx.PaymentID,
			&ts,
			&tx.ID,
		)
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}

		tx.Timestamp = uint64(ts.Unix())
		txs = append(txs, tx)
	}

	if err = rows.Err(); err != nil {
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	writeJSON(res, 200, jsMap{
		"status": "OK",
		"data":   jsMap{"transactions": txs}},
	)
}

// getKeys - retrieves the spend and view key
func getKeys(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var err error
	address := p.ByName("address")

	keys, err := walletAPI.GetKeys(address)
	if err != nil {
		writeJSON(res, 500, jsMap{
			"status": "Error retrieving wallet keys",
		})
		return
	}

	response := jsMap{
		"status": "OK",
		"data": jsMap{
			"keys": jsMap{
				"publicSpendKey":  keys["publicSpendKey"],
				"privateSpendKey": keys["privateSpendKey"],
				"viewKey":         viewKey,
			},
		},
	}

	writeJSON(res, 200, response)
}
