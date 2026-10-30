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
	"banana-weather/pkg/weather"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:     "generate",
	GroupID: groupContent,
	Short:   "Generate preset weather art and video from flags or CSV",
	Long: `Generate 9:16 weather art (Gemini Image) and parallax video loops (Veo) for either
a single preset location specified via flags (--id, --name, --city) or a batch CSV file (--csv).
Existing presets are skipped unless --force is supplied.`,
	Example: `  # Generate a single location preset using the Drink prompt style
  banana generate --id london --name "London, UK" --city "London" --style 2

  # Generate a fictional preset with custom visual context
  banana generate --id arrakis --name "Arrakis" --city "Arrakeen" --category "Sci-Fi" --context "Vast desert dunes with twin moons"

  # Process a batch CSV file (preview first with --dry-run)
  banana generate --csv presets_expanded.csv --dry-run
  banana generate --csv presets_expanded.csv --force`,
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().String("csv", "", "Path to CSV file (format: id,name,city,category,context)")
	generateCmd.Flags().Bool("force", false, "Force overwrite existing preset media")
	generateCmd.Flags().Bool("dry-run", false, "Validate inputs and preview generation plan without calling Vertex AI or writing to GCS/Firestore")

	// Single mode flags
	generateCmd.Flags().String("city", "", "City name or concept to render")
	generateCmd.Flags().String("context", "", "Extra visual prompt context (e.g. for fictional places)")
	generateCmd.Flags().String("name", "", "Display name for the preset")
	generateCmd.Flags().String("category", "General", "Grouping category name")
	generateCmd.Flags().String("id", "", "Unique document and asset ID")
	generateCmd.Flags().Int("style", 0, "Prompt style: 0=Random, 1=Classic, 2=Drink")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	csvPath, _ := cmd.Flags().GetString("csv")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	id, _ := cmd.Flags().GetString("id")
	city, _ := cmd.Flags().GetString("city")
	name, _ := cmd.Flags().GetString("name")

	if csvPath != "" && (id != "" || city != "" || name != "") {
		fmt.Fprintln(os.Stderr, "Hint: use either '--csv <file>' for batch mode OR '--id/--name/--city' for single mode, not both.")
		return fmt.Errorf("cannot specify both --csv and single-location flags (--id, --name, --city)")
	}

	if csvPath == "" && (city == "" || name == "" || id == "") {
		fmt.Fprintln(os.Stderr, "Hint: pass --id, --name, and --city for single mode, or --csv <path> for batch mode. Run 'banana generate --help' for examples.")
		return fmt.Errorf("missing required flags: specify --csv or all of (--id, --name, --city)")
	}

	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		printConfigHint()
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if dryRun {
		if csvPath != "" {
			return previewBatchCSV(csvPath)
		}
		if err := weather.ValidateLocationID(id); err != nil {
			return err
		}
		log.Printf("[dry-run] Would generate preset id=%q name=%q city=%q category=%q (imageModel=%s, videoModel=%s, force=%v)",
			id, name, city, cmd.Flag("category").Value.String(), cfg.GeminiImageModel, cfg.VeoVideoModel, force)
		return nil
	}

	genaiService, err := genai.NewService(ctx, cfg.ProjectID, cfg.ImageLocation, cfg.VideoLocation, cfg.BucketName, cfg.GeminiImageModel, cfg.VeoVideoModel)
	if err != nil {
		return fmt.Errorf("failed to init GenAI: %w", err)
	}
	storageService, err := storage.NewService(ctx, cfg.BucketName)
	if err != nil {
		return fmt.Errorf("failed to init Storage: %w", err)
	}
	dbService, err := database.NewClient(ctx, cfg.ProjectID, cfg.DatabaseID)
	if err != nil {
		return fmt.Errorf("failed to init DB: %w", err)
	}
	defer dbService.Close()

	if csvPath != "" {
		if err := runBatchMode(ctx, csvPath, force, genaiService, storageService, dbService); err != nil {
			return err
		}
	} else {
		if err := runSingleMode(ctx, cmd, force, genaiService, storageService, dbService); err != nil {
			return err
		}
	}

	log.Println("Done.")
	return nil
}

func previewBatchCSV(csvPath string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV %q: %w", csvPath, err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse CSV %q: %w", csvPath, err)
	}
	count := 0
	for i, row := range records {
		if i == 0 || len(row) < 4 {
			continue
		}
		if err := weather.ValidateLocationID(row[0]); err != nil {
			return fmt.Errorf("row %d: %w", i+1, err)
		}
		count++
	}
	log.Printf("[dry-run] Validated %d preset rows in %q.", count, csvPath)
	return nil
}

func runBatchMode(ctx context.Context, csvPath string, force bool, gs *genai.Service, ss *storage.Service, db *database.Client) error {
	log.Printf("Running in Batch Mode from %q (force: %v)", csvPath, force)
	f, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("failed to open CSV %q: %w", csvPath, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV %q: %w", csvPath, err)
	}

	for i, row := range records {
		if i == 0 {
			continue // Skip Header
		}
		if len(row) < 4 {
			continue
		}

		pID := row[0]
		pName := row[1]
		pCity := row[2]
		pCat := row[3]
		var pCtx string
		if len(row) > 4 {
			pCtx = row[4]
		}

		if err := weather.ValidateLocationID(pID); err != nil {
			log.Printf("Skipping invalid ID %q on row %d: %v", pID, i+1, err)
			continue
		}

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
	return nil
}

func runSingleMode(ctx context.Context, cmd *cobra.Command, force bool, gs *genai.Service, ss *storage.Service, db *database.Client) error {
	city, _ := cmd.Flags().GetString("city")
	ctxPrompt, _ := cmd.Flags().GetString("context")
	name, _ := cmd.Flags().GetString("name")
	category, _ := cmd.Flags().GetString("category")
	id, _ := cmd.Flags().GetString("id")
	style, _ := cmd.Flags().GetInt("style")

	if err := weather.ValidateLocationID(id); err != nil {
		return err
	}

	existing, err := db.GetLocation(ctx, id)
	exists := err == nil && existing != nil

	if exists && !force {
		log.Printf("Skipping generation for [%s], updating metadata only.", id)
		existing.Name = name
		existing.Category = category
		existing.IsPreset = true
		if err := db.UpsertLocation(ctx, *existing); err != nil {
			return fmt.Errorf("failed to patch %q: %w", id, err)
		}
		return nil
	}

	imgURL, vidURL, err := processPreset(ctx, gs, ss, id, city, ctxPrompt, style)
	if err != nil {
		return err
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
		return fmt.Errorf("failed to save %q: %w", id, err)
	}
	return nil
}

func processPreset(ctx context.Context, gs *genai.Service, ss *storage.Service, id, city, promptCtx string, style int) (string, string, error) {
	// 1. Generate Image
	log.Printf("Generating image for %q (style: %d)...", city, style)
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

	publicVideoURL := "https://storage.googleapis.com/" + strings.TrimPrefix(videoGsURI, "gs://")
	log.Printf("Video generated: %s", publicVideoURL)

	return publicImageURL, publicVideoURL, nil
}
