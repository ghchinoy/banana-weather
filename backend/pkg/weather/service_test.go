package weather

import (
	"context"
	"fmt"
	"testing"
	"time"

	"banana-weather/pkg/database"
)

// -- Mocks --

type MockMapService struct {
	ResolvedCity string
	Err          error
}

func (m *MockMapService) GetReverseGeocoding(ctx context.Context, lat, lng float64) (string, error) {
	return m.ResolvedCity, m.Err
}
func (m *MockMapService) GetCityLocation(ctx context.Context, city string) (string, float64, float64, error) {
	return m.ResolvedCity, 0, 0, m.Err
}

type MockGenAI struct {
	ImageBase64 string
	VideoURI    string
	Err         error
}

func (m *MockGenAI) GenerateImage(ctx context.Context, city string, extra string, mode int) (string, error) {
	return m.ImageBase64, m.Err
}
func (m *MockGenAI) GenerateVideo(ctx context.Context, inputURI, prompt string) (string, error) {
	return m.VideoURI, m.Err
}

type MockStorage struct {
	PublicURL string
	GsURI     string
	Err       error
}

func (m *MockStorage) UploadImage(ctx context.Context, data, name string) (string, string, error) {
	return m.GsURI, m.PublicURL, m.Err
}

type MockDB struct {
	Loc *database.Location
	Err error
}

func (m *MockDB) GetLocation(ctx context.Context, id string) (*database.Location, error) {
	return m.Loc, m.Err
}
func (m *MockDB) UpsertLocation(ctx context.Context, loc database.Location) error {
	return nil
}

// -- Tests --

func TestGetWeatherFlow_CacheHit(t *testing.T) {
	ctx := t.Context()

	// Setup Mocks
	maps := &MockMapService{ResolvedCity: "Paris, France"}
	genai := &MockGenAI{}
	storage := &MockStorage{}

	// Pre-existing location in DB (Fresh)
	db := &MockDB{
		Loc: &database.Location{
			ID:          "paris_france",
			Name:        "Paris, France",
			ImageURL:    "http://cached/image.png",
			LastUpdated: time.Now(), // Fresh
		},
	}

	svc := NewService(maps, genai, storage, db)

	// Capture events
	var events []string
	callback := func(event, data string) {
		events = append(events, event)
	}

	if err := svc.GetWeatherFlow(ctx, "Paris", "", "", callback); err != nil {
		t.Fatalf("GetWeatherFlow(Paris) = %v, want nil", err)
	}

	// Verify "Loading cached forecast..." event
	foundCacheMsg := false
	for _, e := range events {
		if e == "result" {
			foundCacheMsg = true
		}
	}
	if !foundCacheMsg {
		t.Errorf("GetWeatherFlow(Paris) events = %v, want 'result' event for cache hit", events)
	}
}

func TestGetWeatherFlow_CacheMiss(t *testing.T) {
	ctx := t.Context()

	// Setup Mocks
	maps := &MockMapService{ResolvedCity: "London, UK"}
	genai := &MockGenAI{ImageBase64: "base64data", VideoURI: "gs://bucket/video.mp4"}
	storage := &MockStorage{PublicURL: "http://storage/image.png", GsURI: "gs://bucket/image.png"}

	// DB returns error (Not Found)
	db := &MockDB{Err: fmt.Errorf("not found")}

	svc := NewService(maps, genai, storage, db)

	var events []string
	callback := func(event, data string) {
		events = append(events, event)
	}

	if err := svc.GetWeatherFlow(ctx, "London", "", "", callback); err != nil {
		t.Fatalf("GetWeatherFlow(London) = %v, want nil", err)
	}

	// Verify events
	expected := []string{"status", "status", "status", "result", "status", "status", "status", "video"}
	if len(events) < len(expected) {
		t.Errorf("GetWeatherFlow(London) len(events) = %d, want >= %d", len(events), len(expected))
	}
}

func TestGetWeatherFlow_InvalidCoordinates(t *testing.T) {
	ctx := t.Context()
	svc := NewService(&MockMapService{}, &MockGenAI{}, &MockStorage{}, &MockDB{})

	var errorMsg string
	callback := func(event, data string) {
		if event == "error" {
			errorMsg = data
		}
	}

	if err := svc.GetWeatherFlow(ctx, "", "999.0", "-122.4", callback); err == nil {
		t.Fatal("GetWeatherFlow(lat=999.0) = nil, want error for out-of-range latitude")
	}
	if errorMsg != "Invalid coordinates provided." {
		t.Errorf("GetWeatherFlow(lat=999.0) error event = %q, want %q", errorMsg, "Invalid coordinates provided.")
	}
}

func TestGetWeatherFlow_DoesNotLeakSensitiveMapsError(t *testing.T) {
	ctx := t.Context()
	maps := &MockMapService{Err: fmt.Errorf("Get https://maps.googleapis.com/maps/api/geocode/json?key=AIzaSecretKey123: dial tcp timeout")}
	svc := NewService(maps, &MockGenAI{}, &MockStorage{}, &MockDB{})

	var errorMsg string
	callback := func(event, data string) {
		if event == "error" {
			errorMsg = data
		}
	}

	if err := svc.GetWeatherFlow(ctx, "Atlantis", "", "", callback); err == nil {
		t.Fatal("GetWeatherFlow(Atlantis) = nil, want error from Maps service")
	}
	if errorMsg != "Failed to find city." {
		t.Errorf("GetWeatherFlow(Atlantis) error event = %q, want sanitized %q", errorMsg, "Failed to find city.")
	}
}

func TestValidateLocationID(t *testing.T) {
	if err := ValidateLocationID("tokyo_japan"); err != nil {
		t.Errorf("ValidateLocationID(\"tokyo_japan\") = %v, want nil", err)
	}
	for _, bad := range []string{"", "../secret", "foo/bar", "foo\\bar"} {
		if err := ValidateLocationID(bad); err == nil {
			t.Errorf("ValidateLocationID(%q) = nil, want error", bad)
		}
	}
}
