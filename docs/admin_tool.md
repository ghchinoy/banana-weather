# Admin CLI Tool

## Overview
The `admin` tool provides operational insights and management capabilities for the Banana Weather application. It allows administrators to view statistics, list stored locations, and manually refresh specific entries.

## Usage

Run the tool from the `backend/` directory.

```bash
go run cmd/admin/main.go <subcommand> [flags]
```

### Subcommands

#### 1. `stats`
Displays aggregate statistics about the system.

```bash
go run cmd/admin/main.go stats
```

**Output:**
*   Total Locations
*   Preset Count
*   User Generated Count
*   Last Activity Timestamp

#### 2. `list`
Lists the most recently updated locations.

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-limit` | Maximum number of results to show. Set to `0` for all. | `20` |
| `-type` | Filter by type: `all`, `preset`, `user`. | `all` |

```bash
go run cmd/admin/main.go list -limit 10 -type user
```
```bash
go run cmd/admin/main.go list -limit 0 # Show all
```

#### 3. `refresh`
Regenerates the media (Image + Video) for a specific location ID. Useful for fixing broken generations or manually updating a user-requested location.

| Flag | Description | Required? |
| :--- | :--- | :--- |
| `-id` | The Location ID to refresh. | Yes |

```bash
go run cmd/admin/main.go refresh -id "san_francisco__ca"
```

**Note:** This triggers calls to Vertex AI (Gemini 3 Pro Image & Veo 3.1), incurring costs.
