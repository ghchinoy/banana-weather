#!/bin/bash
#
# Service Account Setup Script for Banana Weather
#
# This script creates a dedicated Service Account for the application
# and assigns the necessary permissions (Vertex AI User, Logging Writer).
#

# Load env vars
if [ -f .env ]; then
  source .env
fi

PROJECT_ID=${GOOGLE_CLOUD_PROJECT:-${PROJECT_ID:-$(gcloud config get-value project)}}
SERVICE_NAME="${SERVICE_NAME:-banana-weather}"
SA_NAME="${SERVICE_NAME}-sa"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

echo "------------------------------------------------"
echo "🚀 Setting up Service Account for $SERVICE_NAME"
echo "   Project: $PROJECT_ID"
echo "   Service Account: $SA_EMAIL"
echo "------------------------------------------------"

# 1. Create Service Account
if gcloud iam service-accounts describe "$SA_EMAIL" --project "$PROJECT_ID" >/dev/null 2>&1; then
  echo "✅ Service Account already exists."
else
  echo "Creating Service Account..."
  gcloud iam service-accounts create "$SA_NAME" \
    --display-name="Service Account for $SERVICE_NAME" \
    --project "$PROJECT_ID"
  echo "✅ Service Account created."
fi

# 2. Grant Roles
echo "Granting permissions..."

# Vertex AI User (for Gemini)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/aiplatform.user" \
    --condition=None --quiet

# Logging Writer (for Cloud Run logs)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/logging.logWriter" \
    --condition=None --quiet

# Firestore User (for Database access)
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/datastore.user" \
    --condition=None --quiet

echo "------------------------------------------------"
echo "✅ Setup Complete!"
echo ""
echo "Next Step:"
echo "1. Open 'deploy.sh'"
echo "2. Uncomment the line: ARGS+=( \"--service-account\" \"$SA_EMAIL\" )"
echo "3. Run ./deploy.sh"
echo "------------------------------------------------"
