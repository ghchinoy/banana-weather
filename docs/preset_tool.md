# Preset Generator Tool

## Overview
The `generate_preset` tool allows administrators to pre-generate content (Image + Video) for specific locations—real or fictional—and add them to the application's global `presets.json` registry. This enables the "Gallery" feature in the frontend.

## Usage

Run the tool from the `backend/` directory. Ensure your `.env` file is present in the project root.

### 1. Single Mode (Add One Preset)
Best for adding a new location quickly or testing a prompt.

```bash
# Real Location
go run cmd/generate_preset/main.go -id "paris" -name "Paris, France" -city "Paris" -category "Europe"

# Fictional Location (using -context)
go run cmd/generate_preset/main.go \
  -id "atlantis" \
  -name "Atlantis" \
  -city "Atlantis" \
  -category "Fantasy" \
  -context "A high-tech city submerged underwater, bioluminescent lights, ancient greek architecture mixed with sci-fi."
```

### 2. Batch Mode (CSV)
Best for bulk updates.

```bash
go run cmd/generate_preset/main.go -csv presets_expanded.csv
```

### Flags

| Flag | Description | Required (Single Mode) | Example |
| :--- | :--- | :--- | :--- |
| `-csv` | Path to a CSV file for batch processing. | No | `presets_expanded.csv` |
| `-force` | Overwrite existing presets with the same ID. If false, it only patches metadata. | No | `true` |
| `-city` | The query passed to the prompt. | **Yes** | `"Carthage, Arrakis"` |
| `-context` | Additional visual context for the AI. Use this for fictional places. | No | `"Dune universe..."` |
| `-name` | The human-readable display name. | **Yes** | `"Arrakis (Dune)"` |
| `-category` | The category for grouping in the drawer. Creates a new group if unique. | No | `"Dune Universe"` |
| `-id` | A unique identifier for the preset. | **Yes** | `"arrakis"` |

### CSV Format
The tool expects a CSV file with the following header:
`id,name,city,category,context`

Example:
```csv
id,name,city,category,context
tatooine_mos_eisley,"Mos Eisley",Tatooine,Star Wars,"Desert spaceport, twin suns"
winterfell,"Winterfell",Winterfell,Game of Thrones,"Snowy castle, Stark"
```

## Workflow

1.  **Init:** Connects to Vertex AI, Firestore, and GCS using credentials from `.env`.
2.  **Check Registry:** Checks `Firestore` for existing location ID.
    *   If ID exists and `-force` is false: Updates Metadata (Name, Category) but **skips generation**.
3.  **Generate Image:** Calls Gemini 3 Pro Image with the city and context.
4.  **Upload:** Saves the PNG to GCS.
5.  **Generate Video:** Calls Veo 3.1 Fast with the GCS Image URI.
6.  **Output:** Veo writes the video directly to GCS.
7.  **Registry Update:**
    *   Upserts the Location document in **Firestore**.