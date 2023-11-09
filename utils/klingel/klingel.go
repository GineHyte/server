package klingel

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	models "github.com/GineHyte/server/models"
	mailjet "github.com/mailjet/mailjet-apiv3-go"
)

var klingelCh = make(chan string)

func Mail(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// get arguments from body
		decoder := json.NewDecoder(r.Body)
		var t map[string]interface{}
		err := decoder.Decode(&t)
		if err != nil {
			log.Printf(models.Red+"error decoding json: %s\n"+models.Reset, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "error decoding json"})
			return
		}
		from := t["from"].(string)
		to := t["to"].(string)
		subject := t["subject"].(string)
		text := t["text"].(string)
		html := t["html"].(string)

		// send mail
		mailjetClient := mailjet.NewMailjetClient(os.Getenv("MAILJET_API_PUBLIC"), os.Getenv("MAILJET_API_PRIVATE"))
		messagesInfo := []mailjet.InfoMessagesV31{
			{
				From: &mailjet.RecipientV31{
					Email: from,
					Name:  "Klingel",
				},
				To: &mailjet.RecipientsV31{
					mailjet.RecipientV31{
						Email: to,
						Name:  "info",
					},
				},
				Subject:  subject,
				TextPart: text,
				HTMLPart: html,
			},
		}
		messages := mailjet.MessagesV31{Info: messagesInfo}
		res, err := mailjetClient.SendMailV31(&messages)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Data: %+v\n", res)

		// send response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	default:
		log.Printf(models.Red + "Sorry, only POST method is supported.\n" + r.Method + models.Reset)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Sorry, only POST method is supported."})
	}
}

func Klingel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		klingelCh <- "klingel"
		klingelCh <- "klingel"
		// send response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	default:
		log.Printf(models.Red + "Sorry, only GET method is supported.\n" + r.Method + models.Reset)
		w.WriteHeader(http.StatusBadRequest)
	}

}

func handleLogs(ctx context.Context) {
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		default:
			klingelCh <- "klingel"
			time.Sleep(1 * time.Second)
		}
	}

	close(klingelCh)
}

func formatServerSentEvent(event string, data any) (string, error) {
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("event: %s\n", event))
	sb.WriteString(fmt.Sprintf("data: %v\n\n", data))

	return sb.String(), nil
}

func Events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	for klingel := range klingelCh {
		event, err := formatServerSentEvent("message", klingel)
		if err != nil {
			fmt.Println(err)
			break
		}

		log.Println("sending event: ", event)
		_, err = fmt.Fprint(w, event)
		if err != nil {
			fmt.Println(err)
			break
		}

		flusher.Flush()
	}
}
