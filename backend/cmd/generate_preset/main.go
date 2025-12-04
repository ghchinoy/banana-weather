package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"banana-weather/pkg/database"
	"banana-weather/pkg/genai"
	"banana-weather/pkg/storage"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	_ = godotenv.Load("../../.env") 
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	csvPath := flag.String("csv", "", "Path to CSV file (format: id,name,city,category,context)")
	force := flag.Bool("force", false, "Force overwrite existing presets")
	
	// Single mode flags
	city := flag.String("city", "", "City name")
	ctxPrompt := flag.String("context", "", "Extra prompt context")
	name := flag.String("name", "", "Display name")
	category := flag.String("category", "General", "Category name")
	id := flag.String("id", "", "Unique ID")
	
	flag.Parse()

	ctx := context.Background()

	// Init Services
	genaiService, err := genai.NewService(ctx)
	if err != nil {
		log.Fatalf("Failed to init GenAI: %v", err)
	}
	storageService, err := storage.NewService(ctx)
	if err != nil {
		log.Fatalf("Failed to init Storage: %v", err)
	}
	dbService, err := database.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer dbService.Close()

	if *csvPath != "" {
		// Batch Mode
		log.Printf("Running in Batch Mode from %s (Force: %v)", *csvPath, *force)
		f, err := os.Open(*csvPath)
		if err != nil {
			log.Fatalf("Failed to open CSV: %v", err)
		}
		defer f.Close()

		reader := csv.NewReader(f)
		records, err := reader.ReadAll()
		if err != nil {
			log.Fatalf("Failed to read CSV: %v", err)
		}

		for i, row := range records {
			if i == 0 { continue } // Skip Header
			if len(row) < 4 { continue }
			
			pID := row[0]
			pName := row[1]
			pCity := row[2]
			pCat := row[3]
			pCtx := ""
			if len(row) > 4 { pCtx = row[4] }

			// Check Existing
			existing, err := dbService.GetLocation(ctx, pID)
			exists := err == nil && existing != nil

			if exists && !*force {
				log.Printf("Skipping generation for [%s], updating metadata only.", pID)
				// Patch metadata
				existing.Name = pName
				existing.Category = pCat
				existing.IsPreset = true
				// Preserve URLs
				if err := dbService.UpsertLocation(ctx, *existing); err != nil {
					log.Printf("Failed to patch %s: %v", pID, err)
				}
				continue
			}

			log.Printf("Processing [%d/%d]: %s (%s)", i, len(records)-1, pName, pID)
			imgURL, vidURL, err := processPreset(ctx, genaiService, storageService, pID, pCity, pCtx)
			if err != nil {
				log.Printf("Error processing %s: %v", pID, err)
				continue
			}
			
			// Save to DB
			loc := database.Location{
				ID:        pID,
				Name:      pName,
				Category:  pCat,
				CityQuery: pCity,
				ImageURL:  imgURL,
				VideoURL:  vidURL,
				IsPreset:  true,
			}
			if err := dbService.UpsertLocation(ctx, loc); err != nil {
				log.Printf("Failed to save %s: %v", pID, err)
			}
		}

	} else {
		// Single Mode
		if *city == "" || *name == "" || *id == "" {
			log.Fatal("Missing required flags: -city, -name, -id (or -csv)")
		}
		
		existing, err := dbService.GetLocation(ctx, *id)
		exists := err == nil && existing != nil

		if exists && !*force {
			log.Printf("Skipping generation for [%s], updating metadata only.", *id)
			existing.Name = *name
			existing.Category = *category
			existing.IsPreset = true
			if err := dbService.UpsertLocation(ctx, *existing); err != nil {
				log.Fatalf("Failed to patch %s: %v", *id, err)
			}
		} else {
			imgURL, vidURL, err := processPreset(ctx, genaiService, storageService, *id, *city, *ctxPrompt)
			if err != nil {
				log.Fatalf("Error: %v", err)
			}
			loc := database.Location{
				ID:        *id,
				Name:      *name,
				Category:  *category,
				CityQuery: *city,
				ImageURL:  imgURL,
				VideoURL:  vidURL,
				IsPreset:  true,
			}
			if err := dbService.UpsertLocation(ctx, loc); err != nil {
				log.Fatalf("Failed to save: %v", err)
			}
		}
	}
	
	log.Println("Done.")
}

func processPreset(ctx context.Context, gs *genai.Service, ss *storage.Service, id, city, promptCtx string) (string, string, error) {
	// 1. Generate Image
	log.Printf("Generating image for '%s'...", city)
	imgBase64, err := gs.GenerateImage(ctx, city, promptCtx)
	if err != nil {
		return "", "", fmt.Errorf("image gen failed: %w", err)
	}

	// 2. Upload Image
	imgFileName := fmt.Sprintf("preset_%s_image_%d.png", id, time.Now().Unix())
	gsImageURI, publicImageURL, err := ss.UploadImage(ctx, imgBase64, imgFileName)
	if err != nil {
		return "", "", fmt.Errorf("image upload failed: %w", err)
	}
	log.Printf("Image uploaded: %s", publicImageURL)

	// 3. Generate Video
	log.Printf("Generating video (Veo)...")
	videoGsURI, err := gs.GenerateVideo(ctx, gsImageURI, "")
	if err != nil {
		return "", "", fmt.Errorf("video gen failed: %w", err)
	}
	
	bucketName := os.Getenv("GENMEDIA_BUCKET")
	publicVideoURL := strings.Replace(videoGsURI, "gs://"+bucketName, "https://storage.googleapis.com/"+bucketName, 1)
	log.Printf("Video generated: %s", publicVideoURL)

	return publicImageURL, publicVideoURL, nil
}
