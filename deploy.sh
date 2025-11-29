#!/bin/bash
#
# Banana Weather Deployment Script
#
# This script deploys the "Banana Weather" application to Google Cloud Run.
# It handles loading configuration from a local .env file, resolving defaults
# for the Google Cloud project and resources, and executing the gcloud deploy command.
#
# Usage:
#   ./deploy.sh
#
# Configuration (.env):
#   Required:
#     GEMINI_API_KEY       - Vertex AI / Gemini API Key
#     GOOGLE_MAPS_API_KEY  - Google Maps API Key (Geocoding)
#
#   Optional:
#     PROJECT_ID           - GCP Project ID (Defaults to active gcloud config)
#     REGION               - Cloud Run Region (Defaults to 'us-central1')
#     SERVICE_NAME         - Cloud Run Service Name (Defaults to 'banana-weather')
#

# Load environment variables from .env
if [ -f .env ]; then
  echo "Loading configuration from .env..."
  set -o allexport
  source .env
  set +o allexport
else
  echo "Error: .env file not found. Please create one with the required variables."
  exit 1
fi

# 1. Resolve Project ID
if [ -n "$GOOGLE_CLOUD_PROJECT" ]; then
  PROJECT_ID="$GOOGLE_CLOUD_PROJECT"
elif [ -z "$PROJECT_ID" ]; then
  PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
  if [ -z "$PROJECT_ID" ]; then
    echo "Error: GOOGLE_CLOUD_PROJECT or PROJECT_ID not found in .env and no default project set in gcloud."
    exit 1
  fi
fi
echo "Using Google Cloud Project: $PROJECT_ID"

# 2. Set Defaults for Region and Service Name
REGION="${REGION:-us-central1}"
SERVICE_NAME="${SERVICE_NAME:-banana-weather}"
LOCATION="${GOOGLE_CLOUD_LOCATION:-us-central1}"

# 3. Resolve Bucket Name (Default: project-service)
if [ -z "$GENMEDIA_BUCKET" ]; then
  GENMEDIA_BUCKET="${PROJECT_ID}-${SERVICE_NAME}"
  echo "Using default bucket name: $GENMEDIA_BUCKET"
fi

# 4. Resolve Firestore Database
if [ -z "$FIRESTORE_DATABASE" ]; then
  FIRESTORE_DATABASE="banana-weather"
  echo "Using default firestore database: $FIRESTORE_DATABASE"
fi

# 5. Check for Required API Keys
if [ -z "$GOOGLE_MAPS_API_KEY" ]; then
  echo "Error: Missing GOOGLE_MAPS_API_KEY in .env"
  exit 1
fi

# 6. Build Frontend
echo "🎨 Building Frontend (Flutter Web) for deployment..."
(cd frontend && flutter clean && flutter pub get && flutter build web)
if [ $? -ne 0 ]; then
  echo "Error: Flutter build failed."
  exit 1
fi

# 7. Deploy
# Resolve Service Account (Optional)
SA_NAME="${SERVICE_NAME}-sa"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

echo "Deploying $SERVICE_NAME to $REGION in project $PROJECT_ID..."
echo "GenAI Location: $LOCATION"
echo "Bucket: $GENMEDIA_BUCKET"
echo "Firestore DB: $FIRESTORE_DATABASE"

ARGS=(
  "$SERVICE_NAME"
  "--source" "."
  "--project" "$PROJECT_ID"
  "--region" "$REGION"
  "--allow-unauthenticated"
  "--set-env-vars" "GOOGLE_MAPS_API_KEY=$GOOGLE_MAPS_API_KEY,GOOGLE_CLOUD_PROJECT=$PROJECT_ID,GOOGLE_CLOUD_LOCATION=$LOCATION,GENMEDIA_BUCKET=$GENMEDIA_BUCKET,FIRESTORE_DATABASE=$FIRESTORE_DATABASE"
)

# If you created a specific SA, uncomment the line below:
ARGS+=( "--service-account" "$SA_EMAIL" )

gcloud run deploy "${ARGS[@]}"
