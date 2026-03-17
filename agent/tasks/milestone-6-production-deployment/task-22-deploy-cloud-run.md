# Task 22: Deploy to Cloud Run

**Status**: Not Started
**Milestone**: M6 - Production Deployment
**Estimated Hours**: 3-4
**Priority**: High

---

## Objective

Build the Docker image, push to Artifact Registry, and deploy the cloudcut-media-server to Google Cloud Run with proper configuration.

---

## Context

With infrastructure provisioned (Task 21) and Docker image created (Task 20), we can now deploy to Cloud Run. Cloud Run will:
- Auto-scale based on traffic (0-10 instances)
- Provide HTTPS endpoint automatically
- Handle health checks and graceful shutdown
- Manage container lifecycle

---

## Steps

### 1. Set Environment Variables

**Action**: Export required variables

```bash
export PROJECT_ID=$(gcloud config get-value project)
export REGION="us-central1"
export SERVICE_NAME="cloudcut-media-server"
export IMAGE_NAME="${REGION}-docker.pkg.dev/${PROJECT_ID}/cloudcut/${SERVICE_NAME}"
export SA_EMAIL="cloudcut-server@${PROJECT_ID}.iam.gserviceaccount.com"
export BUCKET_NAME="cloudcut-media-${PROJECT_ID}"
```

### 2. Build and Tag Docker Image

**Action**: Build image for Cloud Run

```bash
# Build image
docker build -t ${IMAGE_NAME}:latest .

# Verify build
docker images ${IMAGE_NAME}
```

### 3. Push Image to Artifact Registry

**Action**: Push to GCP

```bash
# Push image
docker push ${IMAGE_NAME}:latest

# Verify in Artifact Registry
gcloud artifacts docker images list ${REGION}-docker.pkg.dev/${PROJECT_ID}/cloudcut
```

### 4. Deploy to Cloud Run

**Action**: Create Cloud Run service

```bash
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
  --execution-environment=gen2

# Get service URL
gcloud run services describe ${SERVICE_NAME} --region=${REGION} --format='value(status.url)'
```

**Flags explained**:
- `--platform=managed`: Fully managed Cloud Run (not GKE)
- `--service-account`: Use custom SA with minimal permissions
- `--memory=512Mi`: Sufficient for video processing queue
- `--cpu=1`: 1 vCPU per instance
- `--timeout=60s`: Max request timeout (WebSocket keeps alive separately)
- `--max-instances=10`: Limit concurrent instances for cost control
- `--min-instances=0`: Scale to zero to save costs
- `--allow-unauthenticated`: Public access (add auth later)
- `--cpu-boost`: Faster cold starts
- `--execution-environment=gen2`: Latest Cloud Run gen (better performance)

### 5. Test Deployment

**Action**: Verify service is running

```bash
# Get service URL
export SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region=${REGION} --format='value(status.url)')

echo "Service URL: ${SERVICE_URL}"

# Test health endpoint
curl ${SERVICE_URL}/health

# Expected: {"status":"ok"}
```

### 6. Test WebSocket Connection

**Action**: Verify WebSocket works on Cloud Run

```bash
# Test with websocat (install: brew install websocat)
websocat ${SERVICE_URL/https/wss}/ws

# Or test with JavaScript in browser console:
# const ws = new WebSocket('wss://YOUR-SERVICE-URL.run.app/ws');
# ws.onmessage = (e) => console.log(e.data);
```

### 7. Configure Custom Domain (Optional)

**Action**: Map custom domain to Cloud Run service

```bash
# Add domain mapping
gcloud run domain-mappings create \
  --service=${SERVICE_NAME} \
  --domain=api.cloudcut.media \
  --region=${REGION}

# Verify DNS records (will show required DNS records to add)
gcloud run domain-mappings describe \
  --domain=api.cloudcut.media \
  --region=${REGION}
```

**Note**: Skip if not using custom domain for MVP

### 8. Enable Request Logging

**Action**: Configure structured logging

