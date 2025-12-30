package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"banana-weather/pkg/database"
	"banana-weather/pkg/weather"
)

type Handler struct {
	DB      *database.Client
	Weather *weather.Service
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

	// Call Service Flow
	err := h.Weather.GetWeatherFlow(r.Context(), city, latStr, lngStr, sendEvent)
	if err != nil {
		// Error is already logged and sent via SSE inside the service if needed,
		// or we can catch generic errors here.
		// The service sends "error" events for user-facing issues.
		log.Printf("Weather flow finished with error: %v", err)
	}
}
