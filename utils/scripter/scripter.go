package scripter

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	models "github.com/GineHyte/server/models"
	query "github.com/GineHyte/server/utils/query"
	schalter "github.com/GineHyte/server/utils/schalter"
	tools "github.com/GineHyte/server/utils/tools"
)

var wg sync.WaitGroup
var wgWhile sync.WaitGroup

func Script(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)
		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error decoding json: %s", err))
			return
		}

		//check if session token is valid
		session_token := t["session_token"].(string)
		if session_token == "" {
			tools.SendError(w, http.StatusUnauthorized, errors.New("no session token"))
			return
		}

		//check if session token is valid
		is_valid, err := tools.CheckSession(session_token)
		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error checking session: %s", err))
			return
		}
		if !is_valid {
			tools.SendError(w, http.StatusForbidden, errors.New("session token is invalid"))
			return
		}

		//check if script is valid
		script := t["script"].(string)

		if script == "" {
			tools.SendError(w, http.StatusBadRequest, errors.New("no script"))
			return
		}

		//check if name is valid
		widgetId := t["widget_id"].(string)

		if widgetId == "" {
			tools.SendError(w, http.StatusBadRequest, errors.New("no widget_id"))
			return
		}

		//stop script
		err = StopScript(widgetId)
		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error stopping script: %s", err))
			return
		}

		//set script
		err = DBSetScript(script, widgetId)
		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error setting script: %s", err))
			return
		}

		//start script
		err = StartScript(widgetId, session_token)

		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error starting script: %s", err))
			return
		}

		//send success response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	case "GET":
		//get script
		script, err := DBGetScript(r.URL.Query().Get("widgetId"))
		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error getting script: %s", err))
			return
		}

		//send script
		w.Header().Set("Content-Type", "plain/text")
		json.NewEncoder(w).Encode(script)
		return
	}
}

func ControlScript(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)
		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error decoding json: %s", err))
			return
		}

		//check if session token is valid
		session_token := t["session_token"].(string)
		if session_token == "" {
			tools.SendError(w, http.StatusUnauthorized, errors.New("no session token"))
			return
		}

		//check if session token is valid
		is_valid, err := tools.CheckSession(session_token)
		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error checking session: %s", err))
			return
		}

		if !is_valid {
			tools.SendError(w, http.StatusForbidden, errors.New("session token is invalid"))
			return
		}

		//check if name is valid
		widgetId := t["widget_id"].(string)

		if widgetId == "" {
			tools.SendError(w, http.StatusBadRequest, errors.New("no widgetId"))
			return
		}

		//get command
		command := t["command"].(string)

		if command == "" {
			tools.SendError(w, http.StatusBadRequest, errors.New("no command"))
			return
		} else if command == "ON" {
			err = StartScript(widgetId, session_token)
		} else if command == "OFF" {
			err = StopScript(widgetId)
		}

		if err != nil {
			tools.SendError(w, http.StatusInternalServerError, fmt.Errorf("error executing command: %s", err))
			return
		}

		//send success response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
		return
	default:
		tools.SendError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
}

func DBGetScript(widgetId string) (string, error) {
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return "", fmt.Errorf("error opening db: %s", err)
	}
	defer db.Close()

	//get script
	var script string
	err = db.QueryRow("SELECT script FROM schalter WHERE widgetId = ?", widgetId).Scan(&script)
	if err != nil {
		return "", fmt.Errorf("error getting script: %s", err)
	}

	return script, nil
}

func DBSetScript(script string, widgetId string) error {
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return fmt.Errorf("error opening db: %s", err)
	}
	defer db.Close()

	//set script
	_, err = db.Exec("UPDATE schalter SET script = ? WHERE widgetId = ?", script, widgetId)
	if err != nil {
		return fmt.Errorf("error setting script: %s", err)
	}

	return nil
}

