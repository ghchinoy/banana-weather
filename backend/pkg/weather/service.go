// Package weather orchestrates geocoding, caching, and GenAI media generation.
package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"banana-weather/pkg/database"
)

// MapService defines the geocoding operations required by the weather orchestrator.
type MapService interface {
	GetReverseGeocoding(ctx context.Context, lat, lng float64) (string, error)
	GetCityLocation(ctx context.Context, city string) (string, float64, float64, error)
}

// GenAIService defines the image and video generation operations required by the weather orchestrator.
type GenAIService interface {
	GenerateImage(ctx context.Context, city string, extraContext string, promptMode int) (string, error)
	GenerateVideo(ctx context.Context, inputImageURI string, prompt string) (string, error)
}

// StorageService defines the Cloud Storage upload operations required by the weather orchestrator.
type StorageService interface {
	UploadImage(ctx context.Context, base64Data string, fileName string) (string, string, error)
}

// LocationRepo defines the persistence operations required by the weather orchestrator.
type LocationRepo interface {
	GetLocation(ctx context.Context, id string) (*database.Location, error)
	UpsertLocation(ctx context.Context, loc database.Location) error
}

// Service coordinates geocoding, Firestore caching, and GenAI media generation.
type Service struct {
	Maps    MapService
	GenAI   GenAIService
	Storage StorageService
	DB      LocationRepo
}

// NewService constructs a weather orchestration Service from its dependencies.
func NewService(m MapService, g GenAIService, s StorageService, db LocationRepo) *Service {
	return &Service{
		Maps:    m,
		GenAI:   g,
		Storage: s,
		DB:      db,
	}
}

