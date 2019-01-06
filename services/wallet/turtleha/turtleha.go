package turtleha

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"

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
	Path               string // full path the turtle-service
	BindAddress        string // bind-address
	ContainerFile      string // full path to container file
	ContainerPassword  string
	DaemonAddress      string
	DaemonPort         string
	LogFile            string // full path to log file
	LogLevel           string // 0 - 4 verbosity
	LastBlock          int64  // last block scanned from getTransactions
	MaxPollingFailures int
	PollingFailures    int
	PollingInterval    int // check if the daemon is alive every n seconds
	ScanHeight         int64
	ScanInterval       int // check for transactions every n seconds
	SaveInterval       int // save every n seconds
	Status             int // 1 is alive, 0 is dead
	Timeout            int // polling timeout
	RPCPort            int
	RPCPassword        string
	synced             bool
	mux                sync.Mutex // only allow one goroutine to access a variable
}

// NewService - creates a turtleservice with the default options
func NewService() *TurtleService {
	service := &TurtleService{
		Path:               cwd + "/turtle-service",
		BindAddress:        "localhost",
		DaemonAddress:      "turtlenode.online",
		DaemonPort:         "11898",
		LogFile:            cwd + "/data/turtle.log",
		LogLevel:           "4",
		LastBlock:          1,
		MaxPollingFailures: 30,
		PollingFailures:    0,
		SaveInterval:       60000,
		ScanInterval:       5000,
		Status:             1,
		Timeout:            5000,
		PollingInterval:    10000,
		RPCPort:            8070,
	}
	return service
}

// Start - starts the turtle-service
func (service *TurtleService) Start() error {
	cmd := exec.Command(
		"unbuffer",
		service.Path,
		"--rpc-password", service.RPCPassword,
		"--container-file", service.ContainerFile,
		"--container-password", service.ContainerPassword,
		"--log-file", service.LogFile,
		"--log-level", service.LogLevel,
		"--daemon-address", service.DaemonAddress,
	)
	stat := make(chan int, 1)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(pipe)
	go func() {
		for scanner.Scan() {
			output := scanner.Text()
			if strings.Contains(output, "Outdated pool transactions processed") {
				fmt.Println("wallet started")
				stat <- 1
			} else if strings.Contains(output, "ERROR") {
				fmt.Println(output)
				if strings.Contains(output, "Synchronization error") {
					fmt.Println("fatal error")
					cmd.Process.Kill()
				}
			} else if strings.Contains(output, "New wallet added") {
				println("new wallet")
				go service.Save()
			}
		}
	}()
	if 1 == <-stat {
		fmt.Println("starting monitoring routines")
		service.loadConfig()
		go service.pinger(cmd)
		go service.saver(cmd)
		go service.scanner()
	}
	if err := cmd.Wait(); err != nil {
		panic(err)
	}

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
func (service *TurtleService) saver(cmd *exec.Cmd) {
	for service.Status == 1 {
		if service.isSynced() {
			service.Save()
			service.backup()
			fmt.Println("wallet saved and backed up")
			service.PollingFailures = 0
		} else {
			fmt.Println("not saving: wallet not synced")
		}
		time.Sleep(time.Millisecond * time.Duration(service.SaveInterval))
	}
	fmt.Println("out saver")
}

// backs up the container file
func (service *TurtleService) backup() {
	src, err := os.Open(service.ContainerFile)
	if err != nil {
		panic(err)
	}
	defer src.Close()

	bck, err := os.Create(service.ContainerFile + ".backup")
	if err != nil {
		panic(err)
	}
	defer bck.Close()
	_, err = io.Copy(bck, src)
	if err == nil {
		fmt.Println("backup created")
	}
}

// scans the blockschain for transactions every at scan interval
func (service *TurtleService) scanner() {
	fmt.Println("scanner started")
	for service.Status == 1 {
		result := map[string]interface{}{}
		if service.isSynced() && service.ScanHeight < service.LastBlock {
			fmt.Println(service.ScanHeight, " ", service.LastBlock)
			response := walletd.GetTransactions(
				service.RPCPassword,
				service.BindAddress,
				service.RPCPort,
				int(service.ScanHeight),
				int(service.LastBlock-service.ScanHeight),
			)
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
		time.Sleep(time.Duration(service.ScanInterval) * time.Millisecond)

	}
	fmt.Println("out scanner")
}

// checks if the wallet responds to rpc calls in the timeout period
func (service *TurtleService) pinger(cmd *exec.Cmd) {
	for service.Status == 1 {
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
				service.killer(cmd)
			}
		}
		time.Sleep(time.Duration(service.PollingInterval) * time.Millisecond)
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

// kills the daemon
func (service *TurtleService) killer(cmd *exec.Cmd) {
	fmt.Println("killed")
	cmd.Process.Kill()
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
