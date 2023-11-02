package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"
)

func HandleEmailRequest(
	w http.ResponseWriter,
	r *http.Request,
	locals *NewOrderLocals,
) error {

	if len(locals.Purchases) == 0 {
		errMsg := "Order has no valid purchases"
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, errMsg)
		return errors.New(errMsg)
	}

	sentEmailCounter := 0

	for _, purchase := range locals.Purchases {

		recipients, err := produceRecipients(purchase)

		if err != nil {
			log.Printf(
				"No valid recipients for purchase of product %s\n",
				purchase.PurchaseType,
			)
			continue
		}

		err = sendEmail(recipients)

		if err != nil {
			log.Printf("Error sending mail. Err: %s\n", err)
			continue
		}

		successMsg := fmt.Sprintf(
			"Email successfully sent to recipients %s for purchase of %s\n",
			strings.Join(recipients, ", "),
			purchase.PurchaseType,
		)
		fmt.Print(successMsg)
		sentEmailCounter++
	}

	if sentEmailCounter == 0 {
		errMsg := "Failed to notify all recipients for purchases."
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, errMsg)
		return errors.New(errMsg)
	} else if sentEmailCounter == len(locals.Purchases) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Successfully notifed recipients for all purchases.")
		return nil
	} else {
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, "Partialy failed. Could not notify all recipients for purchases.")
		return nil
	}
}

func ValidateSqSpaceOrder(
	w http.ResponseWriter,
	r *http.Request,
	locals *NewOrderLocals,
) error {

	hasOrderId := r.URL.Query().Has("orderId")
	hasCustomerEmail := r.URL.Query().Has("customerEmailAddress")

	if !hasOrderId || !hasCustomerEmail {
		errMsg := "No order specified\n"
		log.Print(errMsg)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, errMsg)
		return errors.New(errMsg)
	}

	SqSpaceAPIKey := os.Getenv("SQSPACE_API_KEY")
	reqUrl := "https://api.squarespace.com/1.0/commerce/orders/"

	client := &http.Client{}

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", SqSpaceAPIKey))
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making request to Squarespace:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		orderId := r.URL.Query().Get("orderId")
		customerEmailAddress := r.URL.Query().Get("customerEmailAddress")

		var data SqSpaceOrders
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&data); err != nil {
			log.Println("Error parsing JSON:", err)
			return err
		}

		var order *SqSpaceOrder
		for _, res := range data.Orders {
			if res.CustomerEmail == customerEmailAddress &&
				res.OrderNumber == orderId {
				order = &res
				break
			}
		}

		if order == nil {
			errMsg := "Invalid order"
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, errMsg)
			return errors.New(errMsg)
		}

		locals.OrderId = order.Id
		locals.CustomerInfo.FirstName = order.BillingAddress.FirstName
		locals.CustomerInfo.LastName = order.BillingAddress.LastName
		locals.CustomerInfo.Email = order.CustomerEmail
		for _, purchase := range order.LineItems {
			item := Purchase{
				PurchaseType: purchase.ProductName,
				ProductSKU:   purchase.ProductSKU,
			}
			if purchase.Customizations != nil {
				for _, c := range *purchase.Customizations {
					if c.Label == "Subject Property Address" {
						item.SubjectAddress = c.Value
						break
					}
				}
			}
			locals.Purchases = append(locals.Purchases, item)
		}
	} else {
		errMsg := fmt.Sprintf(
			"Request failed with status code %d\n",
			resp.StatusCode,
		)
		log.Println(errMsg)
		return errors.New(errMsg)
	}
	return err
}

func sendEmail(recipients []string) error {

	senderEmail := os.Getenv("SENDER_EMAIL")
	senderPassword := os.Getenv("SENDER_PASSWORD")

	if senderEmail == "" || senderPassword == "" {
		errMsg := "Sender email or password does not exist"
		log.Println(errMsg)
		return errors.New(errMsg)
	}

	auth := smtp.PlainAuth(
		"",
		senderEmail,
		senderPassword,
		"smtp.gmail.com",
	)

	body := new(bytes.Buffer)
	tmpl, err := template.ParseFiles("./templates/new-order.html")

	if err != nil {
		log.Printf("Failed to parse email template. Err: %s\n", err)
		return err
	}

	err = tmpl.Execute(
		body,
		EmailTemplate{Recipient: "noob"},
	)

	if err != nil {
		log.Printf("Failed to execute email template. Err: %s\n", err)
		return err
	}

	request := Email{
		Sender:     senderEmail,
		Recipients: recipients,
		Subject:    "New order!",
		Body:       body.String(),
	}

	msg := buildMessage(request)

	err = smtp.SendMail(
		"smtp.gmail.com:587",
		auth,
		senderEmail,
		recipients,
		[]byte(msg),
	)

	if err != nil {
		log.Printf("Failed to send email. Err: %s\n", err)
	}

	return err
}

func buildMessage(message Email) string {
	msg := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n"
	msg += fmt.Sprintf("From: %s\r\n", message.Sender)
	msg += fmt.Sprintf("To: %s\r\n", strings.Join(message.Recipients, ";"))
	msg += fmt.Sprintf("Subject: %s\r\n", message.Subject)
	msg += fmt.Sprintf("\r\n%s\r\n", message.Body)
	return msg
}

func produceRecipients(purchase Purchase) ([]string, error) {
	inEdRecp := os.Getenv("IN_ED_RECP")
	outsideEdRecp := os.Getenv("OUTSIDE_ED_RECP")

	if inEdRecp == "" || outsideEdRecp == "" {
		errMsg := "Env variables for recipients do not exist"
		log.Println(errMsg)
		return nil, errors.New(errMsg)
	}

	// Residential Property Appraisal In Edmonton
	if purchase.ProductSKU == "SQ5929745" {
		return []string{inEdRecp}, nil
	}

	// Residential Property Appraisal Outside Edmonton
	if purchase.ProductSKU == "SQ8618609" {
		return []string{outsideEdRecp}, nil
	}

	return []string{}, errors.New("No valid recipients found")
}
