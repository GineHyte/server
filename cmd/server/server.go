package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	models "github.com/GineHyte/server/models"
	auth "github.com/GineHyte/server/utils/auth"
	klingel "github.com/GineHyte/server/utils/klingel"
	query "github.com/GineHyte/server/utils/query"
	register "github.com/GineHyte/server/utils/register"
	schalter "github.com/GineHyte/server/utils/schalter"
	scripter "github.com/GineHyte/server/utils/scripter"
	tools "github.com/GineHyte/server/utils/tools"
)

/* main */
func main() {
	//load .env file
	godotenv.Load(".env")

	//clear terminal
	fmt.Print("\033[H\033[J")

	go schalter.SchalterEventStream()
	go tools.SchalterDbTimer()

	http.HandleFunc("/register", register.Register)
	http.HandleFunc("/auth", auth.Auth)
	http.HandleFunc("/query", query.Query)
	http.HandleFunc("/schalter", schalter.SchalterControl)
	http.HandleFunc("/script", scripter.Script)
	http.HandleFunc("/control_script", scripter.ControlScript)

	http.HandleFunc("/mail", klingel.Mail)
	http.HandleFunc("/klingel", klingel.Klingel)

	http.HandleFunc("/klingel_events", klingel.Events)

	err := http.ListenAndServe(":3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		log.Printf(models.Green + "server closed\n" + models.Reset)
	} else if err != nil {
		log.Printf(models.Red+"error starting server: %s\n", err)
		fmt.Printf(models.Reset)
		os.Exit(1)
	}
}
