package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func auth(w http.ResponseWriter, r *http.Request) {
	//authenticate user with username and password
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)

		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error decoding json: %s", err))
			return
		}
		log.Printf("auth: %s\n", t["username"].(string))

		//check if username is valid
		influx_token, err := getInfluxToken(t["username"].(string), t["password"].(string))
		if err != nil {
			sendError(w, http.StatusUnauthorized, fmt.Errorf("error getting influxdb token: %s", err))
			return
		}

		//create session
		session_token, err := createSession(influx_token)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error creating session: %s", err))
			return
		}

		//get user data
		firstname, lastname, email, err := getUserData(influx_token)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error getting user data: %s", err))
			return
		}
		json.NewEncoder(w).Encode(authResponse{SessionToken: session_token, FirstName: firstname, LastName: lastname, Email: email})
		log.Printf(Green+"auth success: %s %s\n", firstname, lastname)
		fmt.Printf(Reset)
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func getInfluxToken(username string, password string) (string, error) {
	//get influxdb token from username and password
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return "", fmt.Errorf("getInfluxToken %s: %s", username, err)
	}
	defer db.Close()

	influx_token, err := db.Query("SELECT influxToken FROM sys.accounts WHERE username = ? AND password = ?", username, password)
	if err != nil {
		return "", fmt.Errorf("getInfluxToken %s: %s", username, err)
	}
	defer influx_token.Close()
	if influx_token.Next() {
		var token string
		err := influx_token.Scan(&token)
		if err != nil {
			return "", fmt.Errorf("getInfluxToken %s: %s", username, err)
		}
		return token, nil
	}
	return "", errors.New("getInfluxToken: no token found")
}

func createSession(influx_token string) (string, error) {
	//create session with influx token
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return "", fmt.Errorf("createSession %s: %s", influx_token, err)
	}
	defer db.Close()

	session_token, _ := randomHex(32)

	is_already_used, err := db.Query("SELECT sessionToken FROM sys.sessions WHERE influxToken = ?", influx_token)

	if err != nil {
		return "", fmt.Errorf("createSession %s: %s", influx_token, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		var token string
		err := is_already_used.Scan(&token)
		if err != nil {
			return "", fmt.Errorf("createSession %s: %s", influx_token, err)
		}
		_, err = db.Exec("UPDATE sys.sessions SET sessionToken = ? WHERE influxToken = ?", session_token, influx_token)
		if err != nil {
			return "", fmt.Errorf("createSession %s: %s", influx_token, err)
		}
		return session_token, nil
	} else {
		_, err = db.Exec("INSERT INTO sys.sessions (influxToken, sessionToken) VALUES (?, ?)", influx_token, session_token)
		if err != nil {
			return "", fmt.Errorf("createSession %s: %s", influx_token, err)
		}

		return session_token, nil
	}
}

func getUserData(influx_token string) (string, string, string, error) {
	//get user data from influx token
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return "", "", "", fmt.Errorf("getUserData %s: %s", influx_token, err)
	}
	defer db.Close()

	//get user data from influx token
	user_data, err := db.Query("SELECT firstname, lastname, email FROM sys.accounts WHERE influxToken = ?", influx_token)
	if err != nil {
		return "", "", "", fmt.Errorf("getUserData %s: %s", influx_token, err)
	}
	defer user_data.Close()

	if user_data.Next() {
		var firstname string
		var lastname string
		var email string
		err := user_data.Scan(&firstname, &lastname, &email)
		if err != nil {
			return "", "", "", fmt.Errorf("getUserData %s: %s", influx_token, err)
		}
		return firstname, lastname, email, nil
	}
	return "", "", "", errors.New("getUserData: no user data found")
}