func ExecuteCommand(command string, widgetId string, session_token string) error {
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return fmt.Errorf("error opening db: %s", err)
	}
	defer db.Close()

	//set current command
	_, err = db.Exec("UPDATE schalter SET currentCommand = ? WHERE widgetId = ?", command, widgetId)
	if err != nil {
		return fmt.Errorf("error setting command: %s", err)
	}

	//get scriptState
	scriptState, err := GetScriptState(widgetId)
	if err != nil {
		return fmt.Errorf("error getting scriptState: %s", err)
	}

	if scriptState == "0" {
		wg = sync.WaitGroup{}
		return nil
	}

	//get command
	commandType := strings.Split(command, " ")[0]

	switch commandType {
	case "WAIT":
		waitTime, _ := strconv.Atoi(strings.Split(command, " ")[1])
		time.Sleep(time.Duration(waitTime) * time.Second)
	case "ON", "OFF":
		err = onOff(command, commandType, widgetId)
		if err != nil {
			wg = sync.WaitGroup{}
			return fmt.Errorf("error executing command: %s", err)
		}
	case "WHILE":
		condition := strings.Split(command, " ")[1:4]
		statements := strings.Split(strings.Join(strings.Split(command, " ")[4:], " "), ";")
		err = whileLoop(condition, statements, widgetId, session_token)
		if err != nil {
			wg = sync.WaitGroup{}
			return fmt.Errorf("error executing command: %s", err)
		}
	}

	//waitgroup reset
	wg = sync.WaitGroup{}

	return nil
}

func onOff(command string, commandType string, widgetId string) error {
	SchalterStatusRes := models.SchalterStatus{}
	SchalterStatusRes.Name = ""
	SchalterStatusRes.State = commandType
	SchalterStatusRes.WidgetId = widgetId
	if commandType == "ON" {
		SchalterStatusRes.Locked = 45
	} else {
		SchalterStatusRes.Locked = 55
	}

	//sync with Schalter db
	dbSchalterStatus, err := schalter.GetSchalterStatus("", SchalterStatusRes.WidgetId)
	if err != nil {
		return fmt.Errorf("error syncing Schalter data: %s", err)
	}

	if dbSchalterStatus.Locked > 0 {
		return errors.New("schalter is locked")
	}

	if dbSchalterStatus.State == SchalterStatusRes.State {
		return errors.New("schalter is already" + SchalterStatusRes.State)
	}

	//update Schalter db
	err = schalter.UpdateSchalterStatus(SchalterStatusRes)
	if err != nil {
		return fmt.Errorf("error updating schalter status: %s", err)
	}

	var SchalterRawData string
	SchalterRawData, err = schalter.GetRawSchalterStatus()
	if err != nil {
		return fmt.Errorf("error getting raw schalter status: %s", err)
	}
	var SchalterData []models.SchalterStatus
	SchalterData, err = schalter.ParseSchalterStatus(SchalterRawData)
	if err != nil {
		return fmt.Errorf("error parsing schalter status: %s", err)
	}

	var i = schalter.SchalterDataContains(SchalterData, SchalterStatusRes.WidgetId)
	var timerDuration time.Duration

	if strings.Contains(SchalterStatusRes.Name, "Licht") {
		//Query Schalter
		err = schalter.SchalterControllFunc(SchalterStatusRes.Name, SchalterStatusRes.State)
		if err != nil {
			return fmt.Errorf("error querying schalter: %s", err)
		}
		return nil
	}
	if SchalterStatusRes.State == "ON" && i != -1 {
		//Query Schalter
		err = schalter.SchalterControllFunc(SchalterData[i].Name, "ON")
		if err != nil {
			return fmt.Errorf("error querying schalter: %s", err)
		}
		err = schalter.SchalterControllFunc(SchalterData[i+1].Name, "ON")
		if err != nil {
			return fmt.Errorf("error querying schalter: %s", err)
		}
		timerDuration = 45 * time.Second
		log.Printf("schalter %s is now %s%s%s, lock: %s\n", SchalterStatusRes.Name, models.Green, SchalterStatusRes.State, models.Reset, strconv.Itoa(SchalterStatusRes.Locked))
	}
	if SchalterStatusRes.State == "OFF" && i != -1 {
		//Query Schalter
		err = schalter.SchalterControllFunc(SchalterData[i].Name, "OFF")
		if err != nil {
			return fmt.Errorf("error querying qchalter: %s", err)
		}
		err = schalter.SchalterControllFunc(SchalterData[i+1].Name, "ON")
		if err != nil {
			return fmt.Errorf("error querying qchalter: %s", err)
		}
		timerDuration = 55 * time.Second
		log.Printf("schalter %s is now %s%s%s, lock: %s\n", SchalterStatusRes.Name, models.Green, SchalterStatusRes.State, models.Reset, strconv.Itoa(SchalterStatusRes.Locked))
	}

	//wait for timer
	time.Sleep(timerDuration)

	//Query Schalter
	err = schalter.SchalterControllFunc(SchalterData[i].Name, "OFF")
	if err != nil {
		return fmt.Errorf("error querying schalter: %s", err)
	}
	err = schalter.SchalterControllFunc(SchalterData[i+1].Name, "OFF")
	if err != nil {
		return fmt.Errorf("error querying schalter: %s", err)
	}
	return nil
}

