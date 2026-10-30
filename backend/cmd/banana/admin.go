package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"banana-weather/pkg/config"
	"banana-weather/pkg/database"
	"banana-weather/pkg/genai"
	"banana-weather/pkg/storage"
	"banana-weather/pkg/weather"

	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:     "admin",
	GroupID: groupAdmin,
	Short:   "Inspect and maintain Firestore locations and media",
	Long: `Inspect aggregate Firestore collection statistics, list stored preset and user-generated
locations, or regenerate image and video assets for an existing location document.`,
	Example: `  banana admin stats
  banana admin stats --json
  banana admin list --type preset --limit 10 --json
  banana admin refresh --id london --style 1`,
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display aggregate Firestore location statistics",
	Long: `Query Firestore aggregation counts for total locations, curated presets, and
user-generated locations, along with the most recent update timestamp.`,
	Example: `  banana admin stats
  banana admin stats --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		ctx := cmd.Context()
		cfg, err := config.Load()
		if err != nil {
			printConfigHint()
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		db, err := database.NewClient(ctx, cfg.ProjectID, cfg.DatabaseID)
		if err != nil {
			return fmt.Errorf("failed to initialize Firestore client: %w", err)
		}
		defer db.Close()
		return runStats(ctx, db, asJSON)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored weather locations ordered by recent activity",
	Long: `List location documents from the Firestore "locations" collection ordered by
