# Task 23: Configure Secrets Management

**Status**: Not Started
**Milestone**: M6 - Production Deployment
**Estimated Hours**: 2-3
**Priority**: Medium

---

## Objective

Store sensitive configuration (API keys, credentials) in Google Secret Manager and configure Cloud Run to access secrets securely.

---

## Context

Currently, configuration is passed via environment variables in Cloud Run config. For sensitive data (future: API keys, OAuth secrets, database passwords), we should use Secret Manager for:
- **Security**: Secrets encrypted at rest and in transit
- **Rotation**: Easy to rotate without redeploying
- **Audit**: Track secret access
- **Separation**: Secrets separate from code/config

**For MVP**: We don't have many secrets yet, but we'll set up the infrastructure for future needs.

---

## Steps

### 1. Identify Secrets

**Current secrets** (none for MVP, but prepare for future):
- None currently required

**Future secrets**:
- `DATABASE_URL` (when adding persistent storage)
- `JWT_SECRET` (when adding authentication)
- `STRIPE_API_KEY` (when adding payments)
- `GCS_SERVICE_ACCOUNT_KEY` (if not using workload identity)

**Action**: For MVP, create example secret to validate setup

### 2. Create Secrets in Secret Manager

**Action**: Create example secrets

```bash
export PROJECT_ID=$(gcloud config get-value project)

# Create example secret (JWT secret for future auth)
echo -n "your-secret-jwt-key-$(openssl rand -hex 32)" | \
  gcloud secrets create jwt-secret \
    --replication-policy="automatic" \
    --data-file=-

# Verify secret created
gcloud secrets list
gcloud secrets versions list jwt-secret
```

### 3. Grant Service Account Access to Secrets

**Action**: Allow Cloud Run service account to read secrets

```bash
export SA_EMAIL="cloudcut-server@${PROJECT_ID}.iam.gserviceaccount.com"

# Grant Secret Accessor role (already done in Task 21, but verify)
gcloud secrets add-iam-policy-binding jwt-secret \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/secretmanager.secretAccessor"

# Verify IAM binding
gcloud secrets get-iam-policy jwt-secret
```

### 4. Update Cloud Run to Mount Secrets

**Action**: Configure Cloud Run to expose secrets as environment variables

```bash
export REGION="us-central1"
export SERVICE_NAME="cloudcut-media-server"

# Update service to mount secret
gcloud run services update ${SERVICE_NAME} \
  --region=${REGION} \
  --update-secrets=JWT_SECRET=jwt-secret:latest

# Verify secret mounted
gcloud run services describe ${SERVICE_NAME} \
  --region=${REGION} \
  --format='get(spec.template.spec.containers[0].env)'
```

**Alternatively, mount as file** (for larger secrets):

```bash
gcloud run services update ${SERVICE_NAME} \
  --region=${REGION} \
  --update-secrets=/secrets/jwt-secret=jwt-secret:latest
```

### 5. Update Config Loading (Code Changes)

**Action**: Modify `internal/config/config.go` to support secrets

```go
// Add optional secret loading
func Load() *Config {
    cfg := &Config{
        Port:              getEnvInt("PORT", 8080),
        Env:               getEnv("ENV", "development"),
        GCPProjectID:      getEnv("GCP_PROJECT_ID", ""),
        GCSBucketName:     getEnv("GCS_BUCKET_NAME", "cloudcut-media"),
        FFmpegPath:        getEnv("FFMPEG_PATH", "ffmpeg"),
        ProxyResolution:   getEnvInt("PROXY_RESOLUTION", 720),
        ProxyBitrate:      getEnv("PROXY_BITRATE", "1M"),
        JWTSecret:         getEnv("JWT_SECRET", ""), // Will be populated from Secret Manager
    }
    return cfg
}
```

**Note**: For MVP, we don't need JWT yet. This is preparation.

### 6. Create Secret Management Script

**Action**: Create `scripts/manage-secrets.sh`

