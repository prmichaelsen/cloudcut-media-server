# Deployment Guide

## Prerequisites

- Google Cloud Platform account with billing enabled
- `gcloud` CLI installed and authenticated
- Docker installed locally
- GitHub repository access

## Initial Setup

### 1. GCP Infrastructure

Run infrastructure setup (one-time):

```bash
# Set project
gcloud config set project YOUR_PROJECT_ID

# Run automated setup
./scripts/setup-gcp-infrastructure.sh
```

This creates:
- GCS bucket for media storage
- Service account with minimal IAM roles
- Artifact Registry for Docker images
- Lifecycle and CORS policies

See [infrastructure.md](infrastructure.md) for detailed manual steps.

### 2. GitHub Actions (CI/CD)

Configure Workload Identity Federation:

```bash
./scripts/setup-github-actions.sh
```

Then add secrets to GitHub repository settings:
- `https://github.com/YOUR_USERNAME/YOUR_REPO/settings/secrets/actions`

Required secrets (values provided by setup script):
- `GCP_PROJECT_ID`
- `WIF_PROVIDER`
- `WIF_SERVICE_ACCOUNT`

See [cicd.md](cicd.md) for more details.

### 3. Secrets (Optional for MVP)

Initialize default secrets:

```bash
./scripts/manage-secrets.sh init
```

See [secrets.md](secrets.md) for more details.

## Manual Deployment

### Build and Deploy

```bash
# One command deployment
./scripts/deploy.sh
```

This script:
1. Builds Docker image
2. Pushes to Artifact Registry
3. Deploys to Cloud Run
4. Runs health check

### Manual Steps

If you prefer step-by-step:

```bash
# Build image
docker build -t us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest .

# Push to Artifact Registry
docker push us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest

# Deploy to Cloud Run
gcloud run deploy cloudcut-media-server \
  --image=us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest \
  --region=us-central1 \
  --platform=managed
```

## Automated Deployment

### Via GitHub Actions

1. **Create feature branch**:
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make changes and commit**:
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

3. **Push and create PR**:
   ```bash
   git push origin feature/my-feature
   gh pr create --title "Add new feature" --body "Description"
   ```

4. **Tests run automatically** on PR creation

5. **Merge PR**:
   ```bash
   gh pr merge --squash
   ```

6. **Deployment triggers automatically** on merge to main

### Manual Trigger

Trigger deployment without code changes:

```bash
gh workflow run deploy.yml
```

## Verification

After deployment, verify:

```bash
# Get service URL
export SERVICE_URL=$(gcloud run services describe cloudcut-media-server \
  --region=us-central1 --format='value(status.url)')

# Test health endpoint
curl $SERVICE_URL/health

# Expected: {"status":"ok"}

# Test WebSocket (requires websocat)
websocat ${SERVICE_URL/https/wss}/ws
```

## Environment Variables

Configured in Cloud Run:

| Variable | Value | Description |
|----------|-------|-------------|
| `ENV` | `production` | Environment name |
| `GCP_PROJECT_ID` | Your project ID | GCP project |
| `GCS_BUCKET_NAME` | `cloudcut-media-PROJECT_ID` | Storage bucket |
| `FFMPEG_PATH` | `/usr/bin/ffmpeg` | FFmpeg binary path |
| `PORT` | `8080` | HTTP server port (set by Cloud Run) |

Update environment variables:

```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --update-env-vars KEY=VALUE
```

## Rollback

### Via GitHub Actions

1. Go to Actions tab
2. Find last successful deploy workflow
3. Click "Re-run jobs"

### Via gcloud

```bash
# List revisions
gcloud run revisions list \
  --service=cloudcut-media-server \
  --region=us-central1

# Rollback to specific revision
gcloud run services update-traffic cloudcut-media-server \
  --region=us-central1 \
  --to-revisions=cloudcut-media-server-00042-abc=100
```

### Via Image Tag

Deploy specific commit:

```bash
# Deploy previous commit
PREV_SHA=$(git log -2 --format=%H | tail -1)
gcloud run deploy cloudcut-media-server \
  --image=us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:$PREV_SHA \
  --region=us-central1
```

## Health Checks

Cloud Run automatically monitors:
- **HTTP health endpoint**: `/health`
- **Startup probe**: 5 second timeout
- **Liveness probe**: 30 second interval

Health endpoint returns:
```json
{
  "status": "ok"
}
```

## Monitoring

After deployment, monitor at:
- **Logs**: https://console.cloud.google.com/logs
- **Metrics**: https://console.cloud.google.com/monitoring
- **Errors**: https://console.cloud.google.com/errors

See [monitoring.md](monitoring.md) for details.

## Troubleshooting

### Deployment fails

```bash
# Check build logs
gcloud builds list --limit=5

# Check service logs
gcloud run services logs read cloudcut-media-server --region=us-central1 --limit=50
```

### Service not responding

```bash
# Check service status
gcloud run services describe cloudcut-media-server --region=us-central1

# Check recent logs
gcloud run services logs read cloudcut-media-server --region=us-central1 --limit=100
```

### Health check fails

```bash
# Test health endpoint
curl -v https://YOUR-SERVICE-URL.run.app/health

# Check for errors in logs
gcloud logging read "resource.type=cloud_run_revision AND severity>=ERROR" --limit=20
```

See [troubleshooting.md](troubleshooting.md) for detailed guides.

## Related Documents

- [Architecture](architecture.md)
- [Operations Runbook](runbook.md)
- [Infrastructure](infrastructure.md)
- [CI/CD Pipeline](cicd.md)
- [Monitoring](monitoring.md)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
