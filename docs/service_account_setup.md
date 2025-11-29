# Service Account Setup & Usage for Banana Weather

This guide explains how to create a dedicated Service Account (SA) with the principle of Least Privilege for the Banana Weather application and configure `deploy.sh` to use it.

## 1. Create the Service Account

Run the following commands in your terminal (ensure you are authenticated with `gcloud` and have the correct project selected):

```bash
# Set variables
export SERVICE_NAME="banana-weather"
export PROJECT_ID=$(gcloud config get-value project)
export SA_NAME="${SERVICE_NAME}-sa"
export SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

# Create the Service Account
gcloud iam service-accounts create "$SA_NAME" \
    --display-name="Service Account for $SERVICE_NAME"
```

## 2. Grant Necessary Permissions

Grant the SA only the roles required for the application to function (Vertex AI for image generation, Maps API is handled via API Key, but Cloud Run needs to act as this SA):

```bash
# Grant Vertex AI User role (for Gemini API access)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/aiplatform.user"

# Grant Logging Writer (standard for Cloud Run)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/logging.logWriter"

# Grant Firestore User (for Database)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/datastore.user"
    
# Grant Storage Object Viewer (if you later decide to store images in GCS)
# gcloud projects add-iam-policy-binding "$PROJECT_ID" \
#     --member="serviceAccount:${SA_EMAIL}" \
#     --role="roles/storage.objectViewer"
```

## 3. Configure deploy.sh

1.  Open `deploy.sh`.
2.  Locate the section:
    ```bash
    # If you created a specific SA, uncomment the line below:
    # ARGS+=( "--service-account" "$SA_EMAIL" )
    ```
3.  Uncomment the line:
    ```bash
    ARGS+=( "--service-account" "$SA_EMAIL" )
    ```

## 4. Deploy

Run the deployment script again. Cloud Run will now use this specific identity.

```bash
./deploy.sh
```

## Verification

1.  Go to the **Cloud Run** console.
2.  Select the **banana-weather** service.
3.  Go to the **Security** tab.
4.  Verify that the **Service Account** field matches your new SA email (`banana-weather-sa@...`).

```