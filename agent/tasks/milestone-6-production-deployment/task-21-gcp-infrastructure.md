# Task 21: Set Up GCP Infrastructure

**Status**: Not Started
**Milestone**: M6 - Production Deployment
**Estimated Hours**: 4-5
**Priority**: High

---

## Objective

Set up Google Cloud Platform infrastructure including GCS bucket, service accounts, IAM roles, and Artifact Registry for container images.

---

## Context

Before deploying to Cloud Run, we need to provision GCP resources:
- **GCS bucket** for media storage (sources, proxies, exports)
- **Service account** with minimal IAM permissions
- **Artifact Registry** for Docker images
- **Secret Manager** secrets (configured in Task 23)

---

## Steps

### 1. Set GCP Project

**Action**: Ensure GCP project is set

```bash
# List projects
gcloud projects list

# Set active project
gcloud config set project YOUR_PROJECT_ID

# Verify
gcloud config get-value project
```

**If no project exists**: Create one at https://console.cloud.google.com/projectcreate

### 2. Enable Required APIs

**Action**: Enable GCP APIs

```bash
# Enable APIs
gcloud services enable \
  run.googleapis.com \
  artifactregistry.googleapis.com \
  storage.googleapis.com \
  secretmanager.googleapis.com \
  cloudbuild.googleapis.com

# Verify enabled services
gcloud services list --enabled
```

### 3. Create GCS Bucket

**Action**: Create bucket for media storage

```bash
# Set variables
export PROJECT_ID=$(gcloud config get-value project)
export BUCKET_NAME="cloudcut-media-${PROJECT_ID}"
export REGION="us-central1"  # Choose your region

# Create bucket
gsutil mb -c STANDARD -l ${REGION} gs://${BUCKET_NAME}

# Set lifecycle policy (optional: delete files after 30 days)
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

# Verify bucket
gsutil ls -L gs://${BUCKET_NAME}
```

### 4. Create Service Account

**Action**: Create service account for Cloud Run

```bash
export SA_NAME="cloudcut-server"
export SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

# Create service account
gcloud iam service-accounts create ${SA_NAME} \
  --display-name="CloudCut Media Server" \
  --description="Service account for cloudcut-media-server Cloud Run service"

# Verify
gcloud iam service-accounts list
```

### 5. Grant IAM Roles to Service Account

**Action**: Assign minimal required permissions

```bash
# Storage Object Admin (read/write GCS objects)
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/storage.objectAdmin" \
  --condition="expression=resource.name.startsWith('projects/_/buckets/${BUCKET_NAME}'),title=GCS Bucket Access"

# Secret Manager Secret Accessor (read secrets)
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/secretmanager.secretAccessor"

# Logging Writer (write logs)
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/logging.logWriter"

# Error Reporting Writer (write errors)
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/errorreporting.writer"

# Verify IAM bindings
gcloud projects get-iam-policy ${PROJECT_ID} \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:${SA_EMAIL}"
```

### 6. Create Artifact Registry Repository

**Action**: Create Docker repository

```bash
export REPO_NAME="cloudcut"

# Create repository
gcloud artifacts repositories create ${REPO_NAME} \
  --repository-format=docker \
  --location=${REGION} \
  --description="CloudCut Media Server containers"

# Verify
gcloud artifacts repositories list
```

### 7. Configure Docker Authentication

**Action**: Set up Docker to push to Artifact Registry

```bash
# Configure Docker authentication
gcloud auth configure-docker ${REGION}-docker.pkg.dev

# Verify
docker info | grep -A 10 "Registry"
```

### 8. Create CORS Configuration for GCS Bucket

**Action**: Allow browser uploads (for future direct uploads)

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

# Verify
gsutil cors get gs://${BUCKET_NAME}
```

### 9. Document Infrastructure

**Action**: Create `infrastructure.md`

```markdown
# GCP Infrastructure

## Project
- Project ID: `YOUR_PROJECT_ID`
- Region: `us-central1`

## Resources Created

### GCS Bucket
- Name: `cloudcut-media-YOUR_PROJECT_ID`
- Location: `us-central1`
- Storage class: `STANDARD`
- Lifecycle: Delete exports after 30 days

### Service Account
- Email: `cloudcut-server@YOUR_PROJECT_ID.iam.gserviceaccount.com`
- Roles:
  - `storage.objectAdmin` (GCS bucket only)
  - `secretmanager.secretAccessor`
  - `logging.logWriter`
  - `errorreporting.writer`

### Artifact Registry
- Repository: `cloudcut`
- Location: `us-central1-docker.pkg.dev/YOUR_PROJECT_ID/cloudcut`
- Format: Docker

## URLs
- GCS Bucket: https://console.cloud.google.com/storage/browser/cloudcut-media-YOUR_PROJECT_ID
- Artifact Registry: https://console.cloud.google.com/artifacts/docker/YOUR_PROJECT_ID/us-central1/cloudcut
- IAM: https://console.cloud.google.com/iam-admin/serviceaccounts

## Environment Variables
- `GCP_PROJECT_ID`: YOUR_PROJECT_ID
- `GCS_BUCKET_NAME`: cloudcut-media-YOUR_PROJECT_ID
```

**Action**: Save to `docs/infrastructure.md`

---

## Verification

- [ ] GCP project selected
- [ ] Required APIs enabled (run, artifactregistry, storage, secretmanager)
- [ ] GCS bucket created and accessible
- [ ] Bucket lifecycle policy set
- [ ] Service account created
- [ ] IAM roles granted to service account
- [ ] Artifact Registry repository created
- [ ] Docker authenticated to Artifact Registry
- [ ] CORS configuration set on bucket
- [ ] Infrastructure documented in docs/infrastructure.md

---

## Definition of Done

- GCP infrastructure provisioned
- Service account with minimal permissions
- GCS bucket created and configured
- Artifact Registry ready
- Infrastructure documented
- All resources verified in GCP Console

---

## Dependencies

**Blocking**:
- GCP account with billing enabled
- `gcloud` CLI installed and authenticated

**Downstream**:
- Task 22 (Deploy to Cloud Run) needs service account and Artifact Registry
- Task 23 (Secrets) needs Secret Manager API enabled

---

## Notes

- Use consistent naming: `cloudcut-*` prefix for all resources
- Region choice affects latency and costs (us-central1 is cheapest)
- Service account principle of least privilege
- Bucket lifecycle saves storage costs
- CORS allows future browser-based uploads
- Artifact Registry replaces deprecated Container Registry

---

## Related Documents

- [GCS Documentation](https://cloud.google.com/storage/docs)
- [Service Accounts Best Practices](https://cloud.google.com/iam/docs/best-practices-service-accounts)
- [Artifact Registry](https://cloud.google.com/artifact-registry/docs)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
