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

type Service struct {
	client     *genai.Client
	bucketName string
}

func NewService(ctx context.Context, projectID, location, bucketName string) (*Service, error) {
	log.Printf("Initializing GenAI Service. Project: %s, Location: %s, Bucket: %s", projectID, location, bucketName)

	// Initialize GenAI Client
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  projectID,
		Location: location,
	})
	if err != nil {
		return nil, err
	}

	return &Service{client: c, bucketName: bucketName}, nil
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
		p := strings.Replace(secondaryPromptTemplate, "[CITY]", city, -1)
		// Instruct model to resolve [DRINK]
		prompt = fmt.Sprintf("%s\n\nDRINK: the most common AM drink for this location", p)
	}

	if extraContext != "" {
		prompt += fmt.Sprintf("\n\nContext/Setting: %s", extraContext)
	}

	// Nano Banana Pro corresponds to 'gemini-3-pro-image-preview'
	model := "gemini-3-pro-image-preview"

	log.Printf("Generating image for city: %s using model: %s (GenerateContent)", city, model)

	resp, err := s.client.Models.GenerateContent(ctx, model, genai.Text(prompt), &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE"},
		Tools: []*genai.Tool{
			{GoogleSearch: &genai.GoogleSearch{}},
		},
		ImageConfig: &genai.ImageConfig{
			AspectRatio: "9:16",
		},
	})
	if err != nil {
		log.Printf("GenAI GenerateContent failed: %v", err)
		return "", fmt.Errorf("genai error: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		log.Printf("GenAI returned no candidates or parts")
		return "", fmt.Errorf("no content generated")
	}

	// Iterate through parts to find the image
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			log.Printf("Image generated successfully. Bytes: %d", len(part.InlineData.Data))
			return base64.StdEncoding.EncodeToString(part.InlineData.Data), nil
		}
	}
	
	log.Printf("No inline image data found in response")
	return "", fmt.Errorf("no image data found in response")
}

const DefaultVideoPrompt = "The camera moves in parallax as the elements in the image move naturally, while the forecast data—the bold title—remains fixed."

// GenerateVideo generates a 9:16 video using Veo 3.1 Fast.
// Returns: GS URI (string) or error.
func (s *Service) GenerateVideo(ctx context.Context, inputImageURI string, prompt string) (string, error) {
	model := "veo-3.1-fast-generate-preview"
	
	if prompt == "" {
		prompt = DefaultVideoPrompt
	}

	log.Printf("Generating video with model %s. Input: %s", model, inputImageURI)

	// Construct the image object
	image := &genai.Image{
		GCSURI: inputImageURI,
		MIMEType: "image/png",
	}

	// Config
	config := &genai.GenerateVideosConfig{
		AspectRatio: "9:16",
		OutputGCSURI: fmt.Sprintf("gs://%s/videos/", s.bucketName),
	}

	// Call GenerateVideos
	resp, err := s.client.Models.GenerateVideos(ctx, model, prompt, image, config)
	if err != nil {
		log.Printf("GenAI GenerateVideos failed: %v", err)
		return "", fmt.Errorf("veo error: %w", err)
	}

	log.Printf("Veo operation started. ID: %s", resp.Name)

	// Polling Loop using Native SDK method
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled during polling")
		case <-ticker.C:
			// Use native SDK polling
			op, err := s.client.Operations.GetVideosOperation(ctx, resp, nil)
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
				
				// Hack: Marshal/Unmarshal to bypass unknown struct field name
				// The SDK is alpha and field names vary (GcsUri vs VideoUri vs Uri).
				b, _ := json.Marshal(v)
				var m map[string]interface{}
				_ = json.Unmarshal(b, &m)
				
				// Top level check
				uri, _ := m["gcsUri"].(string)
				if uri == "" { uri, _ = m["videoUri"].(string) }
				if uri == "" { uri, _ = m["uri"].(string) }

				// Nested check (video.uri) - This matches the logs!
				if uri == "" {
					if vid, ok := m["video"].(map[string]interface{}); ok {
						uri, _ = vid["uri"].(string)
						if uri == "" { uri, _ = vid["gcsUri"].(string) }
						if uri == "" { uri, _ = vid["videoUri"].(string) }
					}
				}

				if uri != "" {
					log.Printf("Video generated (GCS URI): %s", uri)
					return uri, nil
				}

				return "", fmt.Errorf("video generated but URI is empty (JSON: %s)", string(b))
			}
			log.Printf("Still polling Veo...")
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}
