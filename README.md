# Banana Weather: AI-Powered Atmospheric Art

![Banana Weather Banner](frontend/assets/images/placeholder_vertical.png)

**Banana Weather** is a GenMedia web application that visualizes the current "vibe" and atmospheric essence of a location using Generative AI. 

It combines precise Geolocation with the creative power of **Google Gemini 3 Pro Image** (Nano Banana Pro) and **Google Veo 3.1** to generate high-fidelity, vertical (9:16) 3D isometric art and looping videos representing the weather, architecture, and mood of your city in real-time.

## Features

*   **AI-Generated Atmospheric Art:** Unique, non-deterministic visuals for every request.
*   **Cinematic Video Loops:** Transitions from static image to a "Parallax" animation using **Veo 3.1 Fast**.
*   **Smart Caching:** Reuses generated content for 3 hours to improve performance and reduce costs.
*   **Video Download:** Download your generated animations as MP4 files with timestamped filenames.
*   **Fictional Locations:** Supports generating scenes for fictional worlds (e.g., Arrakis, Middle-earth) via the Presets system.
*   **Presets Gallery:** A curated list of pre-generated scenes categorized by theme, backed by **Firestore**.
*   **Responsive Flutter Web UI:** Mobile-first design with a clean, "Digital Picture Frame" aesthetic.

## What to Expect

1.  **Instant Immersion:** Upon opening the app, it attempts to detect your location and immediately conjures an atmospheric portrait of your current weather and surroundings.
2.  **The "Dreaming" Phase:** When you request a new city, the AI takes a moment (typically 10-20 seconds) to "dream" up the visuals. You'll see a status indicator as it writes the prompt, paints the image, and finally animates the scene.
3.  **Explore the Impossible:** Use the "Presets" drawer to visit fictional locations like *Arrakis*, *Winterfell*, or *Coruscant*, rendered with the same realistic fidelity as real-world cities.

## Architecture

The system follows a Client-Server architecture:

1.  **The Backend (Go 1.25):** 
    *   **Orchestrator:** Handles API requests, validating inputs.
    *   **Geocoding:** Interacts with Google Maps Platform.
    *   **Generative AI:** Gemini 3 Pro Image & Veo 3.1.
    *   **Persistence:** Uses **Firestore** for metadata/caching and **Cloud Storage** for assets.
    *   **Server:** Serves the compiled Flutter Web application.

2.  **The Frontend (Flutter Web):**
    *   **UI:** Single-page app using `Provider`.
    *   **Video:** `video_player` integration with seamless transition.
    *   **Theme:** Light/Dark mode with dynamic "Glassmorphism" overlays.

## Setup & Deployment

### 1. Prerequisites
*   **Google Cloud Project:** [Create a project](https://console.cloud.google.com/) and enable billing.
*   **APIs Enabled:**
    *   [Vertex AI API](https://console.cloud.google.com/vertex-ai) (for Gemini & Veo).
    *   [Google Maps Platform](https://developers.google.com/maps) (Places API, Geocoding API).
    *   [Cloud Run](https://cloud.google.com/run).
*   **Firestore Database:** Provisioned in **Native Mode**. [Quickstart Guide](https://cloud.google.com/firestore/docs/create-database).
*   **GCS Bucket:** A Standard storage bucket for hosting generated media. [Create Bucket Guide](https://cloud.google.com/storage/docs/creating-buckets).

### 2. Service Account Setup
Run the helper script to create the identity and grant permissions (Vertex AI, Logging, Firestore):
```bash
./setup_sa.sh
```

### 3. Environment Configuration
Create a `.env` file:

```bash
GOOGLE_CLOUD_PROJECT="your-gcp-project-id"
GOOGLE_CLOUD_LOCATION="global" 
GOOGLE_MAPS_API_KEY="your-maps-api-key"
GENMEDIA_BUCKET="your-gcs-bucket-name"
FIRESTORE_DATABASE="banana-weather"
PORT=8080
```

### 3. Development
*   **Run Local:** `./dev.sh`
*   **Deploy:** `./deploy.sh`

### 4. Tools
**Preset Generator:**
Populate the gallery with pre-generated content.

```bash
cd backend
go run cmd/generate_preset/main.go -csv presets_expanded.csv
```

**Migration:**
Move from JSON to Firestore (One-time).
```bash
go run cmd/migrate_presets/main.go
```

**Admin Console:**
View stats and manage locations.
```bash
go run cmd/admin/main.go stats
```


