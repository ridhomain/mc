package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"mailcast-service-v2/pkg/logger"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

// GmailOAuth handles OAuth authentication with Gmail
type GmailOAuth struct {
	config       *oauth2.Config
	refreshToken string
	logger       logger.Logger
}

// NewGmailOAuth creates a new Gmail OAuth handler
func NewGmailOAuth(clientID, clientSecret, refreshToken string, logger logger.Logger) *GmailOAuth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{gmail.GmailReadonlyScope},
	}

	return &GmailOAuth{
		config:       config,
		refreshToken: refreshToken,
		logger:       logger,
	}
}

// GetTokenSource returns a token source that can be used with Gmail API
func (o *GmailOAuth) GetTokenSource(ctx context.Context) oauth2.TokenSource {
	token := &oauth2.Token{
		RefreshToken: o.refreshToken,
		Expiry:       time.Now(), // Force refresh
	}

	return o.config.TokenSource(ctx, token)
}

// GenerateAuthURL generates a URL for the user to authorize the application
func (o *GmailOAuth) GenerateAuthURL() string {
	return o.config.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeCode exchanges an authorization code for a token
func (o *GmailOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := o.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Log the refresh token (for initial setup)
	o.logger.Info("Refresh token obtained", "token", token.RefreshToken)

	return token, nil
}

// TokenToJSON converts a token to JSON
func (o *GmailOAuth) TokenToJSON(token *oauth2.Token) (string, error) {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
