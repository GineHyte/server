package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/r3labs/sse/v2"
)

var test = false

/* colors*/
/*
var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Purple = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"/*

/* types */
/*
type Pair struct {
	Title string  `json:"title"`
	Value float64 `json:"value"`
}

type schalterStatus struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	WidgetId string `json:"widgetId"`
	Locked   int    `json:"locked"`
}

type queryResponse struct {
	LineSets [][]Pair `json:"lineSets"`
	Names    []string `json:"names"`
	Amount   int      `json:"amount"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type authResponse struct {
	SessionToken string `json:"session_token"`
	FirstName    string `json:"firstname"`
	LastName     string `json:"lastname"`
	Email        string `json:"email"`
}

type registerResponse struct {
	Success bool `json:"success"`
}
*/
/* handlers */
/*
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
*/
func query(w http.ResponseWriter, r *http.Request) {
	//query influxdb with session token and query
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

		//check if session token is valid
		session_token := t["session_token"].(string)
		if session_token == "" {
			sendError(w, http.StatusUnauthorized, errors.New("no session token"))
			return
		}

		//check if session token is valid
		is_valid, err := checkSession(session_token)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error checking session: %s", err))
			return
		}
		if !is_valid {
			sendError(w, http.StatusForbidden, errors.New("session token is invalid"))
			return
		}

		//check if query is valid
		query := t["query"].(string)

		if query == "" {
			sendError(w, http.StatusBadRequest, errors.New("no query"))
			return
		}

		//query influxdb
		resp, err := queryInfluxDB(session_token, query)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying influxdb: %s", err))
			return
		}

		//send response
		json.NewEncoder(w).Encode(resp)
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func register(w http.ResponseWriter, r *http.Request) {
	//register new account
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)

		if err != nil {
			sendError(w, http.StatusBadRequest, fmt.Errorf("error decoding json: %s", err))
			return
		}

		username := t["username"].(string)
		password := t["password"].(string)
		firstname := t["firstname"].(string)
		lastname := t["lastname"].(string)
		email := t["email"].(string)

		//check if username is valid
		if username == "" {
			sendError(w, http.StatusBadRequest, errors.New("no username"))
			return
		}

		//check if password is valid
		if password == "" {
			sendError(w, http.StatusBadRequest, errors.New("no password"))
			return
		}

		//check if account already exists
		is_already_used, err := checkIfAccountExists(username, email)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error checking if account exists: %s", err))
			return
		}
		if is_already_used {
			sendError(w, http.StatusBadRequest, errors.New("username or email already used"))
			return
		}

		//get influxdb token
		influx_token, err := createInfluxDBToken(username)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error creating influxdb token: %s", err))
			return
		}

		//create db account
		err = createDBAccount(username, password, firstname, lastname, email, influx_token)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error creating db account: %s", err))
			return
		}

		//send response
		json.NewEncoder(w).Encode(registerResponse{Success: true})
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func schalterControl(w http.ResponseWriter, r *http.Request) {
	//quering schalter with session token and schalterCommand
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error decoding json: %s", err))
			return
		}

		//parse session token
		session_token := t["session_token"].(string)
		if session_token == "" {
			sendError(w, http.StatusBadRequest, errors.New("no session token"))
			return
		}

		//check if session token is valid
		is_valid, err := checkSession(session_token)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error checking session: %s", err))
			return
		}
		if !is_valid {
			sendError(w, http.StatusForbidden, errors.New("session token is invalid"))
			return
		}

		temp := t["command"].(map[string]interface{})
		//convert schalterCommand to schalterStatus
		schalterStatusRes := schalterStatus{}
		for key, value := range temp {
			switch key {
			case "name":
				schalterStatusRes.Name = value.(string)
			case "state":
				schalterStatusRes.State = value.(string)
			case "locked":
				schalterStatusRes.Locked, _ = strconv.Atoi(value.(string))
			case "widgetId":
				schalterStatusRes.WidgetId, _ = value.(string)
			}
		}

		//check if schalterCommand is valid
		if schalterStatusRes == (schalterStatus{}) {
			sendError(w, http.StatusBadRequest, errors.New("no schalterCommand"))
			return
		}

		//sync with schalter db
		dbSchalterStatus, err := getSchalterStatus(schalterStatusRes.Name)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error syncing schalter data: %s", err))
			return
		}

		if dbSchalterStatus.Locked > 0 {
			sendError(w, http.StatusForbidden, errors.New("schalter is locked"))
			return
		}

		if dbSchalterStatus.State == schalterStatusRes.State {
			log.Printf(Red + "schalter is already \n" + dbSchalterStatus.State + Reset)
			return
		}

		if test {
			fmt.Printf(Blue + "THIS IS TEST MODE" + Reset + "\n")
		}

		//update schalter db
		err = updateSchalterStatus(schalterStatusRes)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error updating schalter status: %s", err))
			return
		}

		var schalterRawData string
		schalterRawData, err = getRawSchalterStatus()
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error getting raw schalter status: %s", err))
			return
		}
		var schalterData []schalterStatus
		schalterData, err = parseSchalterStatus(schalterRawData)
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error parsing schalter status: %s", err))
			return
		}

		var i = schalterDataContains(schalterData, schalterStatusRes.WidgetId)
		var timerDuration time.Duration

		if strings.Contains(schalterStatusRes.Name, "Licht") {
			//query schalter
			err = schalterControllFunc(schalterStatusRes.Name, schalterStatusRes.State)
			if err != nil {
				sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			return
		}
		if schalterStatusRes.State == "ON" && i != -1 {
			//query schalter
			err = schalterControllFunc(schalterData[i].Name, "ON")
			if err != nil {
				sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			err = schalterControllFunc(schalterData[i+1].Name, "ON")
			if err != nil {
				sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			timerDuration = 45 * time.Second
			log.Printf("schalter %s is now %s%s%s, lock: %s\n", schalterStatusRes.Name, Green, schalterStatusRes.State, Reset, strconv.Itoa(schalterStatusRes.Locked))
		}
		if schalterStatusRes.State == "OFF" && i != -1 {
			//query schalter
			err = schalterControllFunc(schalterData[i].Name, "OFF")
			if err != nil {
				sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			err = schalterControllFunc(schalterData[i+1].Name, "ON")
			if err != nil {
				sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			timerDuration = 55 * time.Second
			log.Printf("schalter %s is now %s%s%s, lock: %s\n", schalterStatusRes.Name, Green, schalterStatusRes.State, Reset, strconv.Itoa(schalterStatusRes.Locked))
		}

		//wait for timer
		time.AfterFunc(timerDuration, func() {
			//query schalter
			err = schalterControllFunc(schalterData[i].Name, "OFF")
			if err != nil {
				sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			err = schalterControllFunc(schalterData[i+1].Name, "OFF")
			if err != nil {
				sendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
		})

		return
	case "GET":
		//get schalter status
		status, err := getSchalterStatuses()
		if err != nil {
			sendError(w, http.StatusInternalServerError, fmt.Errorf("error getting schalter status: %s", err))
			return
		}
		json.NewEncoder(w).Encode(status)
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Sorry, only POST method is supported."})
	}
}

/* auth  */
/*
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
*/
/* query */

func queryInfluxDB(session_token string, query string) (queryResponse, error) {
	//query influxdb with session token and query
	//get influxdb token from session
	influx_token, err := getInfluxTokenFromSession(session_token)
	if err != nil {
		return queryResponse{}, fmt.Errorf("queryInfluxDB %s: %s", session_token, err)
	}

	//http url for influxdb
	API_URL := os.Getenv("API_URL")
	ORG_ID := os.Getenv("ORG_ID")
	httpposturl := API_URL + "/api/v2/query?orgID=" + ORG_ID

	//create http request
	req, err := http.NewRequest("POST", httpposturl, bytes.NewBufferString(query))
	req.Header.Set("Content-Type", "application/vnd.flux")
	req.Header.Set("Authorization", "Token "+influx_token)
	if err != nil {
		return queryResponse{}, fmt.Errorf("queryInfluxDB %s: %s", session_token, err)
	}

	//send http request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return queryResponse{}, fmt.Errorf("queryInfluxDB %s: %s", session_token, err)
	}
	defer resp.Body.Close()

	//read response
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	return processInfluxdbResponse(respStr), nil
}

func getInfluxTokenFromSession(session_token string) (string, error) {
	//get influxdb token from session token
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return "", fmt.Errorf("getInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer db.Close()

	//get influx token from session
	influx_token, err := db.Query("SELECT influxToken FROM sys.sessions WHERE sessionToken = ?", session_token)
	if err != nil {
		return "", fmt.Errorf("getInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer influx_token.Close()
	if influx_token.Next() {
		var token string
		err := influx_token.Scan(&token)
		if err != nil {
			return "", fmt.Errorf("getInfluxTokenFromSession %s: %s", session_token, err)
		}
		return token, nil
	}
	return "", errors.New("getInfluxTokenFromSession: no token found")
}

func processInfluxdbResponse(rawResponse string) queryResponse {
	//process influxdb response to "queryResponse"
	//parse csv in 1 dimensional array
	linesResponse := strings.Split(rawResponse, "\n")
	var splitedLinesResponse [][]string
	lineSets := make([][]Pair, 0)
	names := make([]string, 0)

	//parse csv in 2 dimensional array
	for _, line := range linesResponse {
		splitedLinesResponse = append(splitedLinesResponse, strings.Split(line, ","))
	}

	//get amount of tables
	amount := first(strconv.Atoi(splitedLinesResponse[len(splitedLinesResponse)-3][2])) + 1
	splitedLinesResponse[len(splitedLinesResponse)-2] = []string{"", "", strconv.Itoa(amount + 1)}

	prevTableI, tableI := 0, 0
	j := 1

	//sort data in arrays
	for i := 0; i < amount; i++ {
		lineSets = append(lineSets, make([]Pair, 0))
		for tableI == prevTableI {
			tableI = first(strconv.Atoi(splitedLinesResponse[j][2]))
			if tableI == prevTableI {
				lineSets[i] = append(lineSets[i], Pair{Title: splitedLinesResponse[j][5], Value: first(strconv.ParseFloat(splitedLinesResponse[j][6], 64))})
			}
			j++
		}
		names = append(names, strings.Replace(splitedLinesResponse[j-2][10], "\r", "", -1))
		prevTableI = tableI
	}
	log.Printf("query(first element): %s\n", splitedLinesResponse[2][10])

	//return data
	return queryResponse{
		LineSets: lineSets,
		Names:    names,
		Amount:   amount,
	}
}

/* register */

func createInfluxDBToken(username string) (string, error) {
	//create influxdb token with username as description
	//http url for influxdb
	API_URL := os.Getenv("API_URL")
	httpposturl := API_URL + "/api/v2/authorizations"

	//create request body
	ORG_ID := os.Getenv("ORG_ID")
	var jsonStr = []byte("{" + `"status":"active","description":"` + username + `","orgID": "` + ORG_ID + `", "permissions": [{"action": "read","resource": {"orgID": "` + ORG_ID + `","type": "buckets"}}]` + "}")

	//create http request
	ADMIN_TOKEN := os.Getenv("ADMIN_TOKEN")
	req, err := http.NewRequest("POST", httpposturl, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+ADMIN_TOKEN)
	if err != nil {
		return "", fmt.Errorf("createInfluxDBToken: %s", err)
	}

	//send http request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("createInfluxDBToken: %s", err)
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

func checkIfAccountExists(username string, email string) (bool, error) {
	//check if account already exists
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return false, fmt.Errorf("checkIfAccountExists %s: %s", username, err)
	}
	defer db.Close()

	//get account via username or email
	is_already_used, err := db.Query("SELECT username FROM sys.accounts WHERE username = ? OR email = ?", username, email)
	if err != nil {
		return false, fmt.Errorf("checkIfAccountExists %s: %s", username, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		return true, nil
	}
	return false, nil
}

func createDBAccount(username string, password string, firstname string, lastname string, email string, influx_token string) error {
	//create db account with username, password, firstname, lastname, email and influx_token
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return fmt.Errorf("createDBAccount %s: %s", username, err)
	}
	defer db.Close()

	//check if username is already used
	is_already_used, err := db.Query("SELECT username FROM sys.accounts WHERE username = ?", username)
	if err != nil {
		return fmt.Errorf("createDBAccount %s: %s", username, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		return errors.New("createDBAccount: username already used")
	}

	//check if email is already used
	is_already_used, err = db.Query("SELECT email FROM sys.accounts WHERE email = ?", email)
	if err != nil {
		return fmt.Errorf("createDBAccount %s: %s", username, err)
	}
	defer is_already_used.Close()
	if is_already_used.Next() {
		return errors.New("createDBAccount: email already used")
	}

	//create db account
	_, err = db.Exec("INSERT INTO sys.accounts (username, password, firstname, lastname, email, influxToken) VALUES (?, ?, ?, ?, ?, ?)", username, password, firstname, lastname, email, influx_token)
	if err != nil {
		return fmt.Errorf("createDBAccount %s: %s", username, err)
	}

	return nil
}

/* schalterControl */

func schalterControllFunc(target string, state string) error {
	if test {
		fmt.Printf(Blue + "NO SCHALTER QUERY" + Reset + "\n")
		fmt.Printf(Blue+"target: %s\n", target+Reset)
		if state == "ON" {
			fmt.Printf(Green+"state: %s\n", state+Reset)
		} else {
			fmt.Printf(Red+"state: %s\n", state+Reset)
		}
		return nil
	}
	//http url for influxdb
	SCHALTER_IP := os.Getenv("SCHALTER_IP")
	httpposturl := SCHALTER_IP + "rest/items/" + target

	//create http request
	req, err := http.NewRequest("POST", httpposturl, bytes.NewBufferString(state))
	req.Header.Set("Content-Type", "text/plain")
	if err != nil {
		return fmt.Errorf("schalter %s: %s", target, err)
	}

	//send http request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("schalter %s: %s", target, err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	return nil
}

func getSchalterStatuses() ([]schalterStatus, error) {
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return nil, fmt.Errorf("getSchalterStatus: %s", err)
	}
	defer db.Close()

	//get schalter status
	schalter_status, err := db.Query("SELECT name, state, widgetId, locked FROM sys.schalter")
	if err != nil {
		return nil, fmt.Errorf("getSchalterStatus: %s", err)
	}
	defer schalter_status.Close()

	var schalterStatusRes []schalterStatus

	for schalter_status.Next() {
		var name string
		var state string
		var widgetId string
		var locked int
		err := schalter_status.Scan(&name, &state, &widgetId, &locked)
		if err != nil {
			return nil, fmt.Errorf("getSchalterStatus: %s", err)
		}
		schalterStatusRes = append(schalterStatusRes, schalterStatus{Name: name, State: state, WidgetId: widgetId, Locked: locked})
	}

	return schalterStatusRes, nil
}

func schalterDataContains(schalterData []schalterStatus, widgetId string) int {
	//check if schalterData contains widgetId
	for i, schalter := range schalterData {
		if schalter.WidgetId == widgetId {
			return i
		}
	}
	return -1
}

func updateSchalterStatus(schalterStatusRes schalterStatus, widgetId ...string) error {
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return fmt.Errorf("updateSchalterStatus: %s", err)
	}

	defer db.Close()

	//update schalter db
	if len(widgetId) > 0 {
		_, err = db.Exec("UPDATE sys.schalter SET state = ?, locked = ? WHERE widgetId = ?", schalterStatusRes.State, schalterStatusRes.Locked, widgetId[0])
	} else {
		_, err = db.Exec("UPDATE sys.schalter SET state = ?, locked = ? WHERE name = ?", schalterStatusRes.State, schalterStatusRes.Locked, schalterStatusRes.Name)
	}
	if err != nil {
		return fmt.Errorf("updateSchalterStatus: %s", err)
	}

	return nil
}

func getSchalterStatus(name string, widgetId ...string) (schalterStatus, error) {
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return schalterStatus{}, fmt.Errorf("syncSchalterData: %s", err)
	}
	defer db.Close()

	var schalter_data *sql.Rows
	//read data from db and compare with schalter data
	if len(widgetId) > 0 {
		schalter_data, err = db.Query("SELECT name, state, widgetId, locked FROM sys.schalter WHERE widgetId = ?", widgetId[0])
	} else {
		schalter_data, err = db.Query("SELECT name, state, widgetId, locked FROM sys.schalter WHERE name = ?", name)
	}
	if err != nil {
		return schalterStatus{}, fmt.Errorf("syncSchalterData: %s", err)
	}
	defer schalter_data.Close()

	if schalter_data.Next() {
		var name string
		var state string
		var widgetId string
		var locked int
		err := schalter_data.Scan(&name, &state, &widgetId, &locked)
		if err != nil {
			return schalterStatus{}, fmt.Errorf("syncSchalterData: %s", err)
		}
		schalterStatusRes := schalterStatus{Name: name, State: state, WidgetId: widgetId, Locked: locked}
		return schalterStatusRes, nil
	}
	return schalterStatus{}, errors.New("syncSchalterData: no schalter data found")
}

func getRawSchalterStatus() (string, error) {
	//http url for influxdb
	SCHALTER_IP := os.Getenv("SCHALTER_IP")
	httpposturl := SCHALTER_IP + "basicui/app?w=0300&sitemap=traumhaus"

	//create http request
	req, err := http.NewRequest("GET", httpposturl, nil)
	if err != nil {
		return "", fmt.Errorf("schalter: %s", err)
	}

	//send http request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("schalter: %s", err)
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	return respStr, nil
}

func parseSchalterStatus(rawData string) ([]schalterStatus, error) {
	schalterStatusLines := strings.Split(rawData, "\n")
	schalterStatusRes := make([]schalterStatus, 0)
	var widgetId string
	for _, line := range schalterStatusLines {
		if strings.Contains(line, "<input type=\"checkbox\"") {
			if strings.Contains(line, "checked") {
				schalterStatusRes = append(schalterStatusRes, schalterStatus{strings.Split(strings.Split(line, "id=\"oh-checkbox-")[1], "\"")[0], "ON", widgetId, 0})
			} else {
				schalterStatusRes = append(schalterStatusRes, schalterStatus{strings.Split(strings.Split(line, "id=\"oh-checkbox-")[1], "\"")[0], "OFF", widgetId, 0})
			}
		}
		if strings.Contains(line, "data-widget-id=") {
			widgetId = strings.Split(strings.Split(line, "data-widget-id=\"")[1], "\"")[0]
		}
	}
	return schalterStatusRes, nil
}

func schalterEventStream() {
	SCHALTER_IP := os.Getenv("SCHALTER_IP")

	subscribeUrl := SCHALTER_IP + "rest/sitemaps/events/subscribe"
	req, err := http.NewRequest("POST", subscribeUrl, nil)
	if err != nil {
		log.Printf(Red+"error creating request: %s\n"+Reset, err)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf(Red+"error sending request: %s\n"+Reset, err)
		return
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var t map[string]interface{}
	err = decoder.Decode(&t)

	if err != nil {
		log.Printf(Red+"error decoding response: %s\n"+Reset, err)
		return
	}

	urlToken := t["context"].(map[string]interface{})["headers"].(map[string]interface{})["Location"].([]interface{})[0].(string)
	streamUrl := urlToken + "?sitemap=traumhaus&pageid=0300"
	clientStream := sse.NewClient(streamUrl)

	var widgetsIdEventBus []string

	//subscribe to schalter event
	clientStream.Subscribe("event", func(msg *sse.Event) {
		rawData := string(msg.Data)

		if strings.Contains(rawData, "ALIVE") {
			return
		}

		//parse schalter status
		var t1 map[string]interface{}
		err = json.Unmarshal([]byte(rawData), &t1)
		if err != nil {
			log.Printf(Red+"error decoding response: %s\n"+Reset, err)
			return
		}

		name := t1["item"].(map[string]interface{})["name"].(string)
		state := t1["item"].(map[string]interface{})["state"].(string)
		widgetId := t1["widgetId"].(string)

		if strings.Contains(name, "Richtung") {
			if len(widgetsIdEventBus) > 10 {
				widgetsIdEventBus = widgetsIdEventBus[:len(widgetsIdEventBus)-1]
			}
			widgetsIdEventBus = append(widgetsIdEventBus, widgetId) //TODO: continue here
			return
		}

		if strings.Contains(name, "Licht") {
			err = updateSchalterStatus(schalterStatus{Name: name, State: state, Locked: 0})
			if err != nil {
				log.Printf(Red+"error updating schalter status: %s\n"+Reset, err)
				return
			}
			if state == "ON" {
				log.Printf("schalterCommand name: " + name + Green + " state: " + state + Reset + "\n")
			}
			if state == "OFF" {
				log.Printf("schalterCommand name: " + name + Red + " state: " + state + Reset + "\n")
			}
			return
		}

		//convert widgetId to int
		widgetIdInt, err := strconv.Atoi(widgetId)
		if err != nil {
			log.Printf(Red+"error converting widgetId to int: %s\n"+Reset, err)
			return
		}
		widgetId = "0" + strconv.Itoa(widgetIdInt-1)

		// get schalter status from html
		rawSchalterStatus, err := getRawSchalterStatus()
		if err != nil {
			log.Printf(Red+"error getting raw schalter status: %s\n"+Reset, err)
			return
		}

		schalterStatuses, err := parseSchalterStatus(rawSchalterStatus)
		if err != nil {
			log.Printf(Red+"error parsing schalter status: %s\n"+Reset, err)
			return
		}

		//chrck if schater on or off
		schalterStatusIndex := schalterDataContains(schalterStatuses, widgetId)
		if schalterStatusIndex == -1 {
			log.Printf(Red+"error getting schalter status: %s\n"+Reset, err)
			return
		}
		richtungState := schalterStatuses[schalterStatusIndex].State

		dbSchalterStatus, err := getSchalterStatus(name, widgetId)
		if err != nil {
			log.Printf(Red+"error syncing schalter data: %s\n"+Reset, err)
			return
		}

		if dbSchalterStatus.Locked > 0 {
			return
		}

		//update schalter status
		if state == "ON" {
			err = updateSchalterStatus(schalterStatus{Name: name, State: richtungState, Locked: 60}, widgetId)
			log.Printf("schalterCommand name: " + name + Red + " state: " + richtungState + Reset + "\n")
		}
		if err != nil {
			log.Printf(Red+"error updating schalter status: %s\n"+Reset, err)
			return
		}
	})
}

/* tools */
/*
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
}*/

/* main */

func main() {
	//load .env file
	godotenv.Load(".env")

	//clear terminal
	fmt.Print("\033[H\033[J")

	go schalterEventStream()
	go schalterDbTimer()

	//check if test mode
	if len(os.Args) > 1 {
		test = os.Args[1] == "test"
		fmt.Printf(Blue + "TEST MODE ACTIVE\n" + Reset + "")
	}

	http.HandleFunc("/register", register)
	http.HandleFunc("/auth", auth)
	http.HandleFunc("/query", query)
	http.HandleFunc("/schalter", schalterControl)

	err := http.ListenAndServe(":3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		log.Printf(Green + "server closed\n" + Reset)
	} else if err != nil {
		log.Printf(Red+"error starting server: %s\n", err)
		fmt.Printf(Reset)
		os.Exit(1)
	}
}
