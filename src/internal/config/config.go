package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration
type Config struct {
	Port         string
	DatabasePath string
}

// LoadConfig loads configuration from environment variables or .env file
func LoadConfig() (*Config, error) {
	// Load .env file and override existing environment variables
	if err := godotenv.Overload(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		Port:         getEnv("PORT", "8080"),
		DatabasePath: getEnv("DATABASE_PATH", "./allama.db"),
	}

	return cfg, nil
}

// getEnv retrieves an environment variable or returns a default value if not set
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
