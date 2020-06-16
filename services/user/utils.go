package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/opencoff/go-srp"
	"github.com/pquerna/otp/totp"
)

// jsMap - alias for map[string]interface{}
type jsMap map[string]interface{}

type User struct {
	ID       int
	IH       string
	Username string
	Verifier string
	Address  string
	TOTP     string
}

// sign - returns a HMAC signature for a message
func sign(message, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// isRegistered - check if username is already present in the database
func isRegistered(username string) bool {
	row := db.QueryRow(`
		SELECT username
		FROM accounts
		WHERE username = $1;`, username,
	)
	err := row.Scan()

	return err != sql.ErrNoRows
}

// getUser - retrieves a user from the database
func getUser(username string) (*User, error) {
	row := db.QueryRow(`
		SELECT ih, verifier, username, id, address, totp
		FROM accounts
		WHERE username = $1;`, username,
	)

	user := User{}
	err := row.Scan(
		&user.IH,
		&user.Verifier,
		&user.Username,
		&user.ID,
		&user.Address,
		&user.TOTP,
	)
	if err != nil {
		log.Println("getUser: ", err)
		return nil, err
	}

	return &user, nil
}

// getBody - retrieves the raw data received in a request
func getBody(req *http.Request) ([]byte, error) {
	rawData, err := ioutil.ReadAll(req.Body)
	return rawData, err
}

// writeJSON - writes json to the responseWriter
func writeJSON(res http.ResponseWriter, code int, data jsMap) {
	res.WriteHeader(code)
	json.NewEncoder(res).Encode(data)
}

// authenticateUser - checks if the users credentials are valid
func authenticateUser(user *User, username, password string) ([]byte, error) {
	client, err := srpEnv.NewClient([]byte(username), []byte(password))
	if err != nil {
		return nil, err
	}

	creds := client.Credentials()
	ih, A, err := srp.ServerBegin(creds)
	if err != nil {
		return nil, err
	}

	if user.IH != ih {
		return nil, errors.New("IH's didn't match")
	}

	s, verif, err := srp.MakeSRPVerifier(user.Verifier)
	if err != nil {
		return nil, err
	}

	srv, err := s.NewServer(verif, A)
	if err != nil {
		return nil, err
	}

	creds = srv.Credentials()
	cauth, err := client.Generate(creds)
	if err != nil {
		return nil, err
	}

	proof, ok := srv.ClientOk(cauth)
	if !ok {
		return nil, errors.New("Authentication Failed")
	}
	if !client.ServerOk(proof) {
		return nil, errors.New("Authentication Failed")
	}
	if 1 != subtle.ConstantTimeCompare(client.RawKey(), srv.RawKey()) {
		return nil, errors.New("Authentication Failed")
	}

	return A.Bytes(), nil
}

func verifyTOTP(user *User, passcode string) error {
	key, _ := hex.DecodeString(secretKey)
	ciphertext, _ := hex.DecodeString(user.TOTP)

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	if len(ciphertext) < aes.BlockSize {
		return errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	if !totp.Validate(passcode, string(ciphertext)) {
		return err
	}

	return nil
}
