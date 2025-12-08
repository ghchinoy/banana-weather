# Banana Weather: AI-Powered Atmospheric Art

*A GenMedia application that visualizes the current "vibe" of a city using Generative AI.*

## Project Overview
Banana Weather is a simple yet powerful application that demonstrates the "Oracle & Temple" architecture. It allows users to select a city and generates a high-fidelity, 9:16 artistic representation of that location's current essence using **Google Gemini 3 Pro Image (Nano Banana Pro)** via Vertex AI.

## Architecture

The system follows a client-server model:

1.  **The Backend (Go):** The orchestration layer.
    *   **Role:** API Server, Static File Host, and GenAI Orchestrator.
    *   **Responsibilities:**
        *   **Geocoding:** Google Maps API for location resolution and reverse geocoding.
        *   **Image Generation:** Vertex AI (`gemini-3-pro-image-preview`) using `GenerateContent`.
        *   **Serving:** Hosts the compiled Flutter Web application.

2.  **The Frontend (Flutter):** The user interface.
    *   **Role:** Web-based client (Single Page App).
    *   **Stack:** Flutter Web, `provider` for state management.
    *   **UI:**
        *   Full-screen 9:16 image display.
        *   Geolocation-based initial fetch.
        *   Manual city search overlay.

## Technology Stack

| Component | Technology | Key Libraries |
| :--- | :--- | :--- |
| **Backend** | Go 1.25+ | `chi`, `google.golang.org/genai`, `googlemaps.github.io/maps` |
| **Frontend** | Flutter (Dart) | `provider`, `google_fonts`, `geolocator` |
| **Cloud** | Google Cloud Run | Vertex AI, Maps Platform |

## Development Guide

### 1. Environment Variables
Create a `.env` file:

```bash
GOOGLE_CLOUD_PROJECT="<your-gcp-project>"
GOOGLE_CLOUD_LOCATION="global" # Required for Gemini 3 Pro Image
GOOGLE_MAPS_API_KEY="<your-maps-key>"
PORT=8080
```

### 2. Running Local (Dev)
Use the helper script to build frontend and run backend:
```bash
./dev.sh
```
*   `./dev.sh --quick` skips the Flutter build if only backend changes were made.

### 3. Deployment
Deploy to Google Cloud Run:
```bash
./deploy.sh
```

## Coding Conventions

### Task Management (`bd`)
We use the `bd` (Beads) tool for all issue tracking.

*   **Project Prefix:** `banana-weather`
*   **Workflow:**
    1.  **Create:** Use `bd create` for every unit of work.
    2.  **Implement:** Do the work.
    3.  **Close:** `bd close <id>` upon completion.
*   **Conventions:**
    *   **Descriptions:** **ALWAYS** include a `--description` explaining the "Why" and "What".
    *   **IDs & Parents:**
        *   Find IDs using `bd list`.
        *   Link child tasks to Epics using `--parent <epic-id>`.
        *   **Note:** If `bd` reports a "prefix mismatch" even when using the correct full ID (e.g., `banana-weather-123`), append the `--force` flag to bypass the check.
    *   **Priorities:**
        *   `P0`: Critical / Blocker.
        *   `P1`: Core Feature.
        *   `P2`: Enhancement / Polish.
    *   **Release Management:**
        *   **Changelog Generation:** Run this command to generate `CHANGELOG.md` from closed tasks:
            ```bash
            bd list --status closed --json | jq -r 'sort_by(.closed_at) | reverse | map(select(.closed_at != null)) | group_by(.closed_at[0:10]) | .[] | "## " + (.[0].closed_at[0:10]) + "\n" + (map("- " + .title + " (" + .id + ")") | join("\n")) + "\n"' > CHANGELOG.md
            ```

### Go (Backend)
*   **Structure:** `pkg/` for business logic (GenAI, Maps, Storage), `api/` for HTTP handlers.
*   **GenAI:**
    *   **Images:** Use `GenerateContent` (Multimodal API) for Gemini 3 Pro Image.
    *   **Videos:** Use `GenerateVideos` (Imagen API) for Veo 3.1.
    *   **LRO Polling:** Use `s.client.Operations.GetVideosOperation` (Native SDK) or `aiplatform` SDK if needed for robust polling.
    *   **Storage:** Use `OutputGCSURI` to write large assets directly to GCS. Use `pkg/storage` for manual uploads.
