package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"banana-weather/pkg/config"
	"banana-weather/pkg/database"
	"banana-weather/pkg/genai"
	"banana-weather/pkg/storage"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate presets or single locations",
	Long:  "Generate weather presets from a CSV file or a single location via flags.",
	Run:   runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().String("csv", "", "Path to CSV file (format: id,name,city,category,context)")
	generateCmd.Flags().Bool("force", false, "Force overwrite existing presets")

	// Single mode flags
	generateCmd.Flags().String("city", "", "City name")
	generateCmd.Flags().String("context", "", "Extra prompt context")
	generateCmd.Flags().String("name", "", "Display name")
	generateCmd.Flags().String("category", "General", "Category name")
	generateCmd.Flags().String("id", "", "Unique ID")
	generateCmd.Flags().Int("style", 0, "Prompt Style: 0=Random, 1=Classic, 2=Drink")
}

func runGenerate(cmd *cobra.Command, args []string) {
	csvPath, _ := cmd.Flags().GetString("csv")
	force, _ := cmd.Flags().GetBool("force")
	
	ctx := context.Background()

	// Load Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Init Services
	genaiService, err := genai.NewService(ctx, cfg.ProjectID, cfg.Location, cfg.BucketName)
	if err != nil {
		log.Fatalf("Failed to init GenAI: %v", err)
	}
	storageService, err := storage.NewService(ctx, cfg.BucketName)
	if err != nil {
		log.Fatalf("Failed to init Storage: %v", err)
	}
	dbService, err := database.NewClient(ctx, cfg.ProjectID, cfg.DatabaseID)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer dbService.Close()

	if csvPath != "" {
		runBatchMode(ctx, csvPath, force, genaiService, storageService, dbService)
	} else {
		runSingleMode(ctx, cmd, force, genaiService, storageService, dbService)
	}

	log.Println("Done.")
}

func runBatchMode(ctx context.Context, csvPath string, force bool, gs *genai.Service, ss *storage.Service, db *database.Client) {
	log.Printf("Running in Batch Mode from %s (Force: %v)", csvPath, force)
	f, err := os.Open(csvPath)
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
		existing, err := db.GetLocation(ctx, pID)
		exists := err == nil && existing != nil

		if exists && !force {
			log.Printf("Skipping generation for [%s], updating metadata only.", pID)
			existing.Name = pName
			existing.Category = pCat
			existing.IsPreset = true
			if err := db.UpsertLocation(ctx, *existing); err != nil {
				log.Printf("Failed to patch %s: %v", pID, err)
			}
			continue
		}

		log.Printf("Processing [%d/%d]: %s (%s)", i, len(records)-1, pName, pID)
		// Batch mode defaults to Random (0) unless we add a column later
		imgURL, vidURL, err := processPreset(ctx, gs, ss, pID, pCity, pCtx, 0)
		if err != nil {
			log.Printf("Error processing %s: %v", pID, err)
			continue
		}

		loc := database.Location{
			ID:        pID,
			Name:      pName,
			Category:  pCat,
			CityQuery: pCity,
			ImageURL:  imgURL,
			VideoURL:  vidURL,
			IsPreset:  true,
		}
		if err := db.UpsertLocation(ctx, loc); err != nil {
			log.Printf("Failed to save %s: %v", pID, err)
		}
	}
}

func runSingleMode(ctx context.Context, cmd *cobra.Command, force bool, gs *genai.Service, ss *storage.Service, db *database.Client) {
	city, _ := cmd.Flags().GetString("city")
	ctxPrompt, _ := cmd.Flags().GetString("context")
	name, _ := cmd.Flags().GetString("name")
	category, _ := cmd.Flags().GetString("category")
	id, _ := cmd.Flags().GetString("id")
	style, _ := cmd.Flags().GetInt("style")

	if city == "" || name == "" || id == "" {
		fmt.Println("Usage: banana generate [flags]")
		fmt.Println("\nRequired flags for Single Mode:")
		fmt.Println("  --id       Unique identifier (e.g., 'my_preset')")
		fmt.Println("  --name     Display name (e.g., 'My Preset')")
		fmt.Println("  --city     City query or concept (e.g., 'Atlantis')")
		fmt.Println("\nOptional flags:")
		fmt.Println("  --category Grouping category (default: 'General')")
		fmt.Println("  --context  Visual description for fictional places")
		fmt.Println("  --style    Prompt Style: 0=Random, 1=Classic, 2=Drink (default: 0)")
		fmt.Println("  --force    Overwrite existing preset media")
		fmt.Println("\nOr use batch mode:")
		fmt.Println("  --csv      Path to CSV file")
		os.Exit(1)
	}

	existing, err := db.GetLocation(ctx, id)
	exists := err == nil && existing != nil

	if exists && !force {
		log.Printf("Skipping generation for [%s], updating metadata only.", id)
		existing.Name = name
		existing.Category = category
		existing.IsPreset = true
		if err := db.UpsertLocation(ctx, *existing); err != nil {
			log.Fatalf("Failed to patch %s: %v", id, err)
		}
	} else {
		imgURL, vidURL, err := processPreset(ctx, gs, ss, id, city, ctxPrompt, style)
		if err != nil {
			log.Fatalf("Error: %v", err)
		}
		loc := database.Location{
			ID:        id,
			Name:      name,
			Category:  category,
			CityQuery: city,
			ImageURL:  imgURL,
			VideoURL:  vidURL,
			IsPreset:  true,
		}
		if err := db.UpsertLocation(ctx, loc); err != nil {
			log.Fatalf("Failed to save: %v", err)
		}
	}
}

func processPreset(ctx context.Context, gs *genai.Service, ss *storage.Service, id, city, promptCtx string, style int) (string, string, error) {
	// 1. Generate Image
	log.Printf("Generating image for '%s' (Style: %d)...", city, style)
	imgBase64, err := gs.GenerateImage(ctx, city, promptCtx, style)
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
