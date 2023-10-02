package tools

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

var test = false

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Blue = "\033[34m"

type errorResponse struct {
	Error string `json:"error"`
}

func checkSession(session_token string) (bool, error) {

	log.Printf("checkSession: %s\n", session_token)
	//check if session token is valid
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return false, fmt.Errorf("getInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer db.Close()

	//get influx token from session
	influx_token, err := db.Query("SELECT influxToken FROM sys.sessions WHERE sessionToken = ?", session_token)
	if err != nil {
		return false, fmt.Errorf("getInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer influx_token.Close()
	if influx_token.Next() {
		return true, nil
	}
	return false, nil
}

func sendError(w http.ResponseWriter, errorStatus int, err error) {
	//send error response
	fmt.Printf(Red)
	log.Printf("error: %s\n", err)
	fmt.Printf(Reset)
	w.WriteHeader(errorStatus)
	json.NewEncoder(w).Encode(errorResponse{Error: err.Error()})
}

func randomHex(n int) (string, error) {
	//generate random hex string
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func schalterDbTimer() {
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return
	}
	defer db.Close()

	for {
		time.Sleep(1 * time.Second)
		//get schalter status
		schalter_status, err := db.Query("SELECT name, state, locked FROM sys.schalter")
		if err != nil {
			return
		}
		defer schalter_status.Close()

		for schalter_status.Next() {
			var name string
			var state string
			var locked int
			err := schalter_status.Scan(&name, &state, &locked)
			if err != nil {
				log.Printf(Red+"error updating schalter status: %s\n"+Reset, err)
				return
			}

			if locked > 0 {
				if test {
					log.Printf(Blue + "schalterDbTimer: " + name + " " + strconv.Itoa(locked) + Reset + "\n")
				}
				locked--
				_, err = db.Exec("UPDATE sys.schalter SET locked = ? WHERE name = ?", locked, name)
				if err != nil {
					log.Printf(Red+"error updating schalter status: %s\n"+Reset, err)
					return
				}
			}
		}
	}
}

func first[T, U any](val T, _ U) T {
	//returns first value
	return val
}
