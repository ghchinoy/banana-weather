// Package genai wraps the Google GenAI Vertex AI clients for image and video generation.
package genai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"strings"
	"time"

	"google.golang.org/genai"
)

// Service orchestrates image and video generation across per-location Vertex AI clients.
type Service struct {
	// imageClient is bound to the image location ("global" for Gemini 3 image models).
	imageClient *genai.Client
	// videoClient is bound to the video region (e.g. "us-central1" for Veo).
	// It is also used to poll the Veo LRO it created.
	videoClient *genai.Client
	bucketName  string
	imageModel  string
	videoModel  string
}

// NewService builds the GenAI service. The genai Vertex client binds its location
// at construction, but Gemini 3 image models are served at "global" while Veo
// requires a regional endpoint. A single client cannot serve both, so we construct
// two clients: one per location.
func NewService(ctx context.Context, projectID, imageLocation, videoLocation, bucketName, imageModel, videoModel string) (*Service, error) {
	log.Printf("Initializing GenAI Service. Project: %s, ImageLocation: %s, VideoLocation: %s, Bucket: %s", projectID, imageLocation, videoLocation, bucketName)

	imageClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  projectID,
		Location: imageLocation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init image genai client (location %q): %w", imageLocation, err)
	}

	videoClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  projectID,
		Location: videoLocation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init video genai client (location %q): %w", videoLocation, err)
	}

	return &Service{
		imageClient: imageClient,
		videoClient: videoClient,
		bucketName:  bucketName,
		imageModel:  imageModel,
		videoModel:  videoModel,
	}, nil
}

// GenerateImage generates a 9:16 image for the given city.
// promptMode: 0=Random, 1=Classic, 2=Drink
func (s *Service) GenerateImage(ctx context.Context, city string, extraContext string, promptMode int) (string, error) {
	// a clever prompt inspired by @dotey https://x.com/dotey/status/1993729800922341810?s=20
	const basePromptTemplate = `Present a clear, 45° top-down view of a vertical (9:16) isometric miniature 3D cartoon scene, highlighting iconic landmarks centered in the composition to showcase precise and delicate modeling.

The scene features soft, refined textures with realistic PBR materials and gentle, lifelike lighting and shadow effects. Weather elements are creatively integrated into the urban architecture, establishing a dynamic interaction between the city's landscape and atmospheric conditions, creating an immersive weather ambiance.

Use a clean, unified composition with minimalistic aesthetics and a soft, solid-colored background that highlights the main content. The overall visual style is fresh and soothing.

Display a prominent weather icon at the top-center, with the date (x-small text) and temperature range (medium text) beneath it. The city name (large text) is positioned directly above the weather icon. The weather information has no background and can subtly overlap with the buildings.

The text should match the input city's native language.
Please retrieve current weather conditions for the specified city before rendering.`

	const secondaryPromptTemplate = `Present a clear, 45° top-down view of a vertical (9:16) isometric miniature 3D cartoon scene, highlighting iconic landmarks centered in the composition to showcase precise and delicate modeling. 

A close-up of a porcelain [DRINK] cup filled with [DRINK], subtly floating a detailed city of [CITY] occupying most of the composition. Prominently displayed at the scene's center are the city's most iconic landmarks, vividly detailed and illuminated softly. 

Miniature streets feature realistic, tiny vehicles moving seamlessly. With cinematic-quality lighting and depth-of-field blurring, the image creates a magical, dreamlike atmosphere. Exceptionally detailed and highly photorealistic, the scene achieves an 8K cinematic finish. 

Display a prominent weather icon at the top-center, with the date (x-small text) and temperature range (medium text) beneath it. The city name (large text) is positioned directly above the weather icon. The weather information has no background and can subtly overlap with the buildings. The text should match the input city's native language. Please retrieve current weather conditions for the specified city before rendering.`

	var useSecondary bool
	switch promptMode {
	case 1: // Force Classic
		useSecondary = false
	case 2: // Force Drink
		useSecondary = true
	default: // Random (0 or other)
		useSecondary = rand.IntN(2) == 1
	}

	var prompt string
	if !useSecondary {
		// Use Base Prompt
		log.Printf("Selected Base Prompt for %s (Mode: %d)", city, promptMode)
		prompt = fmt.Sprintf("%s\n\nCity name: %s", basePromptTemplate, city)
	} else {
		// Use Secondary Prompt
		log.Printf("Selected Secondary (Drink) Prompt for %s (Mode: %d)", city, promptMode)
		// Fill [CITY] placeholder
		p := strings.ReplaceAll(secondaryPromptTemplate, "[CITY]", city)
		// Instruct model to resolve [DRINK]
		prompt = fmt.Sprintf("%s\n\nDRINK: the most common AM drink for this location", p)
	}

	if extraContext != "" {
		prompt += fmt.Sprintf("\n\nContext/Setting: %s", extraContext)
	}

	model := s.imageModel
	if model == "" {
		model = "gemini-nano-banana-2.1"
	}

	log.Printf("Generating image for city: %s using model: %s (GenerateContent)", city, model)

	genCfg := &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE"},
		ImageConfig: &genai.ImageConfig{
			AspectRatio: "9:16",
		},
	}
	// gemini-3.1-flash-lite-image rejects tools payloads with HTTP 400 INVALID_ARGUMENT.
	if !strings.Contains(strings.ToLower(model), "lite") {
		genCfg.Tools = []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		}
	} else {
		log.Printf("Skipping GoogleSearch tool for lite image model %s", model)
	}

	resp, err := s.imageClient.Models.GenerateContent(ctx, model, genai.Text(prompt), genCfg)
	if err != nil {
		log.Printf("GenAI GenerateContent failed: %v", err)
		return "", fmt.Errorf("genai error: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		log.Printf("GenAI returned no candidates or parts")
		return "", fmt.Errorf("no content generated")
	}

	imgBytes := extractFinalImageBytes(resp.Candidates[0].Content.Parts)
	if len(imgBytes) > 0 {
		log.Printf("Image generated successfully. Bytes: %d", len(imgBytes))
		return base64.StdEncoding.EncodeToString(imgBytes), nil
	}

	log.Printf("No inline image data found in response")
	return "", fmt.Errorf("no image data found in response")
}

