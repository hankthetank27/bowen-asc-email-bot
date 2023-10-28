package main

import (
    "os"
    "io"
    "fmt"
    "log"
	"bytes"
	"errors"
    "strings"
	"net/http"
	"net/smtp"
    "html/template"

	"github.com/joho/godotenv"
)

type Message struct {
	Sender     string
	Recipients []string
	Subject    string
	Body       string
}

type EmailTemplate struct {
	Recipient string
}

const PORT = 3000

func main() {

	http.HandleFunc("/email", handleEmailRequest)

	err := http.ListenAndServe(
		fmt.Sprintf(":%d", PORT),
		nil,
	)

	if !errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("Server closed\n")
	} else if err != nil {
		log.Fatalf("error running http server: %s\n", err)
	}
}

func handleEmailRequest(w http.ResponseWriter, r *http.Request) {

	hasRecipients := r.URL.Query().Has("recipients")

	if !hasRecipients {
        errMsg := "No recipients specified\n"
        log.Print(errMsg)
        w.WriteHeader(http.StatusPreconditionFailed)
		io.WriteString(w, errMsg)
		return
	}

	recipients := strings.Split(
		r.URL.Query().Get("recipients"), ",",
	)

	err := sendEmail(recipients)

	if err != nil {
		log.Printf("Error sending mail. Err: %s", err)
        w.WriteHeader(http.StatusInternalServerError)
        io.WriteString(w, "Unable to send mail\n")
        return
	}

    successMsg := fmt.Sprintf(
        "Email successfully sent to recipients: %s\n",
		strings.Join(recipients, ", "),
    )
    fmt.Print(successMsg)
    w.WriteHeader(http.StatusOK)
	io.WriteString(w, successMsg)
}

func sendEmail(recipients []string) error {
	err := godotenv.Load()

	if err != nil {
		log.Fatalf("Error loading .env file. Err: %s", err)
        return err
	}

	senderEmail := os.Getenv("SENDER_EMAIL")
	senderPassword := os.Getenv("SENDER_PASSWORD")

	auth := smtp.PlainAuth(
		"",
		senderEmail,
		senderPassword,
		"smtp.gmail.com",
	)

	body := new(bytes.Buffer)
	tmpl, err := template.ParseFiles("./templates/new-order.html")

	if err != nil {
		log.Printf("Failed to parse email template. Err: %s", err)
        return err
	}

	err = tmpl.Execute(
		body,
		EmailTemplate{Recipient: "noob"},
	)

	if err != nil {
		log.Printf("Failed to execute email template. Err: %s", err)
        return err
	}

	request := Message{
		Sender:     senderEmail,
		Recipients: recipients,
		Subject:    "New order!",
		Body:       body.String(),
	}

	msg := BuildMessage(request)

	err = smtp.SendMail(
		"smtp.gmail.com:587",
		auth,
		senderEmail,
		recipients,
		[]byte(msg),
	)

	if err != nil {
		log.Printf("Failed to send email. Err: %s", err)
	}

    return err
}

func BuildMessage(message Message) string {
	msg := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n"
	msg += fmt.Sprintf("From: %s\r\n", message.Sender)
	msg += fmt.Sprintf("To: %s\r\n", strings.Join(message.Recipients, ";"))
	msg += fmt.Sprintf("Subject: %s\r\n", message.Subject)
	msg += fmt.Sprintf("\r\n%s\r\n", message.Body)

	return msg
}
