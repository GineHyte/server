package schalter

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

	. "github.com/GineHyte/server/models"
	"github.com/GineHyte/server/utils/tools"
	. "github.com/GineHyte/server/utils/tools"

	"github.com/r3labs/sse/v2"
)

var test = false

func SchalterControl(w http.ResponseWriter, r *http.Request) {
	//quering Schalter with session token and SchalterCommand
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error decoding json: %s", err))
			return
		}

		//parse session token
		session_token := t["session_token"].(string)
		if session_token == "" {
			SendError(w, http.StatusBadRequest, errors.New("no session token"))
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

		temp := t["command"].(map[string]interface{})
		//convert SchalterCommand to SchalterStatus
		SchalterStatusRes := SchalterStatus{}
		for key, value := range temp {
			switch key {
			case "name":
				SchalterStatusRes.Name = value.(string)
			case "state":
				SchalterStatusRes.State = value.(string)
			case "locked":
				SchalterStatusRes.Locked, _ = strconv.Atoi(value.(string))
			case "widgetId":
				SchalterStatusRes.WidgetId, _ = value.(string)
			case "scriptState":
				SchalterStatusRes.ScriptState, _ = value.(bool)
			}
		}

		//check if SchalterCommand is valid
		if SchalterStatusRes == (SchalterStatus{}) {
			SendError(w, http.StatusBadRequest, errors.New("no SchalterCommand"))
			return
		}

		//sync with Schalter db
		dbSchalterStatus, err := GetSchalterStatus(SchalterStatusRes.Name)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error syncing Schalter data: %s", err))
			return
		}

		if dbSchalterStatus.Locked > 0 {
			SendError(w, http.StatusForbidden, errors.New("schalter is locked"))
			return
		}

		if dbSchalterStatus.State == SchalterStatusRes.State {
			log.Printf(Red + "schalter is already \n" + dbSchalterStatus.State + Reset)
			return
		}

		if test {
			fmt.Printf(Blue + "THIS IS TEST MODE" + Reset + "\n")
		}

		//update Schalter db
		err = UpdateSchalterStatus(SchalterStatusRes)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error updating schalter status: %s", err))
			return
		}

		var SchalterRawData string
		SchalterRawData, err = GetRawSchalterStatus()
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error getting raw schalter status: %s", err))
			return
		}
		var SchalterData []SchalterStatus
		SchalterData, err = ParseSchalterStatus(SchalterRawData)
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error parsing schalter status: %s", err))
			return
		}

		var i = SchalterDataContains(SchalterData, SchalterStatusRes.WidgetId)
		var timerDuration time.Duration

		if strings.Contains(SchalterStatusRes.Name, "Licht") {
			//Query Schalter
			err = SchalterControllFunc(SchalterStatusRes.Name, SchalterStatusRes.State)
			if err != nil {
				SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			return
		}
		if SchalterStatusRes.State == "ON" && i != -1 {
			//Query Schalter
			err = SchalterControllFunc(SchalterData[i].Name, "ON")
			if err != nil {
				SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			err = SchalterControllFunc(SchalterData[i+1].Name, "ON")
			if err != nil {
				SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying schalter: %s", err))
				return
			}
			timerDuration = 45 * time.Second
			log.Printf("schalter %s is now %s%s%s, lock: %s\n", SchalterStatusRes.Name, Green, SchalterStatusRes.State, Reset, strconv.Itoa(SchalterStatusRes.Locked))
		}
		if SchalterStatusRes.State == "OFF" && i != -1 {
			//Query Schalter
			err = SchalterControllFunc(SchalterData[i].Name, "OFF")
			if err != nil {
				SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying qchalter: %s", err))
				return
			}
			err = SchalterControllFunc(SchalterData[i+1].Name, "ON")
			if err != nil {
				SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying qchalter: %s", err))
				return
			}
			timerDuration = 55 * time.Second
			log.Printf("schalter %s is now %s%s%s, lock: %s\n", SchalterStatusRes.Name, Green, SchalterStatusRes.State, Reset, strconv.Itoa(SchalterStatusRes.Locked))
		}

		//wait for timer
		time.AfterFunc(timerDuration, func() {
			//Query Schalter
			err = SchalterControllFunc(SchalterData[i].Name, "OFF")
			if err != nil {
				SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying Schalter: %s", err))
				return
			}
			err = SchalterControllFunc(SchalterData[i+1].Name, "OFF")
			if err != nil {
				SendError(w, http.StatusInternalServerError, fmt.Errorf("error querying Schalter: %s", err))
				return
			}
		})

		return
	case "GET":
		//get Schalter status
		status, err := GetSchalterStatuses()
		if err != nil {
			SendError(w, http.StatusInternalServerError, fmt.Errorf("error getting Schalter status: %s", err))
			return
		}
		json.NewEncoder(w).Encode(status)
		return
	default:
		log.Printf(Red + "Sorry, only POST method is supported.\n" + r.Method + Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func SchalterControllFunc(target string, state string) error {
	if test {
		fmt.Printf(Blue + "NO Schalter Query" + Reset + "\n")
		fmt.Printf(Blue+"target: %s\n", target+Reset)
		if state == "ON" {
			fmt.Printf(Green+"state: %s\n", state+Reset)
		} else {
			fmt.Printf(Red+"state: %s\n", state+Reset)
		}
		return nil
	}
	//http url for influxdb
	Schalter_IP := os.Getenv("schalter_IP")
	httpposturl := Schalter_IP + "rest/items/" + target

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

func GetSchalterStatuses() ([]SchalterStatus, error) {
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return []SchalterStatus{}, fmt.Errorf("getSchalterStatuses: %s", err)
	}
	defer db.Close()

	//get all schalter status
	var Schalter_data *sql.Rows
	Schalter_data, err = db.Query("SELECT name, state, widgetId, locked, scriptState, currentCommand FROM sys.Schalter")
	if err != nil {
		return []SchalterStatus{}, fmt.Errorf("getSchalterStatuses: %s", err)
	}
	defer Schalter_data.Close()

	SchalterStatuses := make([]SchalterStatus, 0)
	for Schalter_data.Next() {
		var name string
		var state string
		var widgetId string
		var locked int
		var scriptState bool
		var currentCommand *string

		err := Schalter_data.Scan(&name, &state, &widgetId, &locked, &scriptState, &currentCommand)
		if err != nil {
			return []SchalterStatus{}, fmt.Errorf("getSchalterStatuses: %s", err)
		}

		SchalterStatuses = append(SchalterStatuses, SchalterStatus{Name: name, State: state, WidgetId: widgetId, Locked: locked, ScriptState: scriptState, CurrentCommand: currentCommand})
	}

	return SchalterStatuses, nil
}

func SchalterDataContains(SchalterData []SchalterStatus, widgetId string) int {
	//check if SchalterData contains widgetId
	for i, Schalter := range SchalterData {
		if Schalter.WidgetId == widgetId {
			return i
		}
	}
	return -1
}

func UpdateSchalterStatus(SchalterStatusRes SchalterStatus, widgetId ...string) error {
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

	//update Schalter db
	if len(widgetId) > 0 {
		_, err = db.Exec("UPDATE sys.Schalter SET state = ?, locked = ? WHERE widgetId = ?", SchalterStatusRes.State, SchalterStatusRes.Locked, widgetId[0])
	} else {
		_, err = db.Exec("UPDATE sys.Schalter SET state = ?, locked = ? WHERE name = ?", SchalterStatusRes.State, SchalterStatusRes.Locked, SchalterStatusRes.Name)
	}
	if err != nil {
		return fmt.Errorf("updateSchalterStatus: %s", err)
	}

	return nil
}

func GetSchalterStatus(name string, widgetId ...string) (SchalterStatus, error) {
	//db connection
	DB_NAME := os.Getenv("DB_NAME")
	DB_PASSWORD := os.Getenv("DB_PASSWORD")
	DB_USERNAME := os.Getenv("DB_USERNAME")
	DB_IP := os.Getenv("DB_IP")

	db, err := sql.Open("mysql",
		DB_USERNAME+":"+DB_PASSWORD+"@tcp("+DB_IP+":3306)/"+DB_NAME)
	if err != nil {
		return SchalterStatus{}, fmt.Errorf("syncSchalterData: %s", err)
	}
	defer db.Close()

	var Schalter_data *sql.Rows
	//read data from db and compare with Schalter data
	if len(widgetId) > 0 {
		Schalter_data, err = db.Query("SELECT name, state, widgetId, locked, scriptState, currentCommand FROM sys.Schalter WHERE widgetId = ?", widgetId[0])
	} else {
		Schalter_data, err = db.Query("SELECT name, state, widgetId, locked, scriptState, currentCommand FROM sys.Schalter WHERE name = ?", name)
	}
	if err != nil {
		return SchalterStatus{}, fmt.Errorf("syncSchalterData: %s", err)
	}
	defer Schalter_data.Close()

	if Schalter_data.Next() {
		var name string
		var state string
		var widgetId string
		var locked int
		var scriptState bool
		var currentCommand *string

		err := Schalter_data.Scan(&name, &state, &widgetId, &locked, &scriptState, &currentCommand)
		if err != nil {
			return SchalterStatus{}, fmt.Errorf("syncSchalterData: %s", err)
		}
		SchalterStatusRes := SchalterStatus{Name: name, State: state, WidgetId: widgetId, Locked: locked, ScriptState: scriptState, CurrentCommand: currentCommand}
		return SchalterStatusRes, nil
	}
	return SchalterStatus{}, errors.New("syncSchalterData: no Schalter data found")
}

func GetRawSchalterStatus() (string, error) {
	//http url for influxdb
	Schalter_IP := os.Getenv("schalter_IP")
	httpposturl := Schalter_IP + "basicui/app?w=0300&sitemap=traumhaus"

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

func ParseSchalterStatus(rawData string) ([]SchalterStatus, error) {
	SchalterStatusLines := strings.Split(rawData, "\n")
	SchalterStatusRes := make([]SchalterStatus, 0)
	var widgetId string
	for _, line := range SchalterStatusLines {
		if strings.Contains(line, "<input type=\"checkbox\"") {
			if strings.Contains(line, "checked") {
				SchalterStatusRes = append(SchalterStatusRes, SchalterStatus{strings.Split(strings.Split(line, "id=\"oh-checkbox-")[1], "\"")[0], "ON", widgetId, 0, false, nil})
			} else {
				SchalterStatusRes = append(SchalterStatusRes, SchalterStatus{strings.Split(strings.Split(line, "id=\"oh-checkbox-")[1], "\"")[0], "OFF", widgetId, 0, false, nil})
			}
		}
		if strings.Contains(line, "data-widget-id=") {
			widgetId = strings.Split(strings.Split(line, "data-widget-id=\"")[1], "\"")[0]
		}
	}
	return SchalterStatusRes, nil
}

func SchalterEventStream() {
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

	//subscribe to Schalter event
	clientStream.Subscribe("event", func(msg *sse.Event) {
		rawData := string(msg.Data)

		if strings.Contains(rawData, "ALIVE") {
			return
		}

		//parse Schalter status
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
			err = UpdateSchalterStatus(SchalterStatus{Name: name, State: state, Locked: 0})
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

		// get Schalter status from html
		rawSchalterStatus, err := GetRawSchalterStatus()
		if err != nil {
			log.Printf(Red+"error getting raw schalter status: %s\n"+Reset, err)
			return
		}

		SchalterStatuses, err := ParseSchalterStatus(rawSchalterStatus)
		if err != nil {
			log.Printf(Red+"error parsing schalter status: %s\n"+Reset, err)
			return
		}

		//chrck if schater on or off
		SchalterStatusIndex := SchalterDataContains(SchalterStatuses, widgetId)
		if SchalterStatusIndex == -1 {
			log.Printf(Red+"error getting schalter status: %s\n"+Reset, err)
			return
		}
		richtungState := SchalterStatuses[SchalterStatusIndex].State

		dbSchalterStatus, err := GetSchalterStatus(name, widgetId)
		if err != nil {
			log.Printf(Red+"error syncing schalter data: %s\n"+Reset, err)
			return
		}

		if dbSchalterStatus.Locked > 0 {
			return
		}

		//update Schalter status
		if state == "ON" {
			if richtungState == "ON" {
				err = UpdateSchalterStatus(SchalterStatus{Name: name, State: richtungState, Locked: 50}, widgetId)
				log.Printf("schalterCommand name: " + name + Green + " state: " + richtungState + Reset + "\n")
			} else {
				err = UpdateSchalterStatus(SchalterStatus{Name: name, State: richtungState, Locked: 60}, widgetId)
				log.Printf("schalterCommand name: " + name + Red + " state: " + richtungState + Reset + "\n")
			}
		}
		if err != nil {
			log.Printf(Red+"error updating schalter status: %s\n"+Reset, err)
			return
		}
	})
}