*   **Logging:** Use `log` for tracking request flows and errors.
*   **Static Serving:** Logic must support both local paths (`../frontend/build/web`) and Docker paths (`frontend/build/web`) for the frontend assets.

### Flutter (Frontend)
*   **State:** Use `Provider` pattern (`WeatherProvider`, `ThemeProvider`).
*   **API Calls:** Use relative paths (e.g., `/api/weather`) in Release mode.
    ```dart
    final String baseUrl = kDebugMode ? 'http://localhost:8080' : '';
    ```
*   **Video:** Use `VideoPlayerController.network` (String) for best Web compatibility. Always `setVolume(0.0)` to ensure auto-play works.
*   **UI:**
    *   **Non-Blocking:** Use `AnimatedOpacity` and Overlay Pills for status updates, keeping the main content visible.
    *   **Theme:** Support Light/Dark modes with `ThemeProvider`.
    *   **Style:** Clean, "Digital Picture Frame" aesthetic (no clutter).

### Tooling
*   **CLI Tools:** Use `cmd/banana` (Unified CLI) for admin tasks and content generation.
    *   **Generate Presets:** `go run cmd/banana/main.go generate ...`
    *   **Admin:** `go run cmd/banana/main.go admin ...`
    *   Supports CSV input (`-csv`) for bulk processing.
    *   Injects context (`-context`) for fictional locations.

## Deployment Strategy

*   **Build Process:** The frontend is built locally (`flutter build web`) by `deploy.sh` before deployment.
*   **Artifacts:** `.gcloudignore` is configured to ignore the frontend *source* but explicitly include the `frontend/build/web` artifacts.
*   **Containerization:** The `Dockerfile` copies the pre-built frontend assets into the Go server's container.
*   **Identity:** Use a dedicated Service Account with `aiplatform.user` and `storage.objectAdmin` roles.

### Troubleshooting
*   **500 Errors:** Check backend logs via Cloud Logging.
*   **"Model not found"::** Ensure `GOOGLE_CLOUD_LOCATION` is set to `global` for `gemini-3-pro-image-preview`.
*   **"Connection Refused":** Verify the Frontend is using relative URLs in production.
*   **"UnimplementedError: init()":** Run `flutter clean` and rebuild to fix Web Plugin registration issues.
*   **Video Not Playing:** Check CORS on the GCS bucket and ensure `setVolume(0.0)` is called.

## Technical Guidelines & Lessons Learned

### Google Cloud & GenAI
*   **LRO Polling:** For Publisher Models (Veo, Imagen), standard GenAI SDKs may miss polling logic for custom operation types. Use `google.golang.org/api/aiplatform/v1beta1` with the **Regional Endpoint** (`us-central1-aiplatform...`) for reliable operation tracking if the high-level SDK fails.
*   **Identities:** Always use a Service Account with Least Privilege. Document required roles (e.g. `datastore.user`, `aiplatform.user`) immediately in `setup_sa.sh`.

### Frontend/Backend Contract
*   **API Evolution:** If modifying an endpoint's response format (e.g. JSON fields), update the Frontend Model (`models/preset.dart`) and Provider (`weather_provider.dart`) **simultaneously** to prevent runtime breakages.
*   **Environment Awareness:** Use `kDebugMode` in Flutter to toggle between `localhost` (Dev) and Relative Paths (Production). Never hardcode IP addresses.

### Deployment
*   **Artifacts:** Use `.gcloudignore` to explicitly whitelist build outputs (`!frontend/build/web`) while ignoring source (`frontend/`) to optimize Cloud Build context.
*   **Clean Builds:** Always run `flutter clean` before a production build (`deploy.sh`) to ensure Web Plugins (`video_player`) register correctly.
*   **Visual Regression:** When implementing overlays or opacity animations, verify the "Loading" and "Empty" states to ensure background colors don't flash (e.g. enforce `Colors.black` container background).
