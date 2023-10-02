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
	tools "github.com/GineHyte/server/utils/tools"

	auth "github.com/GineHyte/server/utils/auth"
	query "github.com/GineHyte/server/utils/query"
	register "github.com/GineHyte/server/utils/register"
	schalter "github.com/GineHyte/server/utils/schaltercontrol"
)

/* main */
func main() {
	//load .env file
	godotenv.Load(".env")

	//clear terminal
	fmt.Print("\033[H\033[J")

	go schalter.SchalterEventStream()
	go tools.SchalterDbTimer()

	//check if test mode
	/*if len(os.Args) > 1 {
		var test = os.Args[1] == "test"
		fmt.Printf(models.Blue + "TEST MODE ACTIVE\n" + models.Reset + "")
	}*/

	http.HandleFunc("/Register", register.Register)
	http.HandleFunc("/Auth", auth.Auth)
	http.HandleFunc("/Query", query.Query)
	http.HandleFunc("/Schalter", schalter.SchalterControl)

	err := http.ListenAndServe(":3333", nil)

	if errors.Is(err, http.ErrServerClosed) {
		log.Printf(models.Green + "server closed\n" + models.Reset)
	} else if err != nil {
		log.Printf(models.Red+"error starting server: %s\n", err)
		fmt.Printf(models.Reset)
		os.Exit(1)
	}
}
