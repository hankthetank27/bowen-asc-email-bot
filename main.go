package main

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

type NewOrderLocals struct {
	OrderId      string
	CustomerInfo struct {
		FirstName string
		LastName  string
		Email     string
	}
	Purchases []Purchase
}

type Purchase struct {
	ProductID      string
	PurchaseType   string
	SubjectAddress string
}

type SqSpaceOrders struct {
	Result []struct {
		Id             string `json:"id"`
		CustomerEmail  string `json:"customerEmail"`
		OrderNumber    string `json:"orderNumber"`
		BillingAddress struct {
			FirstName   string `json:"firstName"`
			LastName    string `json:"lastName"`
			Address1    string `json:"address1"`
			Address2    string `json:"address2"`
			City        string `json:"city"`
			State       string `json:"state"`
			CountryCode string `json:"countryCode"`
			PostalCode  string `json:"postalCode"`
			Phone       string `json:"phone"`
		} `json:"billingAddress"`
		LineItems []struct {
			ProductID      string `json:"productId"`
			ProductName    string `json:"productName"`
			Customizations *[]struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"customizations"`
		} `json:"LineItems"`
	} `json:"result"`
}

const PORT = 3000

func main() {

	err := godotenv.Load()

	if err != nil {
		log.Fatalf("Error loading .env file. Err: %s\n", err)
	}

	http.HandleFunc("/newOrder", ChainMiddleware(
		validatedSqSpaceOrder,
		handleEmailRequest,
	))

	err = http.ListenAndServe(
		fmt.Sprintf(":%d", PORT),
		nil,
	)

	if !errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("Server closed\n")
	} else if err != nil {
		log.Fatalf("error running http server: %s\n", err)
	}
}

func ChainMiddleware(
	middlewares ...func(
		http.ResponseWriter,
		*http.Request,
		*NewOrderLocals,
	) error,
) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locals := NewOrderLocals{}
		for _, middleware := range middlewares {
			err := middleware(w, r, &locals)
			if err != nil {
				return
			}
		}
	})
}

func handleEmailRequest(
	w http.ResponseWriter,
	r *http.Request,
	locals *NewOrderLocals,
) error {

	fmt.Println(locals)

	recipients := strings.Split("hjackson277@gmail.com", ",")

	err := sendEmail(recipients)

	if err != nil {
		log.Printf("Error sending mail. Err: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "Unable to send mail\n")
		return err
	}

	successMsg := fmt.Sprintf(
		"Email successfully sent to recipients: %s\n",
		strings.Join(recipients, ", "),
	)
	fmt.Print(successMsg)
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, successMsg)
	return err
}

func validatedSqSpaceOrder(
	w http.ResponseWriter,
	r *http.Request,
	locals *NewOrderLocals,
) error {

	hasOrderId := r.URL.Query().Has("orderId")
	hasCustomerEmail := r.URL.Query().Has("customerEmailAddress")

	if !hasOrderId || !hasCustomerEmail {
		errMsg := "No order specified\n"
		log.Print(errMsg)
		w.WriteHeader(http.StatusPreconditionFailed)
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

		for _, result := range data.Result {
			if result.CustomerEmail == customerEmailAddress && result.OrderNumber == orderId {
				locals.OrderId = result.Id
				locals.CustomerInfo.FirstName = result.BillingAddress.FirstName
				locals.CustomerInfo.LastName = result.BillingAddress.LastName
				locals.CustomerInfo.Email = result.CustomerEmail
				for _, purchase := range result.LineItems {
					if purchase.Customizations != nil {
						for _, c := range *purchase.Customizations {
							if c.Label == "Subject Property Address" {
								item := Purchase{
									PurchaseType:   purchase.ProductName,
									ProductID:      purchase.ProductID,
									SubjectAddress: c.Value,
								}
								locals.Purchases = append(locals.Purchases, item)
								break
							}
						}
					}

				}
				return err
			}
		}
	} else {
		errMsg := fmt.Sprintf("Request failed with status code %d\n", resp.StatusCode)
		log.Println(errMsg)
		return errors.New(errMsg)
	}

	return err
}

func sendEmail(recipients []string) error {

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
		log.Printf("Failed to send email. Err: %s\n", err)
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
