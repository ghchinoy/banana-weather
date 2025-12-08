package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"banana-weather/pkg/database"
	"banana-weather/pkg/genai"
	"banana-weather/pkg/storage"

	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative tasks",
	Long:  "Commands for managing the database, presets, and media.",
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show database statistics",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		db, err := database.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to init DB: %v", err)
		}
		defer db.Close()
		runStats(ctx, db)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List locations",
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		filterType, _ := cmd.Flags().GetString("type")

		ctx := context.Background()
		db, err := database.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to init DB: %v", err)
		}
		defer db.Close()
		runList(ctx, db, limit, filterType)
	},
}

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh a location's media",
	Run: func(cmd *cobra.Command, args []string) {
		id, _ := cmd.Flags().GetString("id")
		style, _ := cmd.Flags().GetInt("style")
		if id == "" {
			log.Fatal("id is required (use --id)")
		}

		ctx := context.Background()
		db, err := database.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to init DB: %v", err)
		}
		defer db.Close()
		runRefresh(ctx, db, id, style)
	},
}

func init() {
	rootCmd.AddCommand(adminCmd)
	adminCmd.AddCommand(statsCmd)
	adminCmd.AddCommand(listCmd)
	adminCmd.AddCommand(refreshCmd)

	listCmd.Flags().Int("limit", 20, "Max number of results")
	listCmd.Flags().String("type", "all", "Filter by type: all, preset, user")

	refreshCmd.Flags().String("id", "", "Location ID to refresh")
	refreshCmd.Flags().Int("style", 0, "Prompt Style: 0=Random, 1=Classic, 2=Drink")
}

func runStats(ctx context.Context, db *database.Client) {
	fmt.Println("Fetching stats...")
	stats, err := db.GetStats(ctx)
	if err != nil {
		log.Fatalf("Error getting stats: %v", err)
	}
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Metric\tValue")
	fmt.Fprintln(w, "------\t-----")
	fmt.Fprintf(w, "Total Locations\t%d\n", stats.TotalLocations)
	fmt.Fprintf(w, "Presets\t%d\n", stats.Presets)
	fmt.Fprintf(w, "User Generated\t%d\n", stats.UserGenerated)
	fmt.Fprintf(w, "Last Activity\t%s (%s ago)\n", stats.LastUpdated.Format(time.RFC822), time.Since(stats.LastUpdated).Round(time.Second))
	w.Flush()
}

func runList(ctx context.Context, db *database.Client, limit int, filterType string) {
	fmt.Printf("Listing top %d locations (type: %s)...\n", limit, filterType)
	locs, err := db.ListLocations(ctx, limit, filterType)
	if err != nil {
		log.Fatalf("Error listing locations: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tType\tCity\tUpdated")
	fmt.Fprintln(w, "--\t----\t----\t----\t-------")
	for _, l := range locs {
		sType := "User"
		if l.IsPreset { sType = "Preset" }
		// Truncate city if too long
		city := l.CityQuery
		if len(city) > 30 { city = city[:27] + "..." }
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", l.ID, l.Name, sType, city, l.LastUpdated.Format("02 Jan 15:04"))
	}
	w.Flush()
}

func runRefresh(ctx context.Context, db *database.Client, id string, style int) {
	log.Printf("Refreshing location: %s (Style: %d)", id, style)
	loc, err := db.GetLocation(ctx, id)
	if err != nil {
		log.Fatalf("Location not found: %v", err)
	}

	genaiService, err := genai.NewService(ctx)
	if err != nil { log.Fatalf("GenAI init failed: %v", err) }
	storageService, err := storage.NewService(ctx)
	if err != nil { log.Fatalf("Storage init failed: %v", err) }

	log.Printf("Generating image for '%s'...", loc.CityQuery)
	imgBase64, err := genaiService.GenerateImage(ctx, loc.CityQuery, "", style)
	if err != nil {
		log.Fatalf("Image gen failed: %v", err)
	}

	imgFileName := fmt.Sprintf("refresh_%s_image_%d.png", id, time.Now().Unix())
	gsImageURI, publicImageURL, err := storageService.UploadImage(ctx, imgBase64, imgFileName)
	if err != nil {
		log.Fatalf("Image upload failed: %v", err)
	}
	log.Printf("Image uploaded: %s", publicImageURL)

	log.Printf("Generating video (Veo)...")
	videoGsURI, err := genaiService.GenerateVideo(ctx, gsImageURI, "")
	if err != nil {
		log.Fatalf("Video gen failed: %v", err)
	}
	
bucketName := os.Getenv("GENMEDIA_BUCKET")
	publicVideoURL := strings.Replace(videoGsURI, "gs://"+bucketName, "https://storage.googleapis.com/"+bucketName, 1)
	log.Printf("Video generated: %s", publicVideoURL)

	// Update DB
	loc.ImageURL = publicImageURL
	loc.VideoURL = publicVideoURL
	loc.LastUpdated = time.Now()
	
	if err := db.UpsertLocation(ctx, *loc); err != nil {
		log.Fatalf("Failed to update DB: %v", err)
	}
	log.Println("Refresh Complete.")
}
