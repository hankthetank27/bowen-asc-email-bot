package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/smtp"
	"os"
	"strings"

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


func main() {
	recipients := []string{"hjackson277@gmail.com"}
	err := sendEmail(recipients)

	if err != nil {
		log.Fatalf("Could not send mail. Err: %s", err)
	}

	fmt.Printf(
		"Email successfully sent to recipients: %s\n",
		strings.Join(recipients, ", "),
	)
}

func sendEmail(recipients []string) error {
	err := godotenv.Load()

	if err != nil {
		log.Fatalf("Error loading .env file. Err: %s", err)
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
        log.Fatalf("Failed to parse email template. Err: %s", err)
	}

	err = tmpl.Execute(
        body, 
        EmailTemplate{Recipient: recipients[0]},
    )

	if err != nil {
        log.Fatalf("Failed to execute email template. Err: %s", err)
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
        log.Fatalf("Failed to send email. Err: %s", err)
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