```bash
#!/bin/bash
set -e

PROJECT_ID=$(gcloud config get-value project)

function create_secret() {
    local name=$1
    local value=$2

    echo "Creating secret: ${name}"
    echo -n "${value}" | gcloud secrets create ${name} \
        --replication-policy="automatic" \
        --data-file=- 2>/dev/null || \
    echo -n "${value}" | gcloud secrets versions add ${name} --data-file=-

    echo "✓ Secret ${name} created/updated"
}

function delete_secret() {
    local name=$1
    gcloud secrets delete ${name} --quiet
    echo "✓ Secret ${name} deleted"
}

function list_secrets() {
    gcloud secrets list
}

function get_secret() {
    local name=$1
    gcloud secrets versions access latest --secret=${name}
}

# Command dispatcher
case "${1}" in
    create)
        create_secret "${2}" "${3}"
        ;;
    delete)
        delete_secret "${2}"
        ;;
    list)
        list_secrets
        ;;
    get)
        get_secret "${2}"
        ;;
    *)
        echo "Usage: $0 {create|delete|list|get} [name] [value]"
        echo ""
        echo "Commands:"
        echo "  create <name> <value>  - Create or update secret"
        echo "  delete <name>          - Delete secret"
        echo "  list                   - List all secrets"
        echo "  get <name>             - Get secret value"
        exit 1
        ;;
esac
```

**Action**: Make executable

```bash
chmod +x scripts/manage-secrets.sh
```

### 7. Document Secret Management

**Action**: Create `docs/secrets.md`

```markdown
# Secrets Management

## Overview
Sensitive configuration stored in Google Secret Manager.

## Current Secrets

### jwt-secret
- **Purpose**: JWT signing key (future authentication)
- **Type**: Random 32-byte hex string
- **Rotation**: Every 90 days (manual for MVP)

## Managing Secrets

### Create/Update Secret
```bash
./scripts/manage-secrets.sh create SECRET_NAME "secret-value"
```

### List Secrets
```bash
./scripts/manage-secrets.sh list
```

### Get Secret Value (for debugging)
```bash
./scripts/manage-secrets.sh get SECRET_NAME
```

### Delete Secret
```bash
./scripts/manage-secrets.sh delete SECRET_NAME
```

## Adding New Secret to Cloud Run

1. Create secret:
   ```bash
   ./scripts/manage-secrets.sh create NEW_SECRET "value"
   ```

2. Update Cloud Run service:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --update-secrets=ENV_VAR_NAME=NEW_SECRET:latest
   ```

3. Access in code:
   ```go
   secret := os.Getenv("ENV_VAR_NAME")
   ```

## Security Best Practices

- ✅ Never commit secrets to git
- ✅ Use Secret Manager for all sensitive data
- ✅ Rotate secrets regularly (90 days)
- ✅ Use latest version in Cloud Run
- ✅ Audit secret access via Cloud Logging
- ❌ Never log secret values
- ❌ Never expose secrets in error messages
```

### 8. Update Deployment Script

**Action**: Modify `scripts/deploy.sh` to preserve secrets

```bash
# Add to deploy.sh before gcloud run deploy:

echo "🔐 Verifying secrets..."
gcloud secrets list --format="value(name)" | while read secret; do
    echo "  ✓ ${secret}"
done
```

---

## Verification

- [ ] Secret Manager API enabled
- [ ] Example secret created (jwt-secret)
- [ ] Service account has secretAccessor role
- [ ] Cloud Run service accesses secret
- [ ] Secret accessible as environment variable in container
- [ ] Secret management script created and tested
- [ ] Documentation created (docs/secrets.md)
- [ ] No secrets committed to git (.env files in .gitignore)

---

## Definition of Done

- Secret Manager configured
- Example secret created and accessible
- Service account permissions verified
- Secret management scripts created
- Documentation complete
- Secrets never committed to repository

---

## Dependencies

**Blocking**:
- Task 21 (Service account created)
- Task 22 (Cloud Run service deployed)

**Required**:
- Secret Manager API enabled
- Service account with secretAccessor role

---

## Notes

- Secrets encrypted at rest and in transit
- Version secrets for rollback capability
- Use `latest` version in Cloud Run for auto-rotation
- Secret access logged in Cloud Audit Logs
- Free tier: 6 active secrets, 10k accesses/month

**Cost**: $0.06/month per secret (6 secrets free)

---

## Related Documents

- [Secret Manager Documentation](https://cloud.google.com/secret-manager/docs)
- [Cloud Run Secrets](https://cloud.google.com/run/docs/configuring/secrets)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
