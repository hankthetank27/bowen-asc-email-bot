package main

import (
    "email_service/controllers"
    "errors"
    "fmt"
    "context"
    "io"
    "log"
    "os"
    "net/http"

    "github.com/joho/godotenv"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

const PORT = 3000

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file. Err: %s\n", err)
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
			fmt.Printf("Listening on port %d\n", PORT)
			return fmt.Sprintf(":%d", PORT)
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
				w.WriteHeader(http.StatusInternalServerError)
				io.WriteString(w, err.Error())
				return
			}
        }
        fmt.Println("Successfully processed and logged request")
	})
}
