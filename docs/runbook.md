# Operations Runbook

## Common Tasks

### View Logs

**Real-time logs**:
```bash
gcloud run services logs tail cloudcut-media-server --region=us-central1
```

**Recent logs**:
```bash
gcloud run services logs read cloudcut-media-server --region=us-central1 --limit=100
```

**Filter by severity**:
```bash
gcloud logging read "resource.type=cloud_run_revision AND severity>=ERROR" --limit=50
```

**Filter by time range**:
```bash
gcloud logging read "resource.type=cloud_run_revision" \
  --freshness=1h \
  --limit=100
```

### Scale Service

**Increase max instances**:
```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --max-instances=20
```

**Set min instances (keep warm)**:
```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --min-instances=1
```

**Reset to defaults**:
```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --min-instances=0 \
  --max-instances=10
```

### Update Environment Variable

```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --update-env-vars KEY=VALUE
```

**Remove environment variable**:
```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --remove-env-vars=KEY
```

### Deploy Hotfix

```bash
# Checkout specific commit
git checkout COMMIT_SHA

# Deploy
./scripts/deploy.sh

# Or manually
docker build -t us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:hotfix .
docker push us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:hotfix
gcloud run deploy cloudcut-media-server \
  --image=us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:hotfix \
  --region=us-central1
```

### Rollback Deployment

**List revisions**:
```bash
gcloud run revisions list --service=cloudcut-media-server --region=us-central1
```

**Route 100% traffic to previous revision**:
```bash
gcloud run services update-traffic cloudcut-media-server \
  --region=us-central1 \
  --to-revisions=cloudcut-media-server-00042-abc=100
```

### Restart Service

Force a restart by deploying the same image:

```bash
gcloud run services update cloudcut-media-server \
  --region=us-central1 \
  --image=us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest
```

### Check Service Status

```bash
gcloud run services describe cloudcut-media-server --region=us-central1
```

Key fields to check:
- `status.url` - Service URL
- `status.conditions` - Service health
- `spec.template.spec.containers[0].image` - Current image
- `status.traffic` - Traffic split between revisions

## Incident Response

### Service Down

**Steps**:

1. **Check service status**:
   ```bash
   gcloud run services describe cloudcut-media-server --region=us-central1
   ```

2. **Check recent deployments**:
   ```bash
   gcloud run revisions list --service=cloudcut-media-server --region=us-central1 --limit=5
   ```

3. **Check logs for errors**:
   ```bash
   gcloud logging read "resource.type=cloud_run_revision AND severity>=ERROR" --limit=20
   ```

4. **Rollback if needed**:
   ```bash
   # Identify last working revision
   gcloud run revisions list --service=cloudcut-media-server --region=us-central1

   # Rollback
   gcloud run services update-traffic cloudcut-media-server \
     --region=us-central1 \
     --to-revisions=PREVIOUS_REVISION=100
   ```

5. **Verify recovery**:
   ```bash
   curl https://YOUR-SERVICE-URL.run.app/health
   ```

### High Error Rate

**Steps**:

1. **Check Error Reporting** dashboard
2. **Identify error pattern** (check stack traces)
3. **Review recent code changes**:
   ```bash
   git log --oneline -10
   ```
4. **Check external dependencies**:
   ```bash
   # Test GCS access
   gsutil ls gs://cloudcut-media-PROJECT_ID

   # Check service account permissions
   gcloud projects get-iam-policy PROJECT_ID \
     --flatten="bindings[].members" \
     --filter="bindings.members:serviceAccount:cloudcut-server@"
   ```
5. **Deploy fix or rollback**

### High Latency

**Steps**:

1. **Check Cloud Monitoring** dashboard
2. **Verify instance count is scaling**:
   ```bash
   gcloud run services describe cloudcut-media-server \
     --region=us-central1 \
     --format='get(status.conditions)'
   ```
3. **Check for cold starts**:
   ```bash
   gcloud logging read 'resource.type="cloud_run_revision" AND textPayload:"Cold start"' --limit=10
   ```
4. **Check FFmpeg job queue** (via application logs)
5. **Increase min instances if needed**:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --min-instances=1
   ```

### Out of Memory

**Steps**:

1. **Check memory usage** in Cloud Monitoring
2. **Review logs for OOM kills**:
   ```bash
   gcloud logging read 'resource.type="cloud_run_revision" AND textPayload:"memory"' --limit=20
   ```
3. **Identify memory leak**:
   - Check for goroutine leaks (WebSocket sessions)
   - Review FFmpeg job cleanup
4. **Increase memory allocation**:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --memory=1Gi
   ```
