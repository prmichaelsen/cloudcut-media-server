# Secrets Management

## Overview

Sensitive configuration is stored in Google Secret Manager for enhanced security, rotation capabilities, and audit logging.

## Current Secrets

### jwt-secret

- **Purpose**: JWT signing key (future authentication)
- **Type**: Random 32-byte hex string
- **Rotation**: Every 90 days (manual for MVP)
- **Format**: `jwt-secret-{64-char-hex}`

## Managing Secrets

The `scripts/manage-secrets.sh` script provides a convenient interface for managing secrets.

### Initialize Default Secrets

Create the default secrets required by the application:

```bash
./scripts/manage-secrets.sh init
```

This creates:
- `jwt-secret` - JWT signing key with auto-generated value
- Grants access to the `cloudcut-server` service account

### Create/Update Secret

```bash
./scripts/manage-secrets.sh create SECRET_NAME "secret-value"
```

Example:
```bash
./scripts/manage-secrets.sh create stripe-api-key "sk_live_abc123"
```

### List All Secrets

```bash
./scripts/manage-secrets.sh list
```

### Get Secret Value

```bash
./scripts/manage-secrets.sh get SECRET_NAME
```

**Warning**: Only use this for debugging. Never log or expose secret values.

### Grant Service Account Access

```bash
./scripts/manage-secrets.sh grant SECRET_NAME [service-account-email]
```

If service account email is not provided, defaults to `cloudcut-server@PROJECT_ID.iam.gserviceaccount.com`.

Example:
```bash
./scripts/manage-secrets.sh grant stripe-api-key
```

### Delete Secret

```bash
./scripts/manage-secrets.sh delete SECRET_NAME
```

**Warning**: This permanently deletes the secret. All versions are destroyed.

## Adding New Secret to Cloud Run

### Step 1: Create Secret

```bash
./scripts/manage-secrets.sh create NEW_SECRET "value"
```

### Step 2: Grant Service Account Access

```bash
./scripts/manage-secrets.sh grant NEW_SECRET
```

### Step 3: Update Cloud Run Service

Mount the secret as an environment variable:

```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --update-secrets=ENV_VAR_NAME=NEW_SECRET:latest
```

Or mount as a file (for larger secrets):

```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --update-secrets=/secrets/NEW_SECRET=NEW_SECRET:latest
```

### Step 4: Access in Code

Environment variable:
```go
secret := os.Getenv("ENV_VAR_NAME")
```

File:
```go
data, err := os.ReadFile("/secrets/NEW_SECRET")
if err != nil {
    log.Fatal(err)
}
secret := string(data)
```

## Secret Rotation

### Manual Rotation

1. Generate new secret value
2. Update secret with new version:
   ```bash
   ./scripts/manage-secrets.sh create SECRET_NAME "new-value"
   ```
3. Cloud Run automatically picks up the latest version (no restart needed if using `:latest`)
4. Verify new version is active:
   ```bash
   gcloud secrets versions list SECRET_NAME
   ```

### Automated Rotation (Future)

For production, consider:
- Secret Manager automatic rotation (for supported secret types)
- Cloud Scheduler triggering rotation functions
- Monitoring for secret age and alerting when rotation is needed

## Security Best Practices

### DO ✅

- **Use Secret Manager** for all sensitive data (API keys, credentials, tokens)
- **Rotate secrets regularly** (90 days recommended)
- **Use `:latest` version** in Cloud Run for automatic updates
- **Audit secret access** via Cloud Logging
- **Grant minimal permissions** (secretAccessor role only)
- **Use different secrets** per environment (dev, staging, prod)

### DON'T ❌

- **Never commit secrets** to git (use .gitignore for .env files)
- **Never log secret values** in application code
- **Never expose secrets** in error messages or API responses
- **Never share secrets** via email, chat, or unencrypted channels
- **Never use the same secret** across multiple projects

## Verifying Secret Configuration

### Check Secret Exists

```bash
gcloud secrets describe SECRET_NAME
```

### Check IAM Bindings

