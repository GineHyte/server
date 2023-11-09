package Register

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"

	. "github.com/GineHyte/server/models"
	. "github.com/GineHyte/server/utils/tools"
)

func Register(w http.ResponseWriter, r *http.Request) {
	//Register new account
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)

		if err != nil {
			SendError(w, http.StatusBadRequest, fmt.Errorf("error decoding json: %s", err))
			return
		}

		username := t["username"].(string)
		password := t["password"].(string)
		firstname := t["firstname"].(string)
		lastname := t["lastname"].(string)
		email := t["email"].(string)

		//check if username is valid
		if username == "" {
			SendError(w, http.StatusBadRequest, errors.New("no username"))
			return
		}

		//check if password is valid
		if password == "" {
			SendError(w, http.StatusBadRequest, errors.New("no password"))
			return
		}

		//check if account already exists
		is_already_used, err := CheckIfAccountExists(username, email)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error checking if account exists: %s", err))
			return
		}
		if is_already_used {
			SendError(w, http.StatusBadRequest, errors.New("username or email already used"))
			return
		}

		//get influxdb token
		influx_token, err := CreateInfluxDBToken(username)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error creating influxdb token: %s", err))
			return
		}

		//create db account
		err = CreateDBAccount(username, password, firstname, lastname, email, influx_token)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error creating db account: %s", err))
			return
		}

		//send response
		json.NewEncoder(w).Encode(RegisterResponse{Success: true})
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func CreateInfluxDBToken(username string) (string, error) {
	//create influxdb token with username as description
	//http url for influxdb
	API_URL := os.Getenv("API_URL")
	httpposturl := API_URL + "/api/v2/Authorizations"

	//create request body
	ORG_ID := os.Getenv("ORG_ID")
	var jsonStr = []byte("{" + `"status":"active","description":"` + username + `","orgID": "` + ORG_ID + `", "permissions": [{"action": "read","resource": {"orgID": "` + ORG_ID + `","type": "buckets"}}]` + "}")

	//create http request
	ADMIN_TOKEN := os.Getenv("ADMIN_TOKEN")
	req, err := http.NewRequest("POST", httpposturl, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+ADMIN_TOKEN)
	if err != nil {
		return "", fmt.Errorf("CreateInfluxDBToken: %s", err)
	}

	//send http request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("CreateInfluxDBToken: %s", err)
	}
	defer resp.Body.Close()

	//read response
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	//parse response
	var result map[string]interface{}
	json.Unmarshal([]byte(respStr), &result)
	token := result["token"].(string)

	return token, nil
}

func CheckIfAccountExists(username string, email string) (bool, error) {
	//check if account already exists
	//db connection
	db, err := DBConnection()
	if err != nil {
		return false, fmt.Errorf("CheckIfAccountExists %s: %s", username, err)
	}
	defer db.Close()

	//get account via username or email
	is_already_used, err := db.Query("SELECT username FROM sys.accounts WHERE username = ? OR email = ?", username, email)
	if err != nil {
		return false, fmt.Errorf("CheckIfAccountExists %s: %s", username, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		return true, nil
	}
	return false, nil
}

func CreateDBAccount(username string, password string, firstname string, lastname string, email string, influx_token string) error {
	//create db account with username, password, firstname, lastname, email and influx_token
	//db connection
	db, err := DBConnection()
	if err != nil {
		return fmt.Errorf("CreateDBAccount %s: %s", username, err)
	}
	defer db.Close()

	//check if username is already used
	is_already_used, err := db.Query("SELECT username FROM sys.accounts WHERE username = ?", username)
	if err != nil {
		return fmt.Errorf("CreateDBAccount %s: %s", username, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		return errors.New("CreateDBAccount: username already used")
	}

	//check if email is already used
	is_already_used, err = db.Query("SELECT email FROM sys.accounts WHERE email = ?", email)
	if err != nil {
		return fmt.Errorf("CreateDBAccount %s: %s", username, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		return errors.New("CreateDBAccount: email already used")
	}

	//create db account
	_, err = db.Exec("INSERT INTO sys.accounts (username, password, firstname, lastname, email, influxToken) VALUES (?, ?, ?, ?, ?, ?)", username, password, firstname, lastname, email, influx_token)
	if err != nil {
		return fmt.Errorf("CreateDBAccount %s: %s", username, err)
	}

	return nil
}