func whileLoop(condition []string, statements []string, widgetId string, session_token string) error {
	//parse condition
	param1 := condition[0]
	param2 := condition[2]
	operator := condition[1]

	var getDataFor = 0
	var name string

	// Try to parse param1 to float and date
	param1FloatCheck, _ := strconv.ParseFloat(param1, 64)
	param1DateCheck, _ := time.Parse("2006-01-02T15:04:05", param1)

	// Try to parse param2 to float and date
	param2FloatCheck, _ := strconv.ParseFloat(param2, 64)
	param2DateCheck, _ := time.Parse("2006-01-02T15:04:05", param2)

	if param1FloatCheck != 0 || param2FloatCheck != 0 {
		if param1FloatCheck != 0 {
			if param2FloatCheck != 0 {
			} else {
				getDataFor = 2
				name = param2
			}
		} else if param2FloatCheck != 0 {
			getDataFor = 1
			name = param1
		}

	} else if param1DateCheck != (time.Time{}) || param2DateCheck != (time.Time{}) {
		if param1DateCheck != (time.Time{}) {
			getDataFor = 2
			name = param2
		}
		if param2DateCheck != (time.Time{}) {
			getDataFor = 1
			name = param1
		}
	}
	func() error {
		for {
			wgWhile.Wait()
			//get scriptState
			scriptState, err := GetScriptState(widgetId)
			if err != nil {
				return fmt.Errorf("error getting scriptState: %s", err)
			}

			if scriptState == "0" {
				return nil
			}
			wgWhile.Add(1)

			var tParam1, tParam2 time.Time

			if getDataFor > 0 {
				if getDataFor == 1 && name == "TIME" {
					tParam2, _ = time.Parse("2006-01-02T15:04:05", param2)
					tParam1 = time.Now().UTC().Add(2 * time.Hour)
					log.Printf("WHILE LOOP: %s (%s) %s %s\n", tParam1, name, operator, tParam2)
				} else if getDataFor == 2 && name == "TIME" {
					tParam1, _ = time.Parse("2006-01-02T15:04:05", param1)
					tParam2 = time.Now().UTC().Add(2 * time.Hour)
					log.Printf("WHILE LOOP: %s (%s) %s %s\n", tParam1, name, operator, tParam2)
				} else {
					lastValue, err := GetLastValue(name, session_token)
					if err != nil {
						return fmt.Errorf("error getting last value: %s", err)
					}
					if getDataFor == 1 {
						param1 = strconv.FormatFloat(lastValue, 'f', -1, 64)
						log.Printf("WHILE LOOP: %s (%s) %s %s\n", param1, name, operator, param2)
					} else if getDataFor == 2 {
						param2 = strconv.FormatFloat(lastValue, 'f', -1, 64)
						log.Printf("WHILE LOOP: %s (%s) %s %s\n", param1, name, operator, param2)
					}
				}
			}
			nParam1, _ := strconv.ParseFloat(param1, 32)
			nParam2, _ := strconv.ParseFloat(param2, 32)
			switch operator {
			case "==":
				if nParam1 == nParam2 || tParam1 == tParam2 {
					log.Printf("%sIS TRUE%s", models.Green, models.Reset)
					err := WhileCommandExecuter(statements, widgetId, session_token)
					if err != nil {
						return fmt.Errorf("error executing command: %s", err)
					}
				} else {
					log.Printf("%sIS FALSE%s", models.Red, models.Reset)
					return nil
				}
			case "!=":
				if nParam1 != nParam2 || tParam1 != tParam2 {
					log.Printf("%sIS TRUE%s", models.Green, models.Reset)
					err := WhileCommandExecuter(statements, widgetId, session_token)
					if err != nil {
						return fmt.Errorf("error executing command: %s", err)
					}
				} else {
					log.Printf("%sIS FALSE%s", models.Red, models.Reset)
					return nil
				}
			case ">":
				if nParam1 > nParam2 || tParam1.After(tParam2) {
					log.Printf("%sIS TRUE%s", models.Green, models.Reset)
					err := WhileCommandExecuter(statements, widgetId, session_token)
					if err != nil {
						return fmt.Errorf("error executing command: %s", err)
					}
				} else {
					log.Printf("%sIS FALSE%s", models.Red, models.Reset)
					return nil
				}
			case "<":
				if nParam1 < nParam2 || tParam1.Before(tParam2) {
					log.Printf("%sIS TRUE%s", models.Green, models.Reset)
					err := WhileCommandExecuter(statements, widgetId, session_token)
					if err != nil {
						return fmt.Errorf("error executing command: %s", err)
					}
				} else {
					log.Printf("%sIS FALSE%s", models.Red, models.Reset)
					return nil
				}
			case ">=":
				if nParam1 >= nParam2 || tParam1.After(tParam2) || tParam1 == tParam2 {
					log.Printf("%sIS TRUE%s", models.Green, models.Reset)
					err := WhileCommandExecuter(statements, widgetId, session_token)
					if err != nil {
						return fmt.Errorf("error executing command: %s", err)
					}
				} else {
					log.Printf("%sIS FALSE%s", models.Red, models.Reset)
					return nil
				}
			case "<=":
				if nParam1 <= nParam2 || tParam1.Before(tParam2) || tParam1 == tParam2 {
					log.Printf("%sIS TRUE%s", models.Green, models.Reset)
					err := WhileCommandExecuter(statements, widgetId, session_token)
					if err != nil {
						return fmt.Errorf("error executing command: %s", err)
					}
				} else {
					log.Printf("%sIS FALSE%s", models.Red, models.Reset)
					return nil
				}
			}
			if (wgWhile != sync.WaitGroup{}) {
				wgWhile.Done()
			}
		}
	}()

	return nil
}