```bash
# Update service to enable request logs
gcloud run services update ${SERVICE_NAME} \
  --region=${REGION} \
  --ingress=all \
  --execution-environment=gen2 \
  --log-http-requests
```

### 9. Create Deployment Script

**Action**: Create `scripts/deploy.sh`

```bash
#!/bin/bash
set -e

PROJECT_ID=$(gcloud config get-value project)
REGION="us-central1"
SERVICE_NAME="cloudcut-media-server"
IMAGE_NAME="${REGION}-docker.pkg.dev/${PROJECT_ID}/cloudcut/${SERVICE_NAME}"

echo "🚀 Deploying ${SERVICE_NAME} to Cloud Run..."

# Build and push image
echo "📦 Building Docker image..."
docker build -t ${IMAGE_NAME}:latest .

echo "⬆️  Pushing to Artifact Registry..."
docker push ${IMAGE_NAME}:latest

# Deploy to Cloud Run
echo "🌐 Deploying to Cloud Run..."
gcloud run deploy ${SERVICE_NAME} \
  --image=${IMAGE_NAME}:latest \
  --region=${REGION} \
  --platform=managed \
  --quiet

# Get service URL
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region=${REGION} --format='value(status.url)')

echo ""
echo "✅ Deployment complete!"
echo "🔗 Service URL: ${SERVICE_URL}"
echo ""
echo "Test endpoints:"
echo "  Health: curl ${SERVICE_URL}/health"
echo "  Upload: curl -F file=@test.mp4 ${SERVICE_URL}/api/v1/media/upload"
echo "  WebSocket: wscat -c ${SERVICE_URL/https/wss}/ws"
```

**Action**: Make executable

```bash
chmod +x scripts/deploy.sh
```

### 10. Update README with Deployment Info

**Action**: Add deployment section to README

```markdown
## Deployment

### Production
The server is deployed to Google Cloud Run:
- URL: https://cloudcut-media-server-HASH-uc.a.run.app
- Health: https://cloudcut-media-server-HASH-uc.a.run.app/health

### Deploy Changes
```bash
./scripts/deploy.sh
```

### Environment Variables
Configured in Cloud Run:
- `ENV=production`
- `GCP_PROJECT_ID=your-project-id`
- `GCS_BUCKET_NAME=cloudcut-media-your-project-id`
- `FFMPEG_PATH=/usr/bin/ffmpeg`
```

---

## Verification

- [ ] Docker image built and tagged
- [ ] Image pushed to Artifact Registry
- [ ] Cloud Run service deployed successfully
- [ ] Service URL accessible
- [ ] Health endpoint returns 200 OK
- [ ] Service uses correct service account
- [ ] Environment variables set correctly
- [ ] WebSocket connection works
- [ ] Request logging enabled
- [ ] Deployment script created and tested
- [ ] README updated with deployment info

---

## Definition of Done

- Cloud Run service deployed and accessible
- Health endpoint working
- WebSocket connections functional
- Deployment script created
- Documentation updated
- Service verified in Cloud Run Console

---

## Dependencies

**Blocking**:
- Task 20 (Dockerfile created)
- Task 21 (GCP infrastructure provisioned)

**Required**:
- Docker image builds successfully
- Artifact Registry repository exists
- Service account created with IAM roles

---

## Notes

- Cloud Run auto-generates HTTPS certificates
- WebSocket connections supported with 60min timeout
- Cold starts: ~2-5 seconds with `--cpu-boost`
- Scale to zero saves costs when not in use
- Request timeout separate from WebSocket keep-alive
- Cloud Run gen2 has better performance than gen1

**Cost considerations**:
- min-instances=0: ~$0-5/month (pay per request)
- min-instances=1: ~$10-15/month (always warm)

---

## Related Documents

- [Cloud Run Quickstart](https://cloud.google.com/run/docs/quickstarts/deploy-container)
- [Cloud Run WebSocket Support](https://cloud.google.com/run/docs/triggering/websockets)
- [`agent/milestones/milestone-6-production-deployment.md`](../../milestones/milestone-6-production-deployment.md)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
