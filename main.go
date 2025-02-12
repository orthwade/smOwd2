package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"smOwd2/logs"
	"smOwd2/shikirequests"
)

func handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code, err := shikirequests.HandleAuthCode(w, r)

	if err != nil {
		log.Printf("Error handling OAuth callback: %v", err)
		return
	}

	// Log the authorization code for now
	log.Printf("Authorization Code: %s", code)
}

func main() {
	logger := logs.NewDefault()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, "logger", logger)
	ctx = context.WithValue(ctx, "cancelFunc", cancel)

	http.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		handleOAuthCallback(w, r)
	})

	fmt.Println("Starting server on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