func WhileCommandExecuter(statements []string, widgetId string, session_token string) error {
	for _, statement := range statements {
		wg.Add(1)
		err := ExecuteCommand(statement, widgetId, session_token)
		wg.Wait()
		if err != nil {
			return fmt.Errorf("error executing command: %s", err)
		}
	}
	return nil
}

func GetLastValue(name string, session_token string) (float64, error) {
	//get current time
	currentTime := time.Now()
	//get time + 1 minute
	nextMinute := currentTime.Add(-time.Hour*2 - time.Minute*5)
	//format time
	nextMinuteFormated := nextMinute.Format("2006-01-02T15:04:05.000Z")
	currentTimeFormated := currentTime.Format("2006-01-02T15:04:05.000Z")

	//pack the data from the query in a struct
	queryStr := fmt.Sprintf("from(bucket: \"lcn\")\n"+
		"|> range(start: %s, stop: %s)\n"+
		"|> filter(fn: (r) => r[\"_measurement\"] == \"lcn\")\n"+
		"|> filter(fn: (r) => r[\"name\"] == \"%s\")", nextMinuteFormated, currentTimeFormated, name)

	queraRes, err := query.QueryInfluxDB(session_token, queryStr)
	if err != nil {
		return 0.0, fmt.Errorf("error querying influxdb: %s", err)
	}
	//get last value
	return queraRes.LineSets[0][len(queraRes.LineSets[0])-1].Value, nil
}

