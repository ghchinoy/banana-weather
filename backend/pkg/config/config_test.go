package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Setup Environment
	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	os.Setenv("GENMEDIA_BUCKET", "test-bucket")
	os.Setenv("GOOGLE_MAPS_API_KEY", "test-key")
	os.Setenv("PORT", "9090")
	defer os.Clearenv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.ProjectID != "test-project" {
		t.Errorf("Expected ProjectID 'test-project', got '%s'", cfg.ProjectID)
	}
	if cfg.BucketName != "test-bucket" {
		t.Errorf("Expected BucketName 'test-bucket', got '%s'", cfg.BucketName)
	}
	if cfg.GoogleMapsKey != "test-key" {
		t.Errorf("Expected GoogleMapsKey 'test-key', got '%s'", cfg.GoogleMapsKey)
	}
	if cfg.Port != "9090" {
		t.Errorf("Expected Port '9090', got '%s'", cfg.Port)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	os.Clearenv()
	// Set only one
	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")

	_, err := Load()
	if err == nil {
		t.Error("Expected error when missing required fields, got nil")
	}
}
