package main

import (
	"context"
	"encoding/json"
	"log"

	"banana-weather/pkg/config"
	"banana-weather/pkg/database"
	"banana-weather/pkg/storage"

	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate legacy presets",
	Long:  "Migrate legacy presets.json from GCS to Firestore.",
	Run:   runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

// LegacyPreset matches the JSON structure in presets.json
type LegacyPreset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	ImageURL string `json:"image_url"`
	VideoURL string `json:"video_url"`
}

func runMigrate(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	cfg, _ := config.Load()
	if cfg == nil { log.Fatal("Config load failed") }

	// Init Services
	storageService, err := storage.NewService(ctx, cfg.BucketName)
	if err != nil {
		log.Fatalf("Failed to init Storage: %v", err)
	}
	
	dbService, err := database.NewClient(ctx, cfg.ProjectID, cfg.DatabaseID)
	if err != nil {
		log.Fatalf("Failed to init DB: %v", err)
	}
	defer dbService.Close()

	log.Println("Reading presets.json from GCS...")
	data, err := storageService.ReadObject(ctx, "presets.json")
	if err != nil {
		log.Fatalf("Failed to read presets.json: %v", err)
	}

	var legacyList []LegacyPreset
	if err := json.Unmarshal(data, &legacyList); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	log.Printf("Migrating %d presets to Firestore...", len(legacyList))

	for _, p := range legacyList {
		loc := database.Location{
			ID:        p.ID,
			Name:      p.Name,
			Category:  p.Category,
			CityQuery: p.Name, // Best guess for now, or could parse if stored separately
			ImageURL:  p.ImageURL,
			VideoURL:  p.VideoURL,
			IsPreset:  true,
		}
		
		// Fallback category if empty (older presets)
		if loc.Category == "" {
			loc.Category = "General"
		}

		if err := dbService.UpsertLocation(ctx, loc); err != nil {
			log.Printf("Error migrating %s: %v", p.ID, err)
		} else {
			log.Printf("Migrated: %s", p.ID)
		}
	}

	log.Println("Migration Complete.")
}
