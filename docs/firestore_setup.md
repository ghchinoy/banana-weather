# Firestore Database Setup

Banana Weather uses **Google Cloud Firestore** (Native Mode) for persistent storage of locations, presets, and caching metadata.

## Database Provisioning

1.  **Create Database:**
    *   Go to Google Cloud Console > Firestore.
    *   Create a database named `banana-weather` (or use `(default)` if preferred, but config must match).
    *   Select **Native Mode**.
    *   Region: `nam5` (us-central1) or closest to your Cloud Run service.

2.  **Environment Configuration:**
    Ensure your `.env` and `deploy.sh` have:
    ```bash
    FIRESTORE_DATABASE="banana-weather"
    ```

## Collection Schema

### `locations` (Collection)
Stores both Admin Presets and User-generated Locations.

**Document ID:**
*   **Presets:** Uses the unique ID from CSV (e.g., `arrakis_carthag`).
*   **User Locations:** Sanitized city name string (e.g., `fort_collins_co`).

**Fields:**
| Field | Type | Description |
| :--- | :--- | :--- |
| `id` | String | Matches Document ID. |
| `name` | String | Display name (e.g. "Fort Collins, CO"). |
| `city_query` | String | Original search query. |
| `category` | String | Grouping (e.g., "Dune Universe", "General"). |
| `image_url` | String | Public GCS URL for the generated image. |
| `video_url` | String | Public GCS URL for the generated video. |
| `is_preset` | Boolean | `true` if Admin-managed/Gallery item. `false` if User-generated cache. |
| `last_updated`| Timestamp | Used for TTL Caching (re-generate if > 3h old). |

## Indexes
Standard single-field indexes are sufficient for current queries (`GetLocation` by ID, `GetPresets` filter by `is_preset`).

## Security Rules (If interacting from Client SDK)
*Currently, the Go Backend uses the Admin SDK, which bypasses rules. If Client SDK access is added later, restrict write access to Auth users only.*
