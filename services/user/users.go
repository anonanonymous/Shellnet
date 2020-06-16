package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/pquerna/otp/totp"

	_ "github.com/lib/pq"

	"github.com/julienschmidt/httprouter"
)

func main() {
	defer logFile.Close()
	log.SetOutput(logFile)

	router := httprouter.New()
	router.POST("/signup", signup)
	router.POST("/login", login)
	router.PUT("/update/:username/:setting", updateUser)
	router.DELETE("/delete/:username", deleteUser)

	log.Println("Info: Starting Service on:", hostURI)
	log.Fatal(http.ListenAndServe(hostPort, router))
}

// signup - adds user to db
func signup(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var user map[string]string

	data, err := getBody(req)
	if err != nil {
		writeJSON(res, 500, jsMap{"status": "Server Error"})
		return
	}

	if err := json.Unmarshal(data, &user); err != nil {
		log.Println("signup:", err)
		writeJSON(res, 500, jsMap{"status": "Server Error"})
		return
	}

	username := user["username"]
	password := user["password"]
	if isRegistered(username) {
		writeJSON(res, 400, jsMap{"status": "Username taken"})
		return
	}

	v, err := srpEnv.Verifier([]byte(username), []byte(password))
	if err != nil {
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	ih, verif := v.Encode()
	resp, _, err := sendRequest("POST", walletURI, "")
	if err != nil {
		writeJSON(res, 500, jsMap{"status": err.Error()})
		return
	}

	address := (*resp)["data"].(string)
	_, err = db.Exec(`
		INSERT INTO accounts (ih, verifier, username, address)
		VALUES ($1, $2, $3, $4);`, ih, verif, username, address,
	)

	if err != nil {
		writeJSON(res, 500, jsMap{"status": err.Error()})
	} else {
		writeJSON(res, 201, jsMap{"status": "OK"})
	}
}

// login - verify username/password and sends back a sessionID
func login(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var data map[string]string

	resp, err := getBody(req)
	if err != nil {
		writeJSON(res, 500, jsMap{"status": "Server Error"})
		return
	}

	err = json.Unmarshal(resp, &data)
	if err != nil {
		log.Panicln("login:", err)
	}

	username := data["username"]
	password := data["password"]
	user, err := getUser(username)
	if err != nil {
		writeJSON(res, 401, jsMap{"status": "Authentication Failed"})
		return
	}

	sessID, err := authenticateUser(user, username, password)
	if err != nil {
		log.Println("login:", err)
		writeJSON(res, 401, jsMap{"status": "Authentication Failed"})
		return
	}

	response := jsMap{
		"status":    "OK",
		"sessionID": hex.EncodeToString(sessID),
		"address":   user.Address,
	}

	writeJSON(res, 200, response)
}

// deleteUser - removes user from db
func deleteUser(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	_, err := db.Exec(`
		DELETE FROM accounts
		WHERE username = $1;`, p.ByName("username"),
	)
	if err != nil {
		log.Println("deleteUser:", err)
	}

	writeJSON(res, 200, jsMap{"status": "OK"})
}

// updateUser - modifies user attributes
func updateUser(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
	var data map[string]string
	var username = p.ByName("username")

	resp, err := getBody(req)
	if err != nil {
		writeJSON(res, 500, jsMap{"status": "Server Error"})
		return
	}
	if err := json.Unmarshal(resp, &data); err != nil {
		log.Println("updateUser:", err)
		writeJSON(res, 400, jsMap{"status": "Invalid Data"})
		return
	}

	user, err := getUser(username)
	if err != nil {
		writeJSON(res, 404, jsMap{"status": "Not Found"})
		return
	}
	// if the user has 2fa enabled, verify their totp key
	if user.TOTP != "" {
		err = verifyTOTP(user, data["passcode"])
		if err != nil {
			writeJSON(res, 401, jsMap{"status": err.Error()})
			return
		}
	}

	switch p.ByName("setting") {
	case "password":
		_, err = authenticateUser(user, username, data["password"])
		if err != nil {
			writeJSON(res, 401, jsMap{"status": "Wrong Password"})
			return
		}

		v, err := srpEnv.Verifier([]byte(username), []byte(data["new_password"]))
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}

		ih, verif := v.Encode()
		_, err = db.Exec(`
			UPDATE accounts 
			SET ih = $1, verifier = $2
			WHERE username = $3;`, ih, verif, username,
		)
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}

	case "2fa_enable":
		if !totp.Validate(data["passcode"], data["secret"]) {
			writeJSON(res, 401, jsMap{"status": "Wrong Passcode"})
			return
		}

		key, _ := hex.DecodeString(secretKey)
		plaintext := []byte(data["secret"])
		block, err := aes.NewCipher(key)
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}

		ciphertext := make([]byte, aes.BlockSize+len(plaintext))
		iv := ciphertext[:aes.BlockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}

		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

		_, err = db.Exec(`
			UPDATE accounts
			SET totp = $1
			WHERE username = $2;`,
			hex.EncodeToString(ciphertext),
			username,
		)
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}

	case "2fa_disable":
		_, err = db.Exec(`
			UPDATE accounts
			SET totp = ''
			WHERE username = $1;`, username,
		)
		if err != nil {
			writeJSON(res, 500, jsMap{"status": err.Error()})
			return
		}
	}
	writeJSON(res, 200, jsMap{"status": "OK"})
}
