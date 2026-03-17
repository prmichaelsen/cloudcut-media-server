#!/bin/bash
set -e

PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
if [ -z "$PROJECT_ID" ]; then
  echo "❌ No GCP project set. Please run: gcloud config set project YOUR_PROJECT_ID"
  exit 1
fi

REGION="us-central1"
SERVICE_NAME="cloudcut-media-server"
IMAGE_NAME="${REGION}-docker.pkg.dev/${PROJECT_ID}/cloudcut/${SERVICE_NAME}"
SA_EMAIL="cloudcut-server@${PROJECT_ID}.iam.gserviceaccount.com"
BUCKET_NAME="cloudcut-media-${PROJECT_ID}"

echo "🚀 Deploying ${SERVICE_NAME} to Cloud Run..."
echo ""
echo "Project: ${PROJECT_ID}"
echo "Region: ${REGION}"
echo "Image: ${IMAGE_NAME}:latest"
echo ""

# Build and push image
echo "📦 Building Docker image..."
docker build -t ${IMAGE_NAME}:latest .

echo ""
echo "⬆️  Pushing to Artifact Registry..."
docker push ${IMAGE_NAME}:latest

echo ""
echo "🌐 Deploying to Cloud Run..."
gcloud run deploy ${SERVICE_NAME} \
  --image=${IMAGE_NAME}:latest \
  --region=${REGION} \
  --platform=managed \
  --service-account=${SA_EMAIL} \
  --port=8080 \
  --memory=512Mi \
  --cpu=1 \
  --timeout=60s \
  --max-instances=10 \
  --min-instances=0 \
  --allow-unauthenticated \
  --set-env-vars="ENV=production,GCP_PROJECT_ID=${PROJECT_ID},GCS_BUCKET_NAME=${BUCKET_NAME},FFMPEG_PATH=/usr/bin/ffmpeg" \
  --cpu-boost \
  --execution-environment=gen2 \
  --quiet

# Get service URL
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region=${REGION} --format='value(status.url)')

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  ✅ Deployment complete!"
echo "═══════════════════════════════════════════════════════"
echo ""
echo "🔗 Service URL: ${SERVICE_URL}"
echo ""
echo "Test endpoints:"
echo "  Health: curl ${SERVICE_URL}/health"
echo "  Upload: curl -F file=@test.mp4 ${SERVICE_URL}/api/v1/media/upload"
echo "  WebSocket: websocat ${SERVICE_URL/https/wss}/ws"
echo ""
