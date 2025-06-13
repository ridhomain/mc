package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
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

	PostgresURI      string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	godotenv.Load()

	// Set defaults and override with env vars
	config := &Config{
		Port:         getEnv("PORT", "8080"),
		ReadTimeout:  time.Duration(getEnvAsInt("READ_TIMEOUT", 30)) * time.Second,
		WriteTimeout: time.Duration(getEnvAsInt("WRITE_TIMEOUT", 30)) * time.Second,

		MongoURI:      getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:       getEnv("MONGO_DB", "mailcast"),
		MongoUser:     getEnv("MONGO_USER", ""),
		MongoPassword: getEnv("MONGO_PASSWORD", ""),

		GmailClientID:     getEnv("GMAIL_CLIENT_ID", ""),
		GmailClientSecret: getEnv("GMAIL_CLIENT_SECRET", ""),
		GmailRefreshToken: getEnv("GMAIL_REFRESH_TOKEN", ""),
		GmailPollInterval: time.Duration(getEnvAsInt("GMAIL_POLL_INTERVAL", 60)) * time.Second,

		WhatsAppEndpoint: getEnv("WHATSAPP_ENDPOINT", ""),

		PostgresURI:      getEnv("POSTGRES_URI", ""),
		PostgresUser:     getEnv("POSTGRES_USER", ""),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", ""),
		PostgresDB:       getEnv("POSTGRES_DB", ""),
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
