package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	os.Clearenv()
	t.Cleanup(os.Clearenv)

	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
	os.Setenv("GENMEDIA_BUCKET", "test-bucket")
	os.Setenv("GOOGLE_MAPS_API_KEY", "test-key")
	os.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if cfg.ProjectID != "test-project" {
		t.Errorf("Load().ProjectID = %q, want %q", cfg.ProjectID, "test-project")
	}
	if cfg.BucketName != "test-bucket" {
		t.Errorf("Load().BucketName = %q, want %q", cfg.BucketName, "test-bucket")
	}
	if cfg.GoogleMapsKey != "test-key" {
		t.Errorf("Load().GoogleMapsKey = %q, want %q", cfg.GoogleMapsKey, "test-key")
	}
	if cfg.Port != "9090" {
		t.Errorf("Load().Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.GeminiImageModel != DefaultImageModel {
		t.Errorf("Load().GeminiImageModel = %q, want %q", cfg.GeminiImageModel, DefaultImageModel)
	}
	if cfg.VeoVideoModel != DefaultVideoModel {
		t.Errorf("Load().VeoVideoModel = %q, want %q", cfg.VeoVideoModel, DefaultVideoModel)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	os.Clearenv()
	t.Cleanup(os.Clearenv)

	os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")

	if _, err := Load(); err == nil {
		t.Error("Load() with missing required env vars = nil, want non-nil error")
	}
}

// TestLoadLocationPrecedence covers the per-service location-resolution logic that
// is the heart of the image=global / video=region fix:
//   - ImageLocation = firstNonEmpty(IMAGE_LOCATION, GOOGLE_CLOUD_LOCATION, "global")
//   - VideoLocation = getEnvOr(VEO_LOCATION, "us-central1")  (ignores GOOGLE_CLOUD_LOCATION)
//
// Each case runs with a fully isolated environment (saved/cleared/restored) so it
// neither pollutes nor is polluted by the other tests in this package.
func TestLoadLocationPrecedence(t *testing.T) {
	cases := []struct {
		name              string
		imageLocation     string // IMAGE_LOCATION
		googleCloudLoc    string // GOOGLE_CLOUD_LOCATION
		veoLocation       string // VEO_LOCATION
		wantImageLocation string
		wantVideoLocation string
	}{
		{
			name:              "IMAGE_LOCATION wins over GOOGLE_CLOUD_LOCATION",
			imageLocation:     "us",
			googleCloudLoc:    "eu",
			wantImageLocation: "us",
			wantVideoLocation: "us-central1",
		},
		{
			name:              "GOOGLE_CLOUD_LOCATION drives ImageLocation when IMAGE_LOCATION unset",
			googleCloudLoc:    "eu",
			wantImageLocation: "eu",
			wantVideoLocation: "us-central1",
		},
		{
			name:              "ImageLocation defaults to global when both unset",
			wantImageLocation: "global",
			wantVideoLocation: "us-central1",
		},
		{
			name:              "VideoLocation is unaffected by GOOGLE_CLOUD_LOCATION",
			googleCloudLoc:    "us-east4",
			wantImageLocation: "us-east4",
			wantVideoLocation: "us-central1",
		},
		{
			name:              "VEO_LOCATION overrides VideoLocation default",
			veoLocation:       "europe-west4",
			wantImageLocation: "global",
			wantVideoLocation: "europe-west4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Isolate env: snapshot, clear, restore on exit.
			saved := os.Environ()
			os.Clearenv()
			t.Cleanup(func() {
				os.Clearenv()
				for _, kv := range saved {
					if i := strings.IndexByte(kv, '='); i >= 0 {
						os.Setenv(kv[:i], kv[i+1:])
					}
				}
			})

			// Required fields so Load() succeeds.
			os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
			os.Setenv("GENMEDIA_BUCKET", "test-bucket")
			os.Setenv("GOOGLE_MAPS_API_KEY", "test-key")

			if tc.imageLocation != "" {
				os.Setenv("IMAGE_LOCATION", tc.imageLocation)
			}
			if tc.googleCloudLoc != "" {
				os.Setenv("GOOGLE_CLOUD_LOCATION", tc.googleCloudLoc)
			}
			if tc.veoLocation != "" {
				os.Setenv("VEO_LOCATION", tc.veoLocation)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if cfg.ImageLocation != tc.wantImageLocation {
				t.Errorf("ImageLocation = %q, want %q", cfg.ImageLocation, tc.wantImageLocation)
			}
			if cfg.VideoLocation != tc.wantVideoLocation {
				t.Errorf("VideoLocation = %q, want %q", cfg.VideoLocation, tc.wantVideoLocation)
			}
		})
	}
}
