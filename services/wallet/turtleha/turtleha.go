package turtleha

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq" // benis

	"../turtlecoin-rpc-go/walletd"
)

var (
	cwd           string
	dbUser, dbPwd string
	walletDB      *sql.DB
)

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		panic(err)
	}

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
}

// TurtleService - daemon config
type TurtleService struct {
	MaxPollingFailures int
	PollingFailures    int
	BindAddress        string
	RPCPassword        string
	RPCPort            int
	PollingInterval    int // check if the daemon is alive every n seconds
	ScanHeight         int64
	LastBlock          int64  // last block scanned from getTransactions
	ScanInterval       int // check for transactions every n seconds
	SaveInterval       int // save every n seconds
	Timeout            int // polling timeout
	synced             bool
	mux                sync.Mutex // only allow one goroutine to access a variable
}

// NewService - creates a turtleservice with the default options
func NewService() *TurtleService {
	service := &TurtleService{
		MaxPollingFailures: 30,
		PollingFailures:    0,
		BindAddress:        "localhost",
		SaveInterval:       60000,
		ScanInterval:       5000,
		LastBlock:          1,
		Timeout:            5000,
		PollingInterval:    10000,
		RPCPort:            8070,
	}
	return service
}

// Start - starts the turtle-service
func (service *TurtleService) Start() error {
	service.loadConfig()
	go service.pinger()
	go service.saver()
	service.scanner()
	return nil
}

// loadConfig - loads data from data file
func (service *TurtleService) loadConfig() {
	conf, err := os.Open("./data/ha.data")
	if err != nil {
		panic(err)
	}
	bytes, err := ioutil.ReadAll(conf)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bytes, service)
	if err != nil {
		panic(err)
	}
}

// saves the wallet every save interval if the wallet is synced
func (service *TurtleService) saver() {
	for ; ; time.Sleep(time.Millisecond * time.Duration(service.SaveInterval)) {
		if service.isSynced() {
			fmt.Println("wallet is synced")
			service.PollingFailures = 0
		} else {
			fmt.Println("not saving: blockchain not synced")
		}
	}
}

func (service *TurtleService) scanner() {
	fmt.Println("scanner started")
	for ; ; time.Sleep(time.Duration(service.ScanInterval) * time.Millisecond) {
		result := map[string]interface{}{}
		if service.ScanHeight < service.LastBlock {
			fmt.Println(service.ScanHeight, " ", service.LastBlock)
			response := walletd.GetTransactions(
				service.RPCPassword,
				service.BindAddress,
				service.RPCPort,
				int(service.ScanHeight),
				int(service.LastBlock-service.ScanHeight),
			)
			fmt.Println(response)
			if err := json.NewDecoder(response).Decode(&result); err != nil {
				panic(err)
			}

			for _, block := range result["result"].(map[string]interface{})["items"].([]interface{}) {
				for _, tx := range block.(map[string]interface{})["transactions"].([]interface{}) {
					pID := tx.(map[string]interface{})["paymentId"].(string)
					txID := tx.(map[string]interface{})["transactionHash"].(string)
					transfer := tx.(map[string]interface{})["transfers"].([]interface{})
					fmt.Println("Transaction:\n pId:", pID, "\nhash:", txID)
					if amount := tx.(map[string]interface{})["amount"].(float64); amount > 0 {
						for i := 0; i <= len(transfer)-2; i++ {
							t := transfer[i].(map[string]interface{})
							dest := t["address"].(string)
							amount = t["amount"].(float64)
							addTransaction(dest, "", txID, pID, amount)
						}
					} else {
						fmt.Println(transfer)
						chgAddr := transfer[len(transfer)-1].(map[string]interface{})
						src := chgAddr["address"].(string)
						for i := 1; i < len(transfer)-2; i++ {
							t := transfer[i].(map[string]interface{})
							dest := t["address"].(string)
							amount = t["amount"].(float64)
							addTransaction(src, dest, txID, pID, amount)
						}
					}
				}
			}
			service.ScanHeight = service.LastBlock
		}
	}
	fmt.Println("out scanner")
}

// checks if the wallet responds to rpc calls in the timeout period
func (service *TurtleService) pinger() {
	for ; ; time.Sleep(time.Duration(service.PollingInterval) * time.Millisecond) {
		stat := make(chan int, 1)
		go func() {
			fmt.Println("wallet ping")
			walletd.GetStatus(
				service.RPCPassword,
				service.BindAddress,
				service.RPCPort,
			)
			stat <- 1
		}()
		fmt.Println(service.PollingFailures, service.MaxPollingFailures)
		select {
		case <-stat:
			service.updateData()
			service.PollingFailures = 0
			time.Sleep(time.Millisecond * time.Duration(service.PollingInterval))
		case <-time.After(time.Millisecond * time.Duration(service.Timeout)):
			service.PollingFailures++
			if service.PollingFailures > service.MaxPollingFailures {
				println("failed")
			}
		}
	}
	fmt.Println("out pinger")
}

// check if the wallet is synced, saves the current block in a file
func (service *TurtleService) isSynced() bool {
	var jsonResponse map[string]interface{}
	response := walletd.GetStatus(
		service.RPCPassword,
		service.BindAddress,
		service.RPCPort,
	)
	json.NewDecoder(response).Decode(&jsonResponse)
	blockCount := jsonResponse["result"].(map[string]interface{})["blockCount"].(float64)
	knownBlockCount := jsonResponse["result"].(map[string]interface{})["knownBlockCount"].(float64)
	service.LastBlock = int64(blockCount)
	return int64(blockCount)+1 >= int64(knownBlockCount)
}


// Save - saves the wallet
func (service *TurtleService) Save() {
	walletd.Save(
		service.RPCPassword,
		service.BindAddress,
		service.RPCPort,
	)
}

// updates the values in ./data/ha.data
func (service *TurtleService) updateData() {
	f, err := os.Create("./data/ha.data")
	if err != nil {
		fmt.Println(err)
		return
	}
	err = json.NewEncoder(f).Encode(
		map[string]interface{}{
			"scanHeight": service.ScanHeight,
			"lastBlock":  service.LastBlock,
		},
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	f.Close()
}

// adds a transactoin into the database
func addTransaction(src, dest, hash, paymentID string, amount float64) {
	_, err := walletDB.Exec(`INSERT INTO transactions (addr_id, dest, hash, pID, amount)
			VALUES ((SELECT id FROM addresses WHERE address = $1),
				$2, $3, $4, $5);`, src, dest, hash, paymentID, amount/100)
	if err != nil {
		fmt.Println(err)
	}
}
