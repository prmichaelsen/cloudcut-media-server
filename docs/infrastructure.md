# GCP Infrastructure

## Project

- **Project ID**: `YOUR_PROJECT_ID`
- **Region**: `us-central1`
- **Project Number**: Retrieved dynamically

## Resources Created

### GCS Bucket

- **Name**: `cloudcut-media-YOUR_PROJECT_ID`
- **Location**: `us-central1`
- **Storage class**: `STANDARD`
- **Lifecycle**: Delete exports after 30 days
- **CORS**: Enabled for browser uploads

**Structure**:
```
gs://cloudcut-media-YOUR_PROJECT_ID/
├── sources/      # Original uploaded videos
├── proxies/      # Low-resolution proxy files
└── exports/      # Rendered outputs (auto-deleted after 30 days)
```

### Service Account

- **Name**: `cloudcut-server`
- **Email**: `cloudcut-server@YOUR_PROJECT_ID.iam.gserviceaccount.com`
- **Display Name**: CloudCut Media Server
- **Purpose**: Cloud Run service identity

**IAM Roles**:
- `roles/storage.objectAdmin` - Read/write GCS objects (bucket-scoped)
- `roles/secretmanager.secretAccessor` - Read secrets from Secret Manager
- `roles/logging.logWriter` - Write structured logs
- `roles/errorreporting.writer` - Report errors to Error Reporting

### Artifact Registry

- **Repository Name**: `cloudcut`
- **Location**: `us-central1-docker.pkg.dev/YOUR_PROJECT_ID/cloudcut`
- **Format**: Docker
- **Description**: CloudCut Media Server containers

**Images**:
- `cloudcut-media-server:latest` - Latest build
- `cloudcut-media-server:{commit-sha}` - Version-specific builds

## APIs Enabled

- `run.googleapis.com` - Cloud Run
- `artifactregistry.googleapis.com` - Artifact Registry
- `storage.googleapis.com` - Cloud Storage
- `secretmanager.googleapis.com` - Secret Manager
- `cloudbuild.googleapis.com` - Cloud Build (for CI/CD)

## Setup

### Automated Setup

Run the infrastructure setup script:

```bash
./scripts/setup-gcp-infrastructure.sh
```

This script will:
1. Verify GCP project is set
2. Enable required APIs
3. Create GCS bucket with lifecycle and CORS policies
4. Create service account with minimal IAM roles
5. Create Artifact Registry repository
6. Configure Docker authentication

### Manual Setup

If you prefer manual setup, follow these steps:

#### 1. Set GCP Project

```bash
gcloud config set project YOUR_PROJECT_ID
export PROJECT_ID=$(gcloud config get-value project)
export REGION="us-central1"
```

#### 2. Enable APIs

```bash
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  storage.googleapis.com \
  secretmanager.googleapis.com \
  cloudbuild.googleapis.com
```

#### 3. Create GCS Bucket

```bash
export BUCKET_NAME="cloudcut-media-${PROJECT_ID}"
gsutil mb -c STANDARD -l ${REGION} gs://${BUCKET_NAME}
```

#### 4. Set Lifecycle Policy

```bash
cat > lifecycle.json <<EOF
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

gsutil lifecycle set lifecycle.json gs://${BUCKET_NAME}
```

#### 5. Set CORS Configuration

```bash
cat > cors.json <<EOF
[
  {
    "origin": ["*"],
    "method": ["GET", "HEAD", "PUT", "POST"],
    "responseHeader": ["Content-Type", "Content-Length"],
    "maxAgeSeconds": 3600
  }
]
EOF

gsutil cors set cors.json gs://${BUCKET_NAME}
```

#### 6. Create Service Account

```bash
export SA_NAME="cloudcut-server"
export SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud iam service-accounts create ${SA_NAME} \
  --display-name="CloudCut Media Server" \
  --description="Service account for cloudcut-media-server Cloud Run service"
```

#### 7. Grant IAM Roles

```bash
# Storage Object Admin (bucket-scoped)
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/storage.objectAdmin" \
  --condition="expression=resource.name.startsWith('projects/_/buckets/${BUCKET_NAME}'),title=GCS Bucket Access"

# Secret Manager Secret Accessor
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/secretmanager.secretAccessor"

# Logging Writer
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/logging.logWriter"

# Error Reporting Writer
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/errorreporting.writer"
```

#### 8. Create Artifact Registry Repository

```bash
export REPO_NAME="cloudcut"

gcloud artifacts repositories create ${REPO_NAME} \
  --repository-format=docker \
  --location=${REGION} \
  --description="CloudCut Media Server containers"
```