// extractFinalImageBytes selects the final non-thought image part from a candidate's parts.
// Models with thinking enabled by default (such as gemini-nano-banana-2.1) emit an interim
// draft image marked Thought=true before the final higher-resolution render (Thought=false).
// If only thought image parts are present, it falls back to the last image part.
func extractFinalImageBytes(parts []*genai.Part) []byte {
	var fallback []byte
	for _, part := range parts {
		if part == nil || part.InlineData == nil || len(part.InlineData.Data) == 0 {
			continue
		}
		if !part.Thought {
			return part.InlineData.Data
		}
		fallback = part.InlineData.Data
	}
	return fallback
}

// DefaultVideoPrompt is the default camera motion prompt passed to Veo when no override is supplied.
const DefaultVideoPrompt = "The camera moves in parallax as the elements in the image move naturally, while the forecast data—the bold title—remains fixed."

// GenerateVideo generates a 9:16 video using Veo 3.1 (Lite by default).
// Returns: GS URI (string) or error.
func (s *Service) GenerateVideo(ctx context.Context, inputImageURI string, prompt string) (string, error) {
	model := s.videoModel
	if model == "" {
		model = "veo-3.1-lite-generate-001"
	}

	if prompt == "" {
		prompt = DefaultVideoPrompt
	}

	log.Printf("Generating video with model %s. Input: %s", model, inputImageURI)

	// Construct the image object
	image := &genai.Image{
		GCSURI:   inputImageURI,
		MIMEType: "image/png",
	}

	// Config
	config := &genai.GenerateVideosConfig{
		AspectRatio:  "9:16",
		OutputGCSURI: fmt.Sprintf("gs://%s/videos/", s.bucketName),
	}

	// Call GenerateVideos
	resp, err := s.videoClient.Models.GenerateVideos(ctx, model, prompt, image, config)
	if err != nil {
		log.Printf("GenAI GenerateVideos failed: %v", err)
		return "", fmt.Errorf("veo error: %w", err)
	}

	log.Printf("Veo operation started. ID: %s", resp.Name)

	// Ensure polling is bounded even if the caller passed context.Background().
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
	}

	// Polling Loop using Native SDK method
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled during polling: %w", ctx.Err())
		case <-ticker.C:
			// Use native SDK polling
			op, err := s.videoClient.Operations.GetVideosOperation(ctx, resp, nil)
			if err != nil {
				log.Printf("Native SDK Polling failed: %v", err)
				continue
			}

			if op.Done {
				if op.Error != nil {
					return "", fmt.Errorf("operation failed: %v", op.Error)
				}

				if op.Response == nil || len(op.Response.GeneratedVideos) == 0 {
					return "", fmt.Errorf("operation done but no videos found")
				}

				v := op.Response.GeneratedVideos[0]
				if uri := extractGeneratedVideoURI(v); uri != "" {
					log.Printf("Video generated (GCS URI): %s", uri)
					return uri, nil
				}

				b, _ := json.Marshal(v)
				return "", fmt.Errorf("video generated but URI is empty (JSON: %s)", string(b))
			}
			log.Printf("Still polling Veo...")
		}
	}
}

func extractGeneratedVideoURI(v *genai.GeneratedVideo) string {
	if v == nil {
		return ""
	}
	if v.Video != nil && v.Video.URI != "" {
		return v.Video.URI
	}

	// Fallback: inspect serialized JSON in case field names vary across backend versions.
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}

	for _, key := range []string{"gcsUri", "videoUri", "uri"} {
		if uri, ok := m[key].(string); ok && uri != "" {
			return uri
		}
	}
	if vid, ok := m["video"].(map[string]any); ok {
		for _, key := range []string{"uri", "gcsUri", "videoUri"} {
			if uri, ok := vid[key].(string); ok && uri != "" {
				return uri
			}
		}
	}
	return ""
}