// WeatherResponse mirrors the JSON response expected by the frontend.
type WeatherResponse struct {
	City        string    `json:"city"`
	ImageBase64 string    `json:"image_base64,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	LastUpdated time.Time `json:"last_updated"`
}

// StatusCallback is a function that sends real-time updates to the client.
type StatusCallback func(event string, data string)

// ValidateLocationID checks that a location or preset ID does not contain path separators or traversal sequences.
func ValidateLocationID(id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("location id cannot be empty")
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") || strings.Contains(trimmed, "..") {
		return fmt.Errorf("location id %q must not contain path separators or '..'", id)
	}
	return nil
}

func sanitizeID(s string) string {
	var result []rune
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result = append(result, r)
		} else {
			result = append(result, '_')
		}
	}
	return string(result)
}

const maxCityQueryLength = 200

// GetWeatherFlow orchestrates the entire weather generation process (Maps -> Cache -> AI -> Storage)
func (s *Service) GetWeatherFlow(ctx context.Context, cityQuery, latStr, lngStr string, sendStatus StatusCallback) error {
	var formattedCity string
	var err error

	cityQuery = strings.TrimSpace(cityQuery)
	if len(cityQuery) > maxCityQueryLength {
		sendStatus("error", "City name is too long.")
		return fmt.Errorf("city query exceeds max length (%d)", maxCityQueryLength)
	}

	log.Printf("Weather Flow Started. City: %s, Lat: %s, Lng: %s", cityQuery, latStr, lngStr)
	sendStatus("status", "Identifying location...")

	// 1. Resolve Location
	if latStr != "" && lngStr != "" {
		// Handle Coordinates
		lat, latErr := strconv.ParseFloat(latStr, 64)
		lng, lngErr := strconv.ParseFloat(lngStr, 64)
		if latErr != nil || lngErr != nil || math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lng) || math.IsInf(lng, 0) || lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			sendStatus("error", "Invalid coordinates provided.")
			return fmt.Errorf("invalid coordinates lat=%q lng=%q", latStr, lngStr)
		}

		formattedCity, err = s.Maps.GetReverseGeocoding(ctx, lat, lng)
		if err != nil {
			log.Printf("Error reverse geocoding: %v", err)
			sendStatus("error", "Failed to resolve location from coordinates.")
			return err
		}
	} else {
		// Handle City Name (or default)
		if cityQuery == "" {
			cityQuery = "San Francisco"
		}

		// Resolve City
		formattedCity, _, _, err = s.Maps.GetCityLocation(ctx, cityQuery)
		if err != nil {
			log.Printf("Error resolving location for city %q: %v", cityQuery, err)
			sendStatus("error", "Failed to find city.")
			return err
		}
	}

	log.Printf("Resolved location to: %s", formattedCity)
	sendStatus("status", "Found location: "+formattedCity)

	// 2. Cache Check
	locID := sanitizeID(formattedCity)
	cachedLoc, err := s.DB.GetLocation(ctx, locID)
	// Cache hit if exists and fresh (< 3 hours)
	if err == nil && cachedLoc != nil && time.Since(cachedLoc.LastUpdated) < 3*time.Hour {
		log.Printf("Cache Hit for %s", formattedCity)
		sendStatus("status", "Loading cached forecast...")

		resp := WeatherResponse{
			City:        formattedCity,
			ImageURL:    cachedLoc.ImageURL,
			LastUpdated: cachedLoc.LastUpdated,
		}
		jsonData, _ := json.Marshal(resp)
		sendStatus("result", string(jsonData))

		if cachedLoc.VideoURL != "" {
			sendStatus("video", cachedLoc.VideoURL)
		}
		return nil
	}

	// 3. Generate Image
	sendStatus("status", fmt.Sprintf("Getting a banana image of the weather for %s...", formattedCity))

	// Use formattedCity to ensure the AI gets the full context
	// Defaulting to Random prompt style (0) for standard web flow
	imgBase64, err := s.GenAI.GenerateImage(ctx, formattedCity, "", 0)
	if err != nil {
		log.Printf("Error generating image for %q: %v", formattedCity, err)
		sendStatus("error", "Failed to generate image for location.")
		return err
	}
	log.Printf("Successfully generated image for: %s", formattedCity)

	// Send Image to Frontend immediately (Base64)
	resp := WeatherResponse{
		City:        formattedCity,
		ImageBase64: imgBase64,
		LastUpdated: time.Now(),
	}
	jsonData, _ := json.Marshal(resp)
	sendStatus("result", string(jsonData))

	// 4. Generate Video (If Storage is available)
	if s.Storage == nil {
		log.Printf("Storage service not available, skipping video generation.")
		return nil
	}

	sendStatus("status", "Preparing for animation...")

	// Upload Image
	fileName := fmt.Sprintf("image_%d.png", time.Now().UnixNano())
	gsURI, publicImageURL, err := s.Storage.UploadImage(ctx, imgBase64, fileName)
	if err != nil {
		log.Printf("Failed to upload image for video gen: %v", err)
		// We don't error out the user here, they have the image. just log it.
		return nil
	}

	// Upsert DB with Image URL (Partial Save)
	currentLoc := database.Location{
		ID:          locID,
		Name:        formattedCity,
		CityQuery:   formattedCity,
		ImageURL:    publicImageURL,
		IsPreset:    false,
		LastUpdated: time.Now(),
	}
	if err := s.DB.UpsertLocation(ctx, currentLoc); err != nil {
		log.Printf("Warning: failed to upsert location image URL for %s: %v", locID, err)
	}

	sendStatus("status", "Animating (Veo 3.1)... this may take a minute.")

	// Call Veo
	videoGsURI, err := s.GenAI.GenerateVideo(ctx, gsURI, "")
	if err != nil {
		log.Printf("Veo generation failed: %v", err)
		sendStatus("error", "Video generation failed (Beta). Enjoy the image!")
		return nil
	}

	sendStatus("status", "Finalizing video...")

	// Convert gs://bucket/path to https://storage.googleapis.com/bucket/path safely
	publicVideoURL := "https://storage.googleapis.com/" + strings.TrimPrefix(videoGsURI, "gs://")

	log.Printf("Video available at: %s", publicVideoURL)
	sendStatus("video", publicVideoURL)

	// Final Upsert with Video URL
	currentLoc.VideoURL = publicVideoURL
	if err := s.DB.UpsertLocation(ctx, currentLoc); err != nil {
		log.Printf("Warning: failed to upsert location video URL for %s: %v", locID, err)
	}

	return nil
}
