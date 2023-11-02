package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
    "email_service/controllers"

	"github.com/joho/godotenv"
)

const PORT = 3000

func main() {

	err := godotenv.Load()

	if err != nil {
		log.Fatalf("Error loading .env file. Err: %s\n", err)
	}

	http.HandleFunc("/newOrder", ChainMiddleware(
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
				return
			}
		}
	})
}

