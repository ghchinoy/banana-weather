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
	ctx := context.Background()
	
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

	err := svc.GetWeatherFlow(ctx, "Paris", "", "", callback)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify "Loading cached forecast..." event
	foundCacheMsg := false
	for _, e := range events {
		if e == "result" {
			foundCacheMsg = true
		}
	}
	if !foundCacheMsg {
		t.Error("Expected 'result' event for cache hit")
	}
}

func TestGetWeatherFlow_CacheMiss(t *testing.T) {
	ctx := context.Background()

	// Setup Mocks
	maps := &MockMapService{ResolvedCity: "London, UK"}
	genai := &MockGenAI{ImageBase64: "base64data", VideoURI: "gs://bucket/video.mp4"}
	storage := &MockStorage{PublicURL: "http://storage/image.png", GsURI: "gs://bucket/image.png"}
	
	// DB returns error (Not Found)
	db := &MockDB{Err: fmt.Errorf("not found")} // Simulate 404 behavior, usually err!=nil

	svc := NewService(maps, genai, storage, db)

	var events []string
	callback := func(event, data string) {
		events = append(events, event)
	}

	err := svc.GetWeatherFlow(ctx, "London", "", "", callback)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify events
	expected := []string{"status", "status", "status", "result", "status", "status", "status", "video"}
	if len(events) < len(expected) {
		t.Errorf("Expected at least %d events, got %d", len(expected), len(events))
	}
}
