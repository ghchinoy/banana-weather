# Banana Weather CLI

The `banana` CLI is the unified administrative tool for the Banana Weather backend. It consolidates previous utilities (preset generation, admin stats, migrations) into a single, cohesive interface.

## Installation

Build the binary from the `backend` directory:

```bash
cd backend
go build -o banana ./cmd/banana
```

## Usage

### Global Flags
The tool loads configuration from `.env` files automatically. Ensure you have a `.env` file in your project root or backend directory.

### Commands

#### 1. Generate Content (`generate`)
Generate presets or create single location content (Image + Video).

**Usage:**
```bash
./banana generate [flags]
```

**Flags:**
*   `--csv`: Path to CSV file for batch processing.
*   `--id`: Unique ID (e.g., `paris`).
*   `--name`: Display Name (e.g., `Paris, France`).
*   `--city`: City query for the prompt (e.g., `Paris`).
*   `--style`: Prompt Style (0=Random, 1=Classic, 2=Drink).
*   `--force`: Overwrite existing presets.

**Examples:**
```bash
# Batch mode
./banana generate --csv presets_expanded.csv

# Single mode (Drink Style)
./banana generate --id "london" --name "London" --city "London" --style 2
```

#### 2. Admin Tasks (`admin`)
Manage the running system.

**Subcommands:**
*   `stats`: Show database statistics (Total locations, presets, last activity).
*   `list`: List top locations.
    *   `--limit`: Max results (default 20).
    *   `--type`: Filter (`all`, `preset`, `user`).
*   `refresh`: Re-generate media for a specific location ID.
    *   `--id`: Location ID.
    *   `--style`: Prompt Style (0=Random, 1=Classic, 2=Drink).

**Example:**
```bash
./banana admin stats
./banana admin refresh --id "london"
```

#### 3. Database Migration (`migrate`)
Migrates legacy `presets.json` data from GCS to the Firestore database.

**Usage:**
```bash
./banana migrate
```
