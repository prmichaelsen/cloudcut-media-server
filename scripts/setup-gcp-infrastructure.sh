#!/bin/bash
set -e

echo "🚀 Setting up GCP infrastructure for cloudcut-media-server"
echo ""

# Get project ID
export PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
if [ -z "$PROJECT_ID" ]; then
  echo "❌ No GCP project set. Please run: gcloud config set project YOUR_PROJECT_ID"
  exit 1
fi

export PROJECT_NUMBER=$(gcloud projects describe ${PROJECT_ID} --format='value(projectNumber)')
export REGION="us-central1"
export BUCKET_NAME="cloudcut-media-${PROJECT_ID}"
export SA_NAME="cloudcut-server"
export SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
export REPO_NAME="cloudcut"

echo "Project ID: ${PROJECT_ID}"
echo "Project Number: ${PROJECT_NUMBER}"
echo "Region: ${REGION}"
echo ""

# Enable required APIs
echo "📦 Enabling required GCP APIs..."
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  storage.googleapis.com \
  secretmanager.googleapis.com \
  cloudbuild.googleapis.com \
  --quiet

echo "✅ APIs enabled"
echo ""

# Create GCS bucket
echo "🗄️  Creating GCS bucket: ${BUCKET_NAME}"
if gsutil ls -b gs://${BUCKET_NAME} >/dev/null 2>&1; then
  echo "✓ Bucket already exists"
else
  gsutil mb -c STANDARD -l ${REGION} gs://${BUCKET_NAME}
  echo "✅ Bucket created"
fi
echo ""

# Set lifecycle policy
echo "📋 Setting lifecycle policy..."
cat > /tmp/lifecycle.json <<EOF
{
  "lifecycle": {
    "rule": [
      {
        "action": {"type": "Delete"},
        "condition": {
          "age": 30,
          "matchesPrefix": ["exports/"]
        }
      }
    ]
  }
}
EOF

gsutil lifecycle set /tmp/lifecycle.json gs://${BUCKET_NAME}
rm /tmp/lifecycle.json
echo "✅ Lifecycle policy set (delete exports after 30 days)"
echo ""

# Set CORS configuration
echo "🌐 Setting CORS configuration..."
cat > /tmp/cors.json <<EOF
[
  {
    "origin": ["*"],
    "method": ["GET", "HEAD", "PUT", "POST"],
    "responseHeader": ["Content-Type", "Content-Length"],
    "maxAgeSeconds": 3600
  }
]
EOF

gsutil cors set /tmp/cors.json gs://${BUCKET_NAME}
rm /tmp/cors.json
echo "✅ CORS configuration set"
echo ""

# Create service account
echo "👤 Creating service account: ${SA_NAME}"
if gcloud iam service-accounts describe ${SA_EMAIL} >/dev/null 2>&1; then
  echo "✓ Service account already exists"
else
  gcloud iam service-accounts create ${SA_NAME} \
    --display-name="CloudCut Media Server" \
    --description="Service account for cloudcut-media-server Cloud Run service" \
    --quiet
  echo "✅ Service account created"
fi
echo ""

# Grant IAM roles
echo "🔐 Granting IAM roles..."

# Storage Object Admin (bucket-specific)
echo "  → storage.objectAdmin (bucket-scoped)"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/storage.objectAdmin" \
  --condition="expression=resource.name.startsWith('projects/_/buckets/${BUCKET_NAME}'),title=GCS Bucket Access" \
  --quiet || true

# Secret Manager Secret Accessor
echo "  → secretmanager.secretAccessor"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/secretmanager.secretAccessor" \
  --quiet || true

# Logging Writer
echo "  → logging.logWriter"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/logging.logWriter" \
  --quiet || true

# Error Reporting Writer
echo "  → errorreporting.writer"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/errorreporting.writer" \
  --quiet || true

echo "✅ IAM roles granted"
echo ""

# Create Artifact Registry repository
echo "📦 Creating Artifact Registry repository: ${REPO_NAME}"
if gcloud artifacts repositories describe ${REPO_NAME} --location=${REGION} >/dev/null 2>&1; then
  echo "✓ Repository already exists"
else
  gcloud artifacts repositories create ${REPO_NAME} \
    --repository-format=docker \
    --location=${REGION} \
    --description="CloudCut Media Server containers" \
    --quiet
  echo "✅ Repository created"
fi
echo ""

# Configure Docker authentication
echo "🔑 Configuring Docker authentication..."
gcloud auth configure-docker ${REGION}-docker.pkg.dev --quiet
echo "✅ Docker authentication configured"
echo ""

echo "═══════════════════════════════════════════════════════"
echo "  ✅ GCP Infrastructure Setup Complete"
echo "═══════════════════════════════════════════════════════"
echo ""
echo "Resources created:"
echo "  • GCS Bucket: gs://${BUCKET_NAME}"
echo "  • Service Account: ${SA_EMAIL}"
echo "  • Artifact Registry: ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO_NAME}"
echo ""
echo "Next steps:"
echo "  1. Build and push Docker image"
echo "  2. Deploy to Cloud Run"
echo ""
echo "Environment variables for deployment:"
echo "  export GCP_PROJECT_ID=${PROJECT_ID}"
echo "  export GCS_BUCKET_NAME=${BUCKET_NAME}"
echo "  export REGION=${REGION}"
echo ""
