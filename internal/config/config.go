package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all application configurations.
type Config struct {
	GeminiAPIKey        string
	TelexAPIBaseURL     string
	TelexBotUserID      string
	SupabaseURL         string
	SupabaseServiceKey  string
}

// LoadConfig reads configuration from environment variables or .env file.
func LoadConfig() *Config {
	// Load .env file if it exists
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("Error loading .env file: %v", err)
	} else if os.IsNotExist(err) {
		log.Println(".env file not found, loading config from environment variables.")
	} else {
		log.Println(".env file loaded successfully.")
	}

	cfg := &Config{
		GeminiAPIKey:        getEnv("GEMINI_API_KEY"),
		TelexAPIBaseURL:     getEnv("TELEX_API_BASE_URL"),
		TelexBotUserID:      getEnv("TELEX_BOT_USER_ID"),
		SupabaseURL:         getEnv("SUPABASE_URL"),
		SupabaseServiceKey:  getEnv("SUPABASE_SERVICE_KEY"),
	}

	validateConfig(cfg)

	return cfg
}

func getEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Environment variable %s not set", key)
	}
	return value
}

func validateConfig(cfg *Config) {
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is not set in environment variables or .env file.")
	}
	if cfg.TelexAPIBaseURL == "" {
		log.Fatal("TELEX_API_BASE_URL is not set in environment variables or .env file.")
	}
	if cfg.TelexBotUserID == "" {
		log.Fatal("TELEX_BOT_USER_ID is not set in environment variables or .env file.")
	}
	if cfg.SupabaseURL == "" {
		log.Fatal("SUPABASE_URL is not set in environment variables or .env file.")
	}
	if cfg.SupabaseServiceKey == "" {
		log.Fatal("SUPABASE_SERVICE_KEY is not set in environment variables or .env file.")
	}
}
