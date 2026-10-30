package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"banana-weather/pkg/database"
	"banana-weather/pkg/weather"
)

// -- Minimal fakes implementing the exported weather interfaces --
// These mirror the fakes in pkg/weather/service_test.go, which are not
// importable from the api package.

type fakeMaps struct {
	resolvedCity string
	err          error
}

func (f *fakeMaps) GetReverseGeocoding(ctx context.Context, lat, lng float64) (string, error) {
	return f.resolvedCity, f.err
}
func (f *fakeMaps) GetCityLocation(ctx context.Context, city string) (string, float64, float64, error) {
	return f.resolvedCity, 0, 0, f.err
}

type fakeGenAI struct {
	imageBase64 string
	videoURI    string
	err         error
	imageCalls  int
	videoCalls  int
}

func (f *fakeGenAI) GenerateImage(ctx context.Context, city, extra string, mode int) (string, error) {
	f.imageCalls++
	return f.imageBase64, f.err
}
func (f *fakeGenAI) GenerateVideo(ctx context.Context, inputURI, prompt string) (string, error) {
	f.videoCalls++
	return f.videoURI, f.err
}

type fakeStorage struct {
	publicURL string
	gsURI     string
	err       error
}

func (f *fakeStorage) UploadImage(ctx context.Context, data, name string) (string, string, error) {
	return f.gsURI, f.publicURL, f.err
}

type fakeDB struct {
	loc *database.Location
	err error
}

func (f *fakeDB) GetLocation(ctx context.Context, id string) (*database.Location, error) {
	return f.loc, f.err
}
func (f *fakeDB) UpsertLocation(ctx context.Context, loc database.Location) error {
	return nil
}

// sseEvent is a single parsed Server-Sent Event.
type sseEvent struct {
	event string
	data  string
}

// parseSSE parses an event-stream body into an ordered slice of events.
// It expects the "event: <name>\ndata: <payload>\n\n" framing used by the handler.
func parseSSE(t *testing.T, body string) []sseEvent {
	t.Helper()
	var events []sseEvent
	for _, block := range strings.Split(strings.TrimSpace(body), "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var ev sseEvent
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			default:
				t.Fatalf("unexpected SSE line (bad framing): %q", line)
			}
		}
		events = append(events, ev)
	}
	return events
}

func eventNames(events []sseEvent) []string {
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = e.event
	}
	return names
}

// containsInOrder reports whether want appears as a subsequence of got.
func containsInOrder(got, want []string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

// firstEvent returns the first event with the given name.
func firstEvent(events []sseEvent, name string) (sseEvent, bool) {
	for _, e := range events {
		if e.event == name {
			return e, true
		}
	}
	return sseEvent{}, false
}

func TestHandleGetWeather_CacheMiss_SSEStream(t *testing.T) {
	genai := &fakeGenAI{imageBase64: "base64data", videoURI: "gs://bucket/video.mp4"}
	svc := weather.NewService(
		&fakeMaps{resolvedCity: "London, UK"},
		genai,
		&fakeStorage{publicURL: "http://storage/image.png", gsURI: "gs://bucket/image.png"},
		&fakeDB{err: fmt.Errorf("not found")}, // cache miss
	)
	h := &Handler{Weather: svc}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/weather?city=London", nil)

	h.HandleGetWeather(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q, want no-cache", got)
	}

	events := parseSSE(t, rec.Body.String())
	if len(events) == 0 {
		t.Fatal("expected SSE events, got none")
	}

	names := eventNames(events)
	// Cache-miss should drive the full flow: image result then video.
	if !containsInOrder(names, []string{"status", "result", "video"}) {
		t.Fatalf("event sequence %v does not contain status -> result -> video", names)
	}

	// The result event carries a JSON WeatherResponse for the generated image.
	result, ok := firstEvent(events, "result")
	if !ok {
		t.Fatal("expected a result event")
	}
	var resp weather.WeatherResponse
	if err := json.Unmarshal([]byte(result.data), &resp); err != nil {
		t.Fatalf("result data is not valid JSON WeatherResponse: %v (data=%q)", err, result.data)
	}
	if resp.City != "London, UK" {
		t.Errorf("result city = %q, want %q", resp.City, "London, UK")
	}
	if resp.ImageBase64 != "base64data" {
		t.Errorf("result image_base64 = %q, want %q", resp.ImageBase64, "base64data")
	}

	// The video event carries the public HTTPS URL derived from the gs:// URI.
	video, ok := firstEvent(events, "video")
	if !ok {
		t.Fatal("expected a video event")
	}
	if want := "https://storage.googleapis.com/bucket/video.mp4"; video.data != want {
		t.Errorf("video data = %q, want %q", video.data, want)
	}

	if genai.imageCalls != 1 {
		t.Errorf("GenerateImage called %d times, want 1", genai.imageCalls)
	}
	if genai.videoCalls != 1 {
		t.Errorf("GenerateVideo called %d times, want 1", genai.videoCalls)
	}
}

func TestHandleGetWeather_CacheHit_SkipsGeneration(t *testing.T) {
	genai := &fakeGenAI{}
	svc := weather.NewService(
		&fakeMaps{resolvedCity: "Paris, France"},
		genai,
		&fakeStorage{},
		&fakeDB{loc: &database.Location{
			ID:          "paris_france",
			Name:        "Paris, France",
			ImageURL:    "http://cached/image.png",
			VideoURL:    "https://storage.googleapis.com/bucket/cached.mp4",
			LastUpdated: time.Now(), // fresh -> cache hit
		}},
	)
	h := &Handler{Weather: svc}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/weather?city=Paris", nil)

	h.HandleGetWeather(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	events := parseSSE(t, rec.Body.String())
	names := eventNames(events)

	result, ok := firstEvent(events, "result")
	if !ok {
		t.Fatalf("expected a result event, got sequence %v", names)
	}
	var resp weather.WeatherResponse
	if err := json.Unmarshal([]byte(result.data), &resp); err != nil {
		t.Fatalf("result data is not valid JSON WeatherResponse: %v (data=%q)", err, result.data)
	}
	if resp.ImageURL != "http://cached/image.png" {
		t.Errorf("result image_url = %q, want cached URL", resp.ImageURL)
	}

	// Cache hit must serve the cached video and never invoke generation.
	if _, ok := firstEvent(events, "video"); !ok {
		t.Errorf("expected a video event from cache, got sequence %v", names)
	}
	if genai.imageCalls != 0 {
		t.Errorf("GenerateImage called %d times on cache hit, want 0", genai.imageCalls)
	}
	if genai.videoCalls != 0 {
		t.Errorf("GenerateVideo called %d times on cache hit, want 0", genai.videoCalls)
	}
}

func TestSanitizeSSEField_StripsCRLF(t *testing.T) {
	got := sanitizeSSEField("London\r\nevent: injected\ndata: payload")
	want := "London  event: injected data: payload"
	if got != want {
		t.Errorf("sanitizeSSEField(multiline) = %q, want %q", got, want)
	}
}
