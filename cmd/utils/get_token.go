package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

func main() {
	config := &oauth2.Config{
		ClientID:     "",
		ClientSecret: "",
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8090/oauth2callback",
		Scopes:       []string{gmail.GmailReadonlyScope},
	}

	// Create a random state
	state := "random-state"

	// Start an HTTP server to handle the OAuth callback
	http.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		// Check state parameter
		if r.URL.Query().Get("state") != state {
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Exchange the authorization code for a token
		code := r.URL.Query().Get("code")
		token, err := config.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to exchange code: %v", err), http.StatusInternalServerError)
			return
		}

		// Print the refresh token
		fmt.Printf("\nRefresh Token: %s\n\n", token.RefreshToken)

		// Respond to the user
		fmt.Fprintf(w, "Authentication successful! You can close this window.")
		os.Exit(0)
	})

	// Generate the authorization URL
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Open this URL in your browser:\n%s\n", authURL)

	log.Fatal(http.ListenAndServe(":8090", nil))
}