last_updated descending. Supports filtering by location type (all, preset, user)
and emitting structured JSON via --json.`,
	Example: `  banana admin list
  banana admin list --type preset --limit 10
  banana admin list --type user --limit 5 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		filterType, _ := cmd.Flags().GetString("type")
		asJSON, _ := cmd.Flags().GetBool("json")

		switch filterType {
		case "all", "preset", "user":
		default:
			fmt.Fprintln(os.Stderr, "Hint: valid values for --type are 'all', 'preset', or 'user'.")
			return fmt.Errorf("invalid --type %q (want all, preset, or user)", filterType)
		}

		ctx := cmd.Context()
		cfg, err := config.Load()
		if err != nil {
			printConfigHint()
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		db, err := database.NewClient(ctx, cfg.ProjectID, cfg.DatabaseID)
		if err != nil {
			return fmt.Errorf("failed to initialize Firestore client: %w", err)
		}
		defer db.Close()
		return runList(ctx, db, limit, filterType, asJSON)
	},
}

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Regenerate image and video media for an existing location",
	Long: `Fetch an existing location document by ID from Firestore, generate a fresh 9:16 image
and Veo parallax video, upload the assets to Cloud Storage, and update the Firestore record.`,
	Example: `  banana admin refresh --id london
  banana admin refresh --id tokyo --style 2
  banana admin refresh --id paris --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		style, _ := cmd.Flags().GetInt("style")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if err := weather.ValidateLocationID(id); err != nil {
			fmt.Fprintln(os.Stderr, "Hint: run 'banana admin list' to see available location IDs, then pass --id <id>.")
			return fmt.Errorf("invalid --id: %w", err)
		}

		ctx := cmd.Context()
		cfg, err := config.Load()
		if err != nil {
			printConfigHint()
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		db, err := database.NewClient(ctx, cfg.ProjectID, cfg.DatabaseID)
		if err != nil {
			return fmt.Errorf("failed to initialize Firestore client: %w", err)
		}
		defer db.Close()
		return runRefresh(ctx, db, id, style, dryRun, cfg)
	},
}

func init() {
	rootCmd.AddCommand(adminCmd)
	adminCmd.AddCommand(statsCmd)
	adminCmd.AddCommand(listCmd)
	adminCmd.AddCommand(refreshCmd)

	statsCmd.Flags().Bool("json", false, "Output statistics in JSON format")

	listCmd.Flags().Int("limit", 20, "Maximum number of locations to return")
	listCmd.Flags().String("type", "all", "Filter by location type: all, preset, user")
	listCmd.Flags().Bool("json", false, "Output locations in JSON format")

	refreshCmd.Flags().String("id", "", "Location ID to refresh (required)")
	refreshCmd.Flags().Int("style", 0, "Prompt style: 0=Random, 1=Classic, 2=Drink")
	refreshCmd.Flags().Bool("dry-run", false, "Validate location exists and preview refresh without calling Vertex AI or writing to GCS/Firestore")
}

func runStats(ctx context.Context, db *database.Client, asJSON bool) error {
	stats, err := db.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to query stats: %w", err)
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(stats)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Metric\tValue")
	fmt.Fprintln(w, "------\t-----")
	fmt.Fprintf(w, "Total Locations\t%d\n", stats.TotalLocations)
	fmt.Fprintf(w, "Presets\t%d\n", stats.Presets)
	fmt.Fprintf(w, "User Generated\t%d\n", stats.UserGenerated)
	fmt.Fprintf(w, "Last Activity\t%s (%s ago)\n", stats.LastUpdated.Format(time.RFC822), time.Since(stats.LastUpdated).Round(time.Second))
	return w.Flush()
}

func runList(ctx context.Context, db *database.Client, limit int, filterType string, asJSON bool) error {
	locs, err := db.ListLocations(ctx, limit, filterType)
	if err != nil {
		return fmt.Errorf("failed to list locations: %w", err)
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(locs)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tType\tCity\tUpdated")
	fmt.Fprintln(w, "--\t----\t----\t----\t-------")
	for _, l := range locs {
		sType := "User"
		if l.IsPreset {
			sType = "Preset"
		}
		city := l.CityQuery
		if len(city) > 30 {
			city = city[:27] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", l.ID, l.Name, sType, city, l.LastUpdated.Format("02 Jan 15:04"))
	}
	return w.Flush()
}

func runRefresh(ctx context.Context, db *database.Client, id string, style int, dryRun bool, cfg *config.Config) error {
	log.Printf("Refreshing location %q (style: %d)", id, style)
	loc, err := db.GetLocation(ctx, id)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Hint: run 'banana admin list' to verify the location ID exists in Firestore.")
		return fmt.Errorf("location %q not found: %w", id, err)
	}

	if dryRun {
		log.Printf("[dry-run] Would regenerate image (model=%s) and video (model=%s) for %q (%s)", cfg.GeminiImageModel, cfg.VeoVideoModel, loc.ID, loc.CityQuery)
		return nil
	}

	genaiService, err := genai.NewService(ctx, cfg.ProjectID, cfg.ImageLocation, cfg.VideoLocation, cfg.BucketName, cfg.GeminiImageModel, cfg.VeoVideoModel)
	if err != nil {
		return fmt.Errorf("genai init failed: %w", err)
	}
	storageService, err := storage.NewService(ctx, cfg.BucketName)
	if err != nil {
		return fmt.Errorf("storage init failed: %w", err)
	}

	log.Printf("Generating image for %q...", loc.CityQuery)
	imgBase64, err := genaiService.GenerateImage(ctx, loc.CityQuery, "", style)
	if err != nil {
		return fmt.Errorf("image generation failed: %w", err)
	}

	imgFileName := fmt.Sprintf("refresh_%s_image_%d.png", id, time.Now().Unix())
	gsImageURI, publicImageURL, err := storageService.UploadImage(ctx, imgBase64, imgFileName)
	if err != nil {
		return fmt.Errorf("image upload failed: %w", err)
	}
	log.Printf("Image uploaded: %s", publicImageURL)

	log.Printf("Generating video (Veo)...")
	videoGsURI, err := genaiService.GenerateVideo(ctx, gsImageURI, "")
	if err != nil {
		return fmt.Errorf("video generation failed: %w", err)
	}

	publicVideoURL := "https://storage.googleapis.com/" + strings.TrimPrefix(videoGsURI, "gs://")
	log.Printf("Video generated: %s", publicVideoURL)

	loc.ImageURL = publicImageURL
	loc.VideoURL = publicVideoURL
	loc.LastUpdated = time.Now()

	if err := db.UpsertLocation(ctx, *loc); err != nil {
		return fmt.Errorf("failed to update location %q in Firestore: %w", id, err)
	}
	log.Println("Refresh complete.")
	return nil
}
