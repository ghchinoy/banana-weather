// Package api implements the HTTP and Server-Sent Events handlers for Banana Weather.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"banana-weather/pkg/database"
	"banana-weather/pkg/weather"
)

// Handler serves the HTTP and SSE endpoints for the Banana Weather API.
type Handler struct {
	DB      *database.Client
	Weather *weather.Service
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// HandleGetPresets returns the list of curated preset locations from Firestore as JSON.
func (h *Handler) HandleGetPresets(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	presets, err := h.DB.GetPresets(r.Context())
	if err != nil {
		log.Printf("Failed to get presets from DB: %v", err)
		http.Error(w, "Failed to fetch presets", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(presets); err != nil {
		log.Printf("Failed to encode presets response: %v", err)
	}
}

// HandleGetWeather streams real-time weather image and video generation updates via Server-Sent Events.
func (h *Handler) HandleGetWeather(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	// Check for SSE support
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Helper to send SSE events safely (strip CR/LF to prevent SSE frame injection)
	sendEvent := func(event string, data string) {
		safeEvent := sanitizeSSEField(event)
		safeData := sanitizeSSEField(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", safeEvent, safeData)
		flusher.Flush()
	}

	city := r.URL.Query().Get("city")
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	// Call Service Flow
	if err := h.Weather.GetWeatherFlow(r.Context(), city, latStr, lngStr, sendEvent); err != nil {
		// Error is already logged and sent via SSE inside the service if needed.
		log.Printf("Weather flow finished with error: %v", err)
	}
}

func sanitizeSSEField(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
