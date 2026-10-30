package main

import (
	"encoding/json"
	"fmt"
	"log"

	"banana-weather/pkg/config"
	"banana-weather/pkg/database"
	"banana-weather/pkg/storage"
	"banana-weather/pkg/weather"

	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:     "migrate",
	GroupID: groupAdmin,
	Short:   "Migrate legacy presets.json from Cloud Storage to Firestore",
	Long: `Read the legacy presets.json manifest from the configured Cloud Storage bucket
and upsert each preset record into the Firestore "locations" collection.`,
	Example: `  banana migrate --dry-run
  banana migrate`,
	RunE: runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().Bool("dry-run", false, "Read and validate presets.json from GCS without writing to Firestore")
}

// LegacyPreset matches the JSON structure in presets.json.
type LegacyPreset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	ImageURL string `json:"image_url"`
	VideoURL string `json:"video_url"`
}

func runMigrate(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		printConfigHint()
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	storageService, err := storage.NewService(ctx, cfg.BucketName)
	if err != nil {
		return fmt.Errorf("failed to init Storage: %w", err)
	}

	log.Println("Reading presets.json from GCS...")
	data, err := storageService.ReadObject(ctx, "presets.json")
	if err != nil {
		return fmt.Errorf("failed to read presets.json: %w", err)
	}

	var legacyList []LegacyPreset
	if err := json.Unmarshal(data, &legacyList); err != nil {
		return fmt.Errorf("failed to parse presets.json: %w", err)
	}

	if dryRun {
		log.Printf("[dry-run] Parsed %d legacy presets from presets.json; skipping Firestore writes.", len(legacyList))
		return nil
	}

	dbService, err := database.NewClient(ctx, cfg.ProjectID, cfg.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to init DB: %w", err)
	}
	defer dbService.Close()

	log.Printf("Migrating %d presets to Firestore...", len(legacyList))

	for _, p := range legacyList {
		if err := weather.ValidateLocationID(p.ID); err != nil {
			log.Printf("Skipping invalid preset ID %q: %v", p.ID, err)
			continue
		}
		loc := database.Location{
			ID:        p.ID,
			Name:      p.Name,
			Category:  p.Category,
			CityQuery: p.Name,
			ImageURL:  p.ImageURL,
			VideoURL:  p.VideoURL,
			IsPreset:  true,
		}

		if loc.Category == "" {
			loc.Category = "General"
		}

		if err := dbService.UpsertLocation(ctx, loc); err != nil {
			log.Printf("Error migrating %q: %v", p.ID, err)
			continue
		}
		log.Printf("Migrated: %s", p.ID)
	}

	log.Println("Migration complete.")
	return nil
}
