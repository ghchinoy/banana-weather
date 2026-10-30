// Package main provides the unified banana CLI for managing Banana Weather presets, media, and Firestore data.
package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

const (
	groupContent = "content"
	groupAdmin   = "admin"
)

var rootCmd = &cobra.Command{
	Use:   "banana",
	Short: "Manage Banana Weather presets, media generation, and Firestore data",
	Long: `banana is the unified administrative and content-generation CLI for Banana Weather.

It supports generating single or batch 9:16 weather scenes via Vertex AI (Gemini Image + Veo),
inspecting Firestore statistics and location documents (with --json output for agents),
refreshing cached media, and migrating legacy preset definitions.`,
	Example: `  # View database statistics as JSON
  banana admin stats --json

  # List the 10 most recently updated preset locations
  banana admin list --type preset --limit 10

  # Generate a single preset location
  banana generate --id tokyo --name "Tokyo, Japan" --city "Tokyo" --style 2

  # Preview batch preset generation without mutating GCS or Firestore
  banana generate --csv presets_expanded.csv --dry-run`,
	SilenceUsage: true,
}

func init() {
	rootCmd.AddGroup(
		&cobra.Group{
			ID:    groupContent,
			Title: "Content Generation Commands:",
		},
		&cobra.Group{
			ID:    groupAdmin,
			Title: "Database & Administration Commands:",
		},
	)
}

// Execute runs the root Cobra command and exits with status 1 on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func printConfigHint() {
	fmt.Fprintln(os.Stderr, "Hint: ensure GOOGLE_CLOUD_PROJECT, GENMEDIA_BUCKET, and GOOGLE_MAPS_API_KEY are set in .env or your environment.")
}

func main() {
	// Load .env files
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	Execute()
}
