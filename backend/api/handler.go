package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"banana-weather/pkg/database"
	"banana-weather/pkg/genai"
	"banana-weather/pkg/maps"
	"banana-weather/pkg/storage"
)

type Handler struct {
	Maps    *maps.Service
	GenAI   *genai.Service
	Storage *storage.Service
	DB      *database.Client
}

type WeatherResponse struct {
	City        string    `json:"city"`
	ImageBase64 string    `json:"image_base64,omitempty"`
	ImageURL    string    `json:"image_url,omitempty"`
	LastUpdated time.Time `json:"last_updated"`
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

func (h *Handler) HandleGetPresets(w http.ResponseWriter, r *http.Request) {
	// Fetch from Firestore
	presets, err := h.DB.GetPresets(r.Context())
	if err != nil {
		log.Printf("Failed to get presets from DB: %v", err)
		http.Error(w, "Failed to fetch presets", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(presets)
}

func (h *Handler) HandleGetWeather(w http.ResponseWriter, r *http.Request) {
	// Check for SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Helper to send SSE events
	sendEvent := func(event string, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	city := r.URL.Query().Get("city")
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	var formattedCity string
	var err error

	log.Printf("Received weather request. City: %s, Lat: %s, Lng: %s", city, latStr, lngStr)

	sendEvent("status", "Identifying location...")

	if latStr != "" && lngStr != "" {
		// Handle Coordinates
		var lat, lng float64
		fmt.Sscanf(latStr, "%f", &lat)
		fmt.Sscanf(lngStr, "%f", &lng)
		
		formattedCity, err = h.Maps.GetReverseGeocoding(r.Context(), lat, lng)
		if err != nil {
			log.Printf("Error reverse geocoding: %v", err)
			sendEvent("error", "Failed to resolve location: "+err.Error())
			return
		}
	} else {
		// Handle City Name (or default)
		if city == "" {
			city = "San Francisco"
		}

		// 1. Resolve City
		formattedCity, _, _, err = h.Maps.GetCityLocation(r.Context(), city)
		if err != nil {
			log.Printf("Error resolving location for city '%s': %v", city, err)
			sendEvent("error", "Failed to find city: "+err.Error())
			return
		}
	}
	
	log.Printf("Resolved location to: %s", formattedCity)
	sendEvent("status", "Found location: "+formattedCity)

	// --- CACHE CHECK ---
	locID := sanitizeID(formattedCity)
	cachedLoc, err := h.DB.GetLocation(r.Context(), locID)
	if err == nil && cachedLoc != nil && time.Since(cachedLoc.LastUpdated) < 3*time.Hour {
		log.Printf("Cache Hit for %s", formattedCity)
		sendEvent("status", "Loading cached forecast...")
		
		resp := WeatherResponse{
			City:        formattedCity,
			ImageURL:    cachedLoc.ImageURL,
			LastUpdated: cachedLoc.LastUpdated,
		}
		jsonData, _ := json.Marshal(resp)
		sendEvent("result", string(jsonData))
		
		if cachedLoc.VideoURL != "" {
			sendEvent("video", cachedLoc.VideoURL)
		}
		return
	}

	// 2. Generate Image
	sendEvent("status", fmt.Sprintf("Getting a banana image of the weather for %s...", formattedCity))
	
	// Use formattedCity to ensure the AI gets the full context
	imgBase64, err := h.GenAI.GenerateImage(r.Context(), formattedCity, "")
	if err != nil {
		log.Printf("Error generating image for '%s': %v", formattedCity, err)
		sendEvent("error", "Failed to generate image: "+err.Error())
		return
	}
	log.Printf("Successfully generated image for: %s", formattedCity)

	// Send Image to Frontend immediately (Base64)
	resp := WeatherResponse{
		City:        formattedCity,
		ImageBase64: imgBase64,
		LastUpdated: time.Now(),
	}
	jsonData, _ := json.Marshal(resp)
	sendEvent("result", string(jsonData))

	// 3. Generate Video (If Storage is available)
	if h.Storage == nil {
		log.Printf("Storage service not available, skipping video generation.")
		return
	}

	sendEvent("status", "Preparing for animation...")

	// Upload Image
	fileName := fmt.Sprintf("image_%d.png", time.Now().UnixNano())
	gsURI, publicImageURL, err := h.Storage.UploadImage(r.Context(), imgBase64, fileName)
	if err != nil {
		log.Printf("Failed to upload image for video gen: %v", err)
		return
	}

	// Upsert DB with Image URL (Partial Save)
	currentLoc := database.Location{
		ID:        locID,
		Name:      formattedCity,
		CityQuery: formattedCity,
		ImageURL:  publicImageURL,
		IsPreset:  false,
	}
	h.DB.UpsertLocation(r.Context(), currentLoc)

	sendEvent("status", "Animating (Veo 3.1)... this may take a minute.")

	// Call Veo (Use default prompt in pkg/genai)
	videoGsURI, err := h.GenAI.GenerateVideo(r.Context(), gsURI, "")
	if err != nil {
		log.Printf("Veo generation failed: %v", err)
		sendEvent("error", "Video generation failed (Beta). Enjoy the image!")
		return
	}

	sendEvent("status", "Finalizing video...")

	// Convert gs://bucket/path to https://storage.googleapis.com/bucket/path
	publicVideoURL := "https://storage.googleapis.com/" + videoGsURI[5:] // Strip gs://

	log.Printf("Video available at: %s", publicVideoURL)
	sendEvent("video", publicVideoURL)

	// Final Upsert with Video URL
	currentLoc.VideoURL = publicVideoURL
	h.DB.UpsertLocation(r.Context(), currentLoc)
}
