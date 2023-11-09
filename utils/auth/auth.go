package Auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql"

	. "github.com/GineHyte/server/models"
	scripter "github.com/GineHyte/server/utils/scripter"
	. "github.com/GineHyte/server/utils/tools"
)

// AuthResponse is the response to the auth request
func Auth(w http.ResponseWriter, r *http.Request) {
	//Authenticate user with username and password
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)

		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error decoding json: %s", err))
			return
		}
		log.Printf("Auth: %s\n", t["username"].(string))

		//check if username is valid
		influx_token, err := GetInfluxToken(t["username"].(string), t["password"].(string))
		if err != nil {
			SendError(w, http.StatusUnauthorized, fmt.Errorf("error getting influxdb token: %s", err))
			return
		}

		//create session
		session_token, err := CreateSession(influx_token)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error creating session: %s", err))
			return
		}

		//start all scripts
		scripter.StartAllDBStartedScripts(session_token)
		log.Printf("Started all scripts for %s\n", t["username"].(string))

		//get user data
		firstname, lastname, email, err := GetUserData(influx_token)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error getting user data: %s", err))
			return
		}
		json.NewEncoder(w).Encode(AuthResponse{SessionToken: session_token, FirstName: firstname, LastName: lastname, Email: email})
		log.Printf(Green+"Auth success: %s %s\n", firstname, lastname)
		fmt.Print(Reset)
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func GetInfluxToken(username string, password string) (string, error) {
	//get influxdb token from username and password
	//db connection
	db, err := DBConnection()
	if err != nil {
		return "", fmt.Errorf("GetInfluxToken %s: %s", username, err)
	}
	defer db.Close()

	influx_token, err := db.Query("SELECT influxToken FROM sys.accounts WHERE username = ? AND password = ?", username, password)
	if err != nil {
		return "", fmt.Errorf("GetInfluxToken %s: %s", username, err)
	}
	defer influx_token.Close()
	if influx_token.Next() {
		var token string
		err := influx_token.Scan(&token)
		if err != nil {
			return "", fmt.Errorf("GetInfluxToken %s: %s", username, err)
		}
		return token, nil
	}
	return "", errors.New("GetInfluxToken: no token found")
}

func CreateSession(influx_token string) (string, error) {
	//create session with influx token
	//db connection
	db, err := DBConnection()
	if err != nil {
		return "", fmt.Errorf("CreateSession %s: %s", influx_token, err)
	}
	defer db.Close()

	session_token, _ := RandomHex(32)

	is_already_used, err := db.Query("SELECT sessionToken FROM sys.sessions WHERE influxToken = ?", influx_token)

	if err != nil {
		return "", fmt.Errorf("CreateSession %s: %s", influx_token, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		var token string
		err := is_already_used.Scan(&token)
		if err != nil {
			return "", fmt.Errorf("CreateSession %s: %s", influx_token, err)
		}
		_, err = db.Exec("UPDATE sys.sessions SET sessionToken = ? WHERE influxToken = ?", session_token, influx_token)
		if err != nil {
			return "", fmt.Errorf("CreateSession %s: %s", influx_token, err)
		}
		return session_token, nil
	} else {
		_, err = db.Exec("INSERT INTO sys.sessions (influxToken, sessionToken) VALUES (?, ?)", influx_token, session_token)
		if err != nil {
			return "", fmt.Errorf("CreateSession %s: %s", influx_token, err)
		}

		return session_token, nil
	}
}

func GetUserData(influx_token string) (string, string, string, error) {
	//get user data from influx token
	//db connection
	db, err := DBConnection()
	if err != nil {
		return "", "", "", fmt.Errorf("GetUserData %s: %s", influx_token, err)
	}
	defer db.Close()

	//get user data from influx token
	user_data, err := db.Query("SELECT firstname, lastname, email FROM sys.accounts WHERE influxToken = ?", influx_token)
	if err != nil {
		return "", "", "", fmt.Errorf("GetUserData %s: %s", influx_token, err)
	}
	defer user_data.Close()

	if user_data.Next() {
		var firstname string
		var lastname string
		var email string
		err := user_data.Scan(&firstname, &lastname, &email)
		if err != nil {
			return "", "", "", fmt.Errorf("GetUserData %s: %s", influx_token, err)
		}
		return firstname, lastname, email, nil
	}
	return "", "", "", errors.New("GetUserData: no user data found")
}
