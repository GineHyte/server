package Query

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

	. "github.com/GineHyte/server/models"
	. "github.com/GineHyte/server/utils/tools"
)

// QueryResponse is the response to the Query request
func Query(w http.ResponseWriter, r *http.Request) {
	//Query influxdb with session token and Query
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

		//check if session token is valid
		session_token := t["session_token"].(string)
		if session_token == "" {
			SendError(w, http.StatusUnauthorized, errors.New("no session token"))
			return
		}

		//check if session token is valid
		is_valid, err := CheckSession(session_token)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error checking session: %s", err))
			return
		}
		if !is_valid {
			SendError(w, http.StatusForbidden, errors.New("session token is invalid"))
			return
		}

		//check if Query is valid
		query := t["query"].(string)

		if query == "" {
			SendError(w, http.StatusBadRequest, errors.New("no query"))
			return
		}

		//Query influxdb
		resp, err := QueryInfluxDB(session_token, query)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying influxdb: %s", err))
			return
		}

		//send response
		json.NewEncoder(w).Encode(resp)
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func QueryInfluxDB(session_token string, Query string) (QueryResponse, error) {
	//Query influxdb with session token and Query
	//get influxdb token from session
	influx_token, err := GetInfluxTokenFromSession(session_token)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("QueryInfluxDB %s: %s", session_token, err)
	}

	//http url for influxdb
	API_URL := os.Getenv("API_URL")
	ORG_ID := os.Getenv("ORG_ID")
	httpposturl := API_URL + "/api/v2/query?orgID=" + ORG_ID

	//create http request
	req, err := http.NewRequest("POST", httpposturl, bytes.NewBufferString(Query))
	req.Header.Set("Content-Type", "application/vnd.flux")
	req.Header.Set("Authorization", "Token "+influx_token)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("QueryInfluxDB %s: %s", session_token, err)
	}

	//send http request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("QueryInfluxDB %s: %s", session_token, err)
	}
	defer resp.Body.Close()

	//read response
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	respStr := buf.String()

	return ProcessInfluxdbResponse(respStr), nil
}

func GetInfluxTokenFromSession(session_token string) (string, error) {
	//get influxdb token from session token
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return "", fmt.Errorf("GetInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer db.Close()

	//get influx token from session
	influx_token, err := db.Query("SELECT influxToken FROM sys.sessions WHERE sessionToken = ?", session_token)
	if err != nil {
		return "", fmt.Errorf("GetInfluxTokenFromSession %s: %s", session_token, err)
	}
	defer influx_token.Close()
	if influx_token.Next() {
		var token string
		err := influx_token.Scan(&token)
		if err != nil {
			return "", fmt.Errorf("GetInfluxTokenFromSession %s: %s", session_token, err)
		}
		return token, nil
	}
	return "", errors.New("GetInfluxTokenFromSession: no token found")
}

func ProcessInfluxdbResponse(rawResponse string) QueryResponse {
	//process influxdb response to "QueryResponse"
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
	amount := First(strconv.Atoi(splitedLinesResponse[len(splitedLinesResponse)-3][2])) + 1
	splitedLinesResponse[len(splitedLinesResponse)-2] = []string{"", "", strconv.Itoa(amount + 1)}

	prevTableI, tableI := 0, 0
	j := 1

	//sort data in arrays
	for i := 0; i < amount; i++ {
		lineSets = append(lineSets, make([]Pair, 0))
		for tableI == prevTableI {
			tableI = First(strconv.Atoi(splitedLinesResponse[j][2]))
			if tableI == prevTableI {
				lineSets[i] = append(lineSets[i], Pair{Title: splitedLinesResponse[j][5], Value: First(strconv.ParseFloat(splitedLinesResponse[j][6], 64))})
			}
			j++
		}
		names = append(names, strings.Replace(splitedLinesResponse[j-2][10], "\r", "", -1))
		prevTableI = tableI
	}
	log.Printf("Query(first element): %s\n", splitedLinesResponse[2][10])

	//return data
	return QueryResponse{
		LineSets: lineSets,
		Names:    names,
		Amount:   amount,
	}
}
