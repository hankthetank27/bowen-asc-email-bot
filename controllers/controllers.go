package controllers

import (
	"bytes"
	"context"
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

	"go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
)


func ValidateSqSpaceOrder(
	w http.ResponseWriter,
	r *http.Request,
	locals *NewOrderLocals,
) error {

	genericErr := errors.New("Error validating order")

	hasOrderNum := r.URL.Query().Has("orderId")
	hasCustomerEmail := r.URL.Query().Has("customerEmailAddress")

	if !hasOrderNum || !hasCustomerEmail {
		errMsg := "No order specified\n"
		log.Print(errMsg)
		return errors.New(errMsg)
	}

	orderNum := r.URL.Query().Get("orderId")
	customerEmailAddress := r.URL.Query().Get("customerEmailAddress")
	SqSpaceAPIKey := os.Getenv("SQSPACE_API_KEY")
	reqUrl := "https://api.squarespace.com/1.0/commerce/orders/"

	client := &http.Client{}

	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return genericErr
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", SqSpaceAPIKey))
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making request to Squarespace:", err)
		return genericErr
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {

		var data SqSpaceOrders
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&data); err != nil {
			log.Println("Error parsing JSON:", err)
			return genericErr
		}

		var order *SqSpaceOrder
		for _, res := range data.Orders {
			if res.CustomerEmail == customerEmailAddress &&
				res.OrderNumber == orderNum {
				order = &res
				break
			}
		}

		if order == nil {
			errMsg := "Invalid order"
			fmt.Println(errMsg)
			return errors.New(errMsg)
		}

		var result bson.M
		err = locals.OrdersDB.FindOne(
            context.TODO(), 
            bson.D{ {"order-id", order.Id} },
        ).Decode(&result)
        
        if err != mongo.ErrNoDocuments {
            errMsg := "Order entry already proccessed"
            log.Println(errMsg + ": " + order.Id)
            return errors.New(errMsg)
        }

		locals.OrderNumber = orderNum
		locals.OrderId = order.Id
		locals.CustomerInfo.FirstName = order.BillingAddress.FirstName
		locals.CustomerInfo.LastName = order.BillingAddress.LastName
		locals.CustomerInfo.Phone = order.BillingAddress.Phone
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
			"Request to Squarespace failed with status code %d\n",
			resp.StatusCode,
		)
		log.Println(errMsg)
		return genericErr
	}
	return nil
}


func HandleEmailRequest(
	w http.ResponseWriter,
	r *http.Request,
	locals *NewOrderLocals,
) error {

	if len(locals.Purchases) == 0 {
		errMsg := "Order has no valid purchases"
		log.Println(errMsg)
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

		err = sendEmail(
			recipients,
			locals,
			purchase,
		)

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
		return errors.New("Failed to notify all recipients for purchases.")
	} else if sentEmailCounter == len(locals.Purchases) {
        if err := registerOrderProcessed(locals); err != nil {
            errMsg := "Recipients emailed but could not log order."
            log.Println(errMsg)
            return errors.New(errMsg)
        }
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "Successfully notifed recipients for all purchases.")
		return nil
	} else {
        if err := registerOrderProcessed(locals); err != nil {
            errMsg := "Recipients partialy emailed but could not log order."
            log.Println(errMsg)
            return errors.New(errMsg)
        }
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, "Partialy failed. Could not notify all recipients for purchases.")
		return nil
	}
}


func registerOrderProcessed(
	locals *NewOrderLocals,
) error {
    _, err := locals.OrdersDB.InsertOne(
            context.TODO(),
            bson.D{ {"order-id", locals.OrderId} },
        )
    return err
}


func sendEmail(
	recipients []string,
	locals *NewOrderLocals,
	purchase Purchase,
) error {

	senderEmail := os.Getenv("SENDER_EMAIL")
	senderPassword := os.Getenv("SENDER_PASSWORD")
	smtpServer := os.Getenv("SMTP_SERVER")

	if senderEmail == "" || senderPassword == "" {
		errMsg := "Sender email or password does not exist"
		log.Println(errMsg)
		return errors.New(errMsg)
	}

	if smtpServer == "" {
		errMsg := "STMP server name does not exist"
		log.Println(errMsg)
		return errors.New(errMsg)
	}

	auth := LoginAuth(senderEmail, senderPassword)
	body := new(bytes.Buffer)
	tmpl, err := template.ParseFiles("./templates/new-order.html")

	if err != nil {
		log.Printf("Failed to parse email template. Err: %s\n", err)
		return err
	}

	err = tmpl.Execute(
		body,
		EmailTemplate{
			Locals:   locals,
			Purchase: purchase,
		},
	)

	if err != nil {
		log.Printf("Failed to execute email template. Err: %s\n", err)
		return err
	}

	request := Email{
		Sender:     senderEmail,
		Recipients: recipients,
		Subject:    fmt.Sprintf("New Order: %s\n", purchase.PurchaseType),
		Body:       body.String(),
	}

	msg := buildMessage(request)

	err = smtp.SendMail(
		fmt.Sprintf("%s:587", smtpServer),
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


type loginAuth struct {
	username, password string
}

func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}


func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}


func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, errors.New("Unkown fromServer")
		}
	}
	return nil, nil
}