5. **Deploy fix** if leak identified

### GCS Connectivity Issues

**Steps**:

1. **Verify bucket exists**:
   ```bash
   gsutil ls gs://cloudcut-media-PROJECT_ID
   ```

2. **Check service account IAM**:
   ```bash
   gsutil iam get gs://cloudcut-media-PROJECT_ID
   ```

3. **Test connectivity from Cloud Shell**:
   ```bash
   gsutil ls gs://cloudcut-media-PROJECT_ID/sources/
   ```

4. **Verify service account in Cloud Run**:
   ```bash
   gcloud run services describe cloudcut-media-server \
     --region=us-central1 \
     --format='get(spec.template.spec.serviceAccountName)'
   ```

## Maintenance

### Update Secrets

```bash
# Update secret value
echo -n "new-secret-value" | gcloud secrets versions add SECRET_NAME --data-file=-

# Cloud Run automatically picks up latest version (if using :latest)
```

### Clean Up Old Revisions

**Manual cleanup** (keep last 5):

```bash
gcloud run revisions list --service=cloudcut-media-server --region=us-central1 --format="value(name)" | \
  tail -n +6 | \
  xargs -I {} gcloud run revisions delete {} --region=us-central1 --quiet
```

### Update Dependencies

1. **Update `go.mod`** locally
2. **Run tests**: `go test ./...`
3. **Commit and push** to trigger CI/CD
4. **Verify deployment**

### Rotate Service Account Keys

Not applicable - using Workload Identity Federation (keyless).

### Review Resource Usage

**Monthly cost review**:
```bash
# View in Cloud Console: Billing > Reports
# Or use billing export to BigQuery
```

**Storage usage**:
```bash
gsutil du -s gs://cloudcut-media-PROJECT_ID
```

**Active revisions**:
```bash
gcloud run revisions list --service=cloudcut-media-server --region=us-central1
```

## Monitoring

### Key Metrics to Watch

- **Request Rate**: < 1000 req/s (Cloud Run limit)
- **Error Rate**: < 1% (alert at 5%)
- **Latency p95**: < 500ms (alert at 1000ms)
- **Instance Count**: 0-10 based on load
- **Memory Utilization**: < 80%
- **CPU Utilization**: < 80%

### Alerts to Configure

Set up these alerts:
- Error rate > 5% for 5 minutes
- p95 latency > 1000ms for 5 minutes
- Service unavailable

See [monitoring.md](monitoring.md) for setup instructions.

## Security

### Rotate Secrets

```bash
# Generate new secret
NEW_SECRET=$(openssl rand -hex 32)

# Update in Secret Manager
echo -n "$NEW_SECRET" | gcloud secrets versions add jwt-secret --data-file=-

# No restart needed - Cloud Run picks up latest version
```

### Review IAM Permissions

```bash
# List service account permissions
gcloud projects get-iam-policy PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.members:serviceAccount:cloudcut-server@PROJECT_ID.iam.gserviceaccount.com"
```

### Audit Logs

**Secret access logs**:
```bash
gcloud logging read "protoPayload.serviceName=secretmanager.googleapis.com" --limit=50
```

**GCS access logs**:
```bash
gcloud logging read "protoPayload.serviceName=storage.googleapis.com" --limit=50
```

**Admin activity logs**:
```bash
gcloud logging read "logName=projects/PROJECT_ID/logs/cloudaudit.googleapis.com%2Factivity" --limit=50
```

## Cost Optimization

### Reduce Cloud Run Costs

- **Scale to zero**: `--min-instances=0` when not in use
- **Optimize memory**: Use smallest allocation that works
- **Reduce cold starts**: Optimize Docker image size

### Reduce GCS Costs

- **Clean up old exports**: Lifecycle policy (already configured)
- **Use Nearline storage** for archival (future)
- **Compress proxies**: Lower bitrate for previews

### Monitor Costs

**View current month costs**:
```bash
# Cloud Console: Billing > Reports
# Filter by service: Cloud Run, Cloud Storage
```

**Set budget alerts**:
```bash
# Cloud Console: Billing > Budgets & Alerts
# Set alert at 50%, 90%, 100% of budget
```

## Related Documents

- [Architecture](architecture.md)
- [Deployment Guide](deployment.md)
- [Troubleshooting](troubleshooting.md)
- [Monitoring](monitoring.md)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