func StartScript(widgetId string, session_token string) error {
	//set scriptState
	err := SetScriptState(widgetId, "1")
	if err != nil {
		return fmt.Errorf("error setting scriptState: %s", err)
	}

	//get script
	script, err := DBGetScript(widgetId)

	if err != nil {
		return fmt.Errorf("error getting script: %s", err)
	}

	//execute script
	go func() {
		err := <-ScriptBus(widgetId, script, session_token)
		if err != nil {
			fmt.Println(err)
		}
		wgWhile = sync.WaitGroup{}
		err = StopScript(widgetId)
		if err != nil {
			fmt.Println(err)
		}
	}()
	return nil
}

func StopScript(widgetId string) error {
	wgWhile.Wait()

	//TODO: make func
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return fmt.Errorf("error opening db: %s", err)
	}
	defer db.Close()

	//set current command
	_, err = db.Exec("UPDATE schalter SET currentCommand = ? WHERE widgetId = ?", nil, widgetId)
	if err != nil {
		return fmt.Errorf("error setting command: %s", err)
	}

	//set scriptState
	err = SetScriptState(widgetId, "0")
	if err != nil {
		return fmt.Errorf("error setting scriptState: %s", err)
	}
	wgWhile = sync.WaitGroup{}
	return nil
}

func ScriptBus(widgetId string, script string, session_token string) <-chan error {
	//create channel
	r := make(chan error)
	go func() {
		for _, command := range strings.Split(script, "\n") {
			err := ExecuteCommand(command, widgetId, session_token)
			if err != nil {
				r <- fmt.Errorf("error executing command: %s", err)
				return
			}
		}
		r <- nil
	}()
	return r
}

func GetScriptState(widgetId string) (string, error) {
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return "", fmt.Errorf("error opening db: %s", err)
	}
	defer db.Close()

	//get scriptState
	var scriptState string
	err = db.QueryRow("SELECT scriptState FROM schalter WHERE widgetId = ?", widgetId).Scan(&scriptState)
	if err != nil {
		return "", fmt.Errorf("error getting scriptState: %s", err)
	}

	return scriptState, nil
}

func SetScriptState(widgetId string, scriptState string) error {
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return fmt.Errorf("error opening db: %s", err)
	}
	defer db.Close()

	//set scriptState
	_, err = db.Exec("UPDATE schalter SET scriptState = ? WHERE widgetId = ?", scriptState, widgetId)
	if err != nil {
		return fmt.Errorf("error setting scriptState: %s", err)
	}

	return nil
}

func StartAllDBStartedScripts(session_token string) error {
	//db connection
	db, err := tools.DBConnection()
	if err != nil {
		return fmt.Errorf("error opening db: %s", err)
	}
	defer db.Close()

	//get all started scripts
	rows, err := db.Query("SELECT widgetId FROM schalter WHERE scriptState = 1")
	if err != nil {
		return fmt.Errorf("error getting all started scripts: %s", err)
	}
	defer rows.Close()

	//start all scripts
	for rows.Next() {
		var widgetId string
		err := rows.Scan(&widgetId)
		if err != nil {
			return fmt.Errorf("error scanning widgetId: %s", err)
		}
		err = StartScript(widgetId, session_token)
		if err != nil {
			return fmt.Errorf("error starting script: %s", err)
		}
	}

	return nil
}
