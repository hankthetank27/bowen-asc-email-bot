package main

import (
	"context"
	"email_service/controllers"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {

	godotenv.Load()
	PORT := os.Getenv("$PORT")
	if PORT == "" {
		fmt.Println("Manually assigning port...")
		PORT = "3000"
	}

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		log.Fatal("You must set your 'MONGODB_URI' environment variable.")
	}
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	http.HandleFunc("/newOrder", ChainMiddleware(
		func(
			w http.ResponseWriter,
			r *http.Request,
			locals *controllers.NewOrderLocals,
		) error {
			locals.OrdersDB = client.Database("bowen-asc-email-bot").Collection("orders")
			return nil
		},
		controllers.ValidateSqSpaceOrder,
		controllers.HandleEmailRequest,
	))

	err = http.ListenAndServe(
		func() string {
			fmt.Printf("Listening on port %s\n", PORT)
			return fmt.Sprintf(":%s", PORT)
		}(),
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
		*controllers.NewOrderLocals,
	) error,
) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locals := controllers.NewOrderLocals{}
		for _, middleware := range middlewares {
			err := middleware(w, r, &locals)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, err.Error())
				return
			}
		}
		fmt.Println("Successfully processed and logged request")
	})
}