```bash
gcloud secrets get-iam-policy SECRET_NAME
```

Expected output should include:
```yaml
bindings:
- members:
  - serviceAccount:cloudcut-server@PROJECT_ID.iam.gserviceaccount.com
  role: roles/secretmanager.secretAccessor
```

### Check Cloud Run Configuration

```bash
gcloud run services describe cloudcut-media-server \
  --region=us-central1 \
  --format='get(spec.template.spec.containers[0].env)'
```

Look for secrets mounted as environment variables.

### Test Secret Access from Cloud Run

Deploy a test revision that logs whether secrets are accessible (without logging values):

```go
func checkSecrets() {
    jwtSecret := os.Getenv("JWT_SECRET")
    if jwtSecret == "" {
        log.Warn("JWT_SECRET not available")
    } else {
        log.Info("JWT_SECRET available", map[string]interface{}{
            "length": len(jwtSecret),
        })
    }
}
```

## Cost

### Secret Manager Pricing

- **Storage**: $0.06/month per secret version (6 active secrets free)
- **Access operations**: $0.03 per 10,000 accesses (10,000 free/month)
- **Replication**: Automatic replication included

**Estimated cost for MVP**: ~$0/month (within free tier)

## Troubleshooting

### Secret Access Denied

**Symptom**: Cloud Run service cannot read secret

**Causes**:
- Service account missing secretAccessor role
- IAM binding not propagated yet
- Secret doesn't exist

**Solutions**:
```bash
# Grant access
./scripts/manage-secrets.sh grant SECRET_NAME

# Wait 1-2 minutes for IAM propagation

# Verify IAM binding
gcloud secrets get-iam-policy SECRET_NAME
```

### Secret Not Found

**Symptom**: Error "Secret not found"

**Causes**:
- Secret name typo
- Secret deleted
- Wrong project

**Solutions**:
```bash
# List all secrets
./scripts/manage-secrets.sh list

# Verify project
gcloud config get-value project
```

### Environment Variable Empty

**Symptom**: `os.Getenv("SECRET")` returns empty string

**Causes**:
- Secret not mounted in Cloud Run
- Wrong environment variable name
- Secret version is empty

**Solutions**:
```bash
# Check Cloud Run env vars
gcloud run services describe cloudcut-media-server \
  --region=us-central1 \
  --format='yaml(spec.template.spec.containers[0].env)'

# Verify secret has value
./scripts/manage-secrets.sh get SECRET_NAME

# Update Cloud Run to mount secret
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --update-secrets=SECRET=SECRET_NAME:latest
```

## Migration from Environment Variables

If you currently have secrets in environment variables:

1. Create secrets in Secret Manager:
   ```bash
   ./scripts/manage-secrets.sh create my-secret "$OLD_ENV_VAR_VALUE"
   ```

2. Grant service account access:
   ```bash
   ./scripts/manage-secrets.sh grant my-secret
   ```

3. Update Cloud Run to use secrets:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --update-secrets=MY_SECRET=my-secret:latest \
     --remove-env-vars=OLD_ENV_VAR
   ```

4. Verify application still works

5. Remove old environment variable from deployment scripts

## Audit Logging

Secret access is automatically logged to Cloud Audit Logs.

### View Secret Access Logs

```bash
gcloud logging read "protoPayload.serviceName=secretmanager.googleapis.com" \
  --limit=50 \
  --format=json
```

### Filter by Secret Name

```bash
gcloud logging read \
  "protoPayload.serviceName=secretmanager.googleapis.com AND \
   protoPayload.resourceName=~\"secrets/jwt-secret\"" \
  --limit=20
```

### Monitor Unusual Access Patterns

Set up alerts for:
- Failed access attempts (permission denied)
- Access from unexpected service accounts
- High frequency access (potential leak)

## Related Documents

- [Secret Manager Documentation](https://cloud.google.com/secret-manager/docs)
- [Cloud Run Secrets](https://cloud.google.com/run/docs/configuring/secrets)
- [Secret Manager Best Practices](https://cloud.google.com/secret-manager/docs/best-practices)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
