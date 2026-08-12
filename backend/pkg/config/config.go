package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ProjectID string
	// ImageLocation is the Vertex AI location used for Gemini 3 image models.
	// These models are served at "global" (also "us"/"eu"), NOT at a regular region.
	ImageLocation string
	// VideoLocation is the Vertex AI region used for Veo video generation
	// (e.g. "us-central1"). Veo requires a regional endpoint, not "global".
	VideoLocation    string
	BucketName       string
	DatabaseID       string
	GoogleMapsKey    string
	Port             string
	GeminiImageModel string
	VeoVideoModel    string
}

// Load reads .env files and environment variables, validating required fields.
func Load() (*Config, error) {
	// Try loading .env files from various locations (root, parent, etc)
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")

	cfg := &Config{
		ProjectID: getEnvOr("GOOGLE_CLOUD_PROJECT", os.Getenv("PROJECT_ID")),
		// Image (Gemini 3) models are served at "global"; allowed values: global/us/eu.
		// GOOGLE_CLOUD_LOCATION is honored for backward compat (docs previously told
		// users to set it to "global" for the image models), but IMAGE_LOCATION wins.
		ImageLocation: firstNonEmpty(os.Getenv("IMAGE_LOCATION"), os.Getenv("GOOGLE_CLOUD_LOCATION"), "global"),
		// Veo requires a regional endpoint; it never runs at "global".
		VideoLocation:    getEnvOr("VEO_LOCATION", "us-central1"),
		BucketName:       os.Getenv("GENMEDIA_BUCKET"),
		DatabaseID:       getEnvOr("FIRESTORE_DATABASE", "(default)"),
		GoogleMapsKey:    os.Getenv("GOOGLE_MAPS_API_KEY"),
		Port:             getEnvOr("PORT", "8080"),
		GeminiImageModel: getEnvOr("GEMINI_IMAGE", "gemini-3.1-flash-image"),
		VeoVideoModel:    getEnvOr("VEO_MODEL", "veo-3.1-lite-generate-001"),
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

// firstNonEmpty returns the first non-empty string from vals, or "" if all empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