#### 9. Configure Docker Authentication

```bash
gcloud auth configure-docker ${REGION}-docker.pkg.dev
```

## Verification

Verify all resources were created:

```bash
# Verify bucket
gsutil ls -L gs://cloudcut-media-${PROJECT_ID}

# Verify service account
gcloud iam service-accounts describe cloudcut-server@${PROJECT_ID}.iam.gserviceaccount.com

# Verify Artifact Registry
gcloud artifacts repositories list --location=${REGION}

# Verify IAM bindings
gcloud projects get-iam-policy ${PROJECT_ID} \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:cloudcut-server@${PROJECT_ID}.iam.gserviceaccount.com"

# Verify APIs
gcloud services list --enabled | grep -E "run|artifact|storage|secret|cloudbuild"
```

## Console URLs

- **GCS Bucket**: https://console.cloud.google.com/storage/browser/cloudcut-media-YOUR_PROJECT_ID
- **Artifact Registry**: https://console.cloud.google.com/artifacts/docker/YOUR_PROJECT_ID/us-central1/cloudcut
- **Service Account**: https://console.cloud.google.com/iam-admin/serviceaccounts
- **IAM Permissions**: https://console.cloud.google.com/iam-admin/iam

## Environment Variables

For deployment and development, export these variables:

```bash
export GCP_PROJECT_ID="YOUR_PROJECT_ID"
export GCS_BUCKET_NAME="cloudcut-media-YOUR_PROJECT_ID"
export REGION="us-central1"
export SERVICE_ACCOUNT="cloudcut-server@YOUR_PROJECT_ID.iam.gserviceaccount.com"
```

## Security Notes

### Principle of Least Privilege

The service account has minimal permissions:
- Storage access is scoped to a single bucket (not project-wide)
- No compute permissions (Cloud Run handles this separately)
- No billing or admin permissions
- Secrets are read-only

### CORS Configuration

CORS is currently set to `origin: ["*"]` for development. For production, restrict to your frontend domain:

```json
{
  "origin": ["https://cloudcut.media"],
  "method": ["GET", "HEAD", "PUT", "POST"],
  "responseHeader": ["Content-Type", "Content-Length"],
  "maxAgeSeconds": 3600
}
```

### Lifecycle Policy

The lifecycle policy automatically deletes rendered exports after 30 days to save storage costs. Source files and proxies are retained indefinitely.

## Cost Estimates

### Storage (GCS)
- **First 5GB/month**: ~$1-2
- **100GB/month**: ~$2-5
- **Operations**: Negligible for MVP

### Artifact Registry
- **Storage**: First 500MB free
- **Egress**: Free within same region

### API Usage
- All APIs have free tiers sufficient for MVP
- Cloud Run, Cloud Build covered separately

**Estimated monthly cost for infrastructure**: **$1-5/month**

## Troubleshooting

### Bucket creation fails

**Symptom**: `gsutil mb` returns error

**Causes**:
- Bucket name already taken globally
- Insufficient permissions
- Invalid region

**Solutions**:
1. Try a different bucket name (names are globally unique)
2. Verify you have `storage.admin` permission
3. Check region is valid: `gcloud compute regions list`

### Service account permissions denied

**Symptom**: Cloud Run service cannot access GCS

**Causes**:
- IAM bindings not propagated yet
- Incorrect service account email
- Missing roles

**Solutions**:
1. Wait 1-2 minutes for IAM propagation
2. Verify service account email: `gcloud iam service-accounts list`
3. Re-run IAM binding commands

### Docker authentication fails

**Symptom**: `docker push` to Artifact Registry fails with 401

**Causes**:
- Docker not authenticated
- Expired credentials

**Solutions**:
```bash
gcloud auth configure-docker us-central1-docker.pkg.dev
gcloud auth application-default login
```

## Cleanup

To delete all infrastructure resources:

```bash
# Delete GCS bucket (WARNING: deletes all data)
gsutil rm -r gs://cloudcut-media-${PROJECT_ID}

# Delete service account
gcloud iam service-accounts delete cloudcut-server@${PROJECT_ID}.iam.gserviceaccount.com --quiet

# Delete Artifact Registry repository
gcloud artifacts repositories delete cloudcut --location=us-central1 --quiet
```

## Related Documents

- [GCS Documentation](https://cloud.google.com/storage/docs)
- [Service Accounts Best Practices](https://cloud.google.com/iam/docs/best-practices-service-accounts)
- [Artifact Registry](https://cloud.google.com/artifact-registry/docs)
- [Cloud Run Service Identity](https://cloud.google.com/run/docs/securing/service-identity)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
