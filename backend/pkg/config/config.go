package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ProjectID     string
	Location      string
	BucketName    string
	DatabaseID    string
	GoogleMapsKey string
	Port          string
}

// Load reads .env files and environment variables, validating required fields.
func Load() (*Config, error) {
	// Try loading .env files from various locations (root, parent, etc)
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")

	cfg := &Config{
		ProjectID:     getEnvOr("GOOGLE_CLOUD_PROJECT", os.Getenv("PROJECT_ID")),
		Location:      getEnvOr("GOOGLE_CLOUD_LOCATION", "us-central1"),
		BucketName:    os.Getenv("GENMEDIA_BUCKET"),
		DatabaseID:    getEnvOr("FIRESTORE_DATABASE", "(default)"),
		GoogleMapsKey: os.Getenv("GOOGLE_MAPS_API_KEY"),
		Port:          getEnvOr("PORT", "8080"),
	}

	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("GOOGLE_CLOUD_PROJECT or PROJECT_ID is required")
	}
	if cfg.BucketName == "" {
		return nil, fmt.Errorf("GENMEDIA_BUCKET is required")
	}
	if cfg.GoogleMapsKey == "" {
		return nil, fmt.Errorf("GOOGLE_MAPS_API_KEY is required")
	}

	return cfg, nil
}

func getEnvOr(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
