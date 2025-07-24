// internal/infrastructure/config/config.go
package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	// App
	AppVersion string

	// Server
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// MongoDB
	MongoURI      string
	MongoDB       string
	MongoUser     string
	MongoPassword string

	// Gmail
	GmailClientID     string
	GmailClientSecret string
	GmailRefreshToken string
	GmailPollInterval time.Duration

	// WhatsApp
	WhatsAppEndpoint string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	godotenv.Load()

	// Set defaults and override with env vars
	config := &Config{
		AppVersion:   getEnv("APP_VERSION", "1.0.0"),
		Port:         getEnv("PORT", "8080"),
		ReadTimeout:  time.Duration(getEnvAsInt("READ_TIMEOUT", 30)) * time.Second,
		WriteTimeout: time.Duration(getEnvAsInt("WRITE_TIMEOUT", 30)) * time.Second,

		MongoURI:      getEnv("MONGODB_DSN", "mongodb://localhost:27017"),
		MongoDB:       getEnv("MONGO_DB", "daisi"),
		MongoUser:     getEnv("MONGO_USER", ""),
		MongoPassword: getEnv("MONGO_PASSWORD", ""),

		GmailClientID:     getEnv("GMAIL_CLIENT_ID", ""),
		GmailClientSecret: getEnv("GMAIL_CLIENT_SECRET", ""),
		GmailRefreshToken: getEnv("GMAIL_REFRESH_TOKEN", ""),
		GmailPollInterval: time.Duration(getEnvAsInt("GMAIL_POLL_INTERVAL", 60)) * time.Second,

		WhatsAppEndpoint: getEnv("WHATSAPP_ENDPOINT", ""),
	}

	return config, nil
}

// Helper functions to get environment variables
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
