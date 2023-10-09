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

	models "github.com/GineHyte/server/models"
)

var test = false

func CheckSession(session_token string) (bool, error) {

	log.Printf("CheckSession: %s\n", session_token)
	//check if session token is valid
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return false, fmt.Errorf("GetInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer db.Close()

	//get influx token from session
	influx_token, err := db.Query("SELECT influxToken FROM sys.sessions WHERE sessionToken = ?", session_token)
	if err != nil {
		return false, fmt.Errorf("GetInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer influx_token.Close()
	if influx_token.Next() {
		return true, nil
	}
	return false, nil
}

func SendError(w http.ResponseWriter, errorStatus int, err error) {
	//send error response
	fmt.Print(models.Red)
	log.Printf("error: %s\n", err)
	fmt.Print(models.Reset)
	w.WriteHeader(errorStatus)
	json.NewEncoder(w).Encode(models.ErrorResponse{Error: err.Error()})
}

func RandomHex(n int) (string, error) {
	//generate random hex string
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func SchalterDbTimer() {
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
		//get Schalter status
		Schalter_status, err := db.Query("SELECT name, state, locked FROM sys.Schalter")
		if err != nil {
			return
		}
		defer Schalter_status.Close()

		for Schalter_status.Next() {
			var name string
			var state string
			var locked int
			err := Schalter_status.Scan(&name, &state, &locked)
			if err != nil {
				log.Printf(models.Red+"error updating Schalter status: %s\n"+models.Reset, err)
				return
			}

			if locked > 0 {
				if test {
					log.Printf(models.Blue + "schalterDbTimer: " + name + " " + strconv.Itoa(locked) + models.Reset + "\n")
				}
				locked--
				_, err = db.Exec("UPDATE sys.Schalter SET locked = ? WHERE name = ?", locked, name)
				if err != nil {
					log.Printf(models.Red+"error updating Schalter status: %s\n"+models.Reset, err)
					return
				}
			}
		}
	}
}

func First[T, U any](val T, _ U) T {
	//returns first value
	return val
}

// TODO: replace all db connections with this function
func DBConnection() (*sql.DB, error) {
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %s", err)
	}
	return db, nil
}
