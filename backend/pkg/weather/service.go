package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"banana-weather/pkg/database"
)

// -- Interfaces --

type MapService interface {
	GetReverseGeocoding(ctx context.Context, lat, lng float64) (string, error)
	GetCityLocation(ctx context.Context, city string) (string, float64, float64, error)
}

type GenAIService interface {
	GenerateImage(ctx context.Context, city string, extraContext string, promptMode int) (string, error)
	GenerateVideo(ctx context.Context, inputImageURI string, prompt string) (string, error)
}

type StorageService interface {
	UploadImage(ctx context.Context, base64Data string, fileName string) (string, string, error)
}

type LocationRepo interface {
	GetLocation(ctx context.Context, id string) (*database.Location, error)
	UpsertLocation(ctx context.Context, loc database.Location) error
}

// -- Service --

type Service struct {
	Maps    MapService
	GenAI   GenAIService
	Storage StorageService
	DB      LocationRepo
}

func NewService(m MapService, g GenAIService, s StorageService, db LocationRepo) *Service {
	return &Service{
		Maps:    m,
		GenAI:   g,
		Storage: s,
		DB:      db,
	}
}

// WeatherResponse mirrors the JSON response expected by the frontend
type WeatherResponse struct {
	City        string    `json:"city"`
	ImageBase64 string    `json:"image_base64,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	LastUpdated time.Time `json:"last_updated"`
}

// StatusCallback is a function that sends real-time updates to the client
type StatusCallback func(event string, data string)

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

// GetWeatherFlow orchestrates the entire weather generation process (Maps -> Cache -> AI -> Storage)
func (s *Service) GetWeatherFlow(ctx context.Context, cityQuery, latStr, lngStr string, sendStatus StatusCallback) error {
	var formattedCity string
	var err error

	log.Printf("Weather Flow Started. City: %s, Lat: %s, Lng: %s", cityQuery, latStr, lngStr)
	sendStatus("status", "Identifying location...")

	// 1. Resolve Location
	if latStr != "" && lngStr != "" {
		// Handle Coordinates
		var lat, lng float64
		fmt.Sscanf(latStr, "%f", &lat)
		fmt.Sscanf(lngStr, "%f", &lng)

		formattedCity, err = s.Maps.GetReverseGeocoding(ctx, lat, lng)
		if err != nil {
			log.Printf("Error reverse geocoding: %v", err)
			sendStatus("error", "Failed to resolve location: "+err.Error())
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
			log.Printf("Error resolving location for city '%s': %v", cityQuery, err)
			sendStatus("error", "Failed to find city: "+err.Error())
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
		log.Printf("Error generating image for '%s': %v", formattedCity, err)
		sendStatus("error", "Failed to generate image: "+err.Error())
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
		ID:        locID,
		Name:      formattedCity,
		CityQuery: formattedCity,
		ImageURL:  publicImageURL,
		IsPreset:  false,
		LastUpdated: time.Now(),
	}
	s.DB.UpsertLocation(ctx, currentLoc)

	sendStatus("status", "Animating (Veo 3.1)... this may take a minute.")

	// Call Veo
	videoGsURI, err := s.GenAI.GenerateVideo(ctx, gsURI, "")
	if err != nil {
		log.Printf("Veo generation failed: %v", err)
		sendStatus("error", "Video generation failed (Beta). Enjoy the image!")
		return nil
	}

	sendStatus("status", "Finalizing video...")

	// Convert gs://bucket/path to https://storage.googleapis.com/bucket/path
	// Assuming bucket is public or we need signed URLs. Code used string replacement before.
	// We need the bucket name to do the replacement if the URI is gs://...
	// The GenAI service returns gs://...
	// We can extract bucket from there or read env again. 
	// Ideally the service shouldn't know about ENV too much, but let's stick to the previous pattern:
	// "https://storage.googleapis.com/" + videoGsURI[5:]
	
	publicVideoURL := "https://storage.googleapis.com/" + videoGsURI[5:]

	log.Printf("Video available at: %s", publicVideoURL)
	sendStatus("video", publicVideoURL)

	// Final Upsert with Video URL
	currentLoc.VideoURL = publicVideoURL
	s.DB.UpsertLocation(ctx, currentLoc)

	return nil
}
