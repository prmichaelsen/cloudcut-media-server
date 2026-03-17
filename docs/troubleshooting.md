# Troubleshooting Guide

## Build Issues

### Docker build fails

**Symptom**: `docker build` command errors

**Common causes**:
- Missing dependencies in Dockerfile
- Go module download failure
- Build context too large
- Insufficient disk space

**Solutions**:

1. **Check Docker daemon**:
   ```bash
   docker info
   ```

2. **Verify `.dockerignore`** excludes large files:
   ```bash
   cat .dockerignore
   ```

3. **Check network connectivity** for `go mod download`:
   ```bash
   go mod download -x
   ```

4. **Clear Docker cache**:
   ```bash
   docker system prune -a
   ```

5. **Build with verbose output**:
   ```bash
   docker build --progress=plain -t test .
   ```

### Tests fail in CI

**Symptom**: GitHub Actions test workflow fails

**Common causes**:
- Test dependencies missing
- Environment differences (timezone, PATH)
- Race conditions
- Flaky tests

**Solutions**:

1. **Run tests locally**:
   ```bash
   go test -v ./...
   ```

2. **Run with race detector**:
   ```bash
   go test -race ./...
   ```

3. **Check test logs** in GitHub Actions UI

4. **Test locally with act** (GitHub Actions locally):
   ```bash
   act -j test
   ```

## Deployment Issues

### Image push fails

**Symptom**: `docker push` to Artifact Registry fails

**Common causes**:
- Not authenticated to Artifact Registry
- Repository doesn't exist
- Insufficient permissions
- Network issues

**Solutions**:

1. **Authenticate Docker**:
   ```bash
   gcloud auth configure-docker us-central1-docker.pkg.dev
   ```

2. **Verify repository exists**:
   ```bash
   gcloud artifacts repositories list --location=us-central1
   ```

3. **Check IAM permissions**:
   ```bash
   gcloud projects get-iam-policy PROJECT_ID \
     --flatten="bindings[].members" \
     --filter="bindings.members:serviceAccount:"
   ```

4. **Test with smaller image**:
   ```bash
   docker pull alpine
   docker tag alpine us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/test
   docker push us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/test
   ```

### Cloud Run deployment fails

**Symptom**: `gcloud run deploy` errors

**Common causes**:
- Image not found
- Service account missing permissions
- Invalid configuration
- Quota exceeded

**Solutions**:

1. **Verify image exists**:
   ```bash
   gcloud artifacts docker images list us-central1-docker.pkg.dev/PROJECT_ID/cloudcut
   ```

2. **Check service account**:
   ```bash
   gcloud iam service-accounts list | grep cloudcut
   ```

3. **Review deployment logs**:
   ```bash
   gcloud run services describe cloudcut-media-server --region=us-central1
   ```

4. **Check quotas**:
   ```bash
   gcloud compute regions describe us-central1 --format="table(quotas)"
   ```

### Workload Identity Federation fails

**Symptom**: GitHub Actions authentication fails

**Common causes**:
- WIF not configured correctly
- Service account missing permissions
- GitHub secrets incorrect
- Repository attribute mismatch

**Solutions**:

1. **Re-run setup script**:
   ```bash
   ./scripts/setup-github-actions.sh
   ```

2. **Verify IAM bindings**:
   ```bash
   gcloud iam service-accounts get-iam-policy \
     github-actions-deployer@PROJECT_ID.iam.gserviceaccount.com
   ```

3. **Check GitHub secrets**:
   ```bash
   gh secret list
   ```

4. **Test WIF manually**:
   ```bash
   # See WIF provider configuration
   gcloud iam workload-identity-pools providers describe github-actions-provider \
     --workload-identity-pool=github-actions-pool \
     --location=global
   ```

## Runtime Issues

### Service returns 500 errors

**Symptom**: Health endpoint or API returns 500

**Common causes**:
- Application crash on startup
- Missing environment variables
- GCS connectivity issues
- Panic in request handler

**Solutions**:

1. **Check logs**:
   ```bash
   gcloud run services logs read cloudcut-media-server --region=us-central1 --limit=50
   ```

2. **Verify environment variables**:
   ```bash
   gcloud run services describe cloudcut-media-server --region=us-central1 \
     --format='get(spec.template.spec.containers[0].env)'
   ```

3. **Test GCS access**:
   ```bash
   gsutil ls gs://cloudcut-media-PROJECT_ID
   ```

4. **Check for panics** in logs:
   ```bash
   gcloud logging read 'resource.type="cloud_run_revision" AND textPayload:"panic"' --limit=10
   ```

### WebSocket connections fail

**Symptom**: Client cannot connect to `/ws`

**Common causes**:
- WebSocket not supported by proxy
- Cloud Run timeout (60s default)
- Network/proxy issues
- Certificate issues

**Solutions**:

1. **Verify WebSocket URL** uses `wss://`:
   ```javascript
   const ws = new WebSocket('wss://YOUR-SERVICE.run.app/ws');
   ```

2. **Test with websocat**:
   ```bash
   websocat wss://YOUR-SERVICE.run.app/ws
   ```

3. **Check Cloud Run configuration**:
   ```bash
   gcloud run services describe cloudcut-media-server --region=us-central1 \
     --format='get(spec.template.spec.containers[0].ports)'
   ```

4. **Review session logs**:
   ```bash
   gcloud logging read 'jsonPayload.message:"websocket"' --limit=20
   ```

### FFmpeg jobs fail

**Symptom**: Render jobs fail with FFmpeg errors

**Common causes**:
- Invalid EDL format
- Missing media files in GCS
- FFmpeg out of memory
- Unsupported codec/format

**Solutions**:

1. **Check EDL validation logs**:
   ```bash
   gcloud logging read 'jsonPayload.message:"edl_validation"' --limit=10
   ```

2. **Verify media files exist**:
   ```bash
   gsutil ls gs://cloudcut-media-PROJECT_ID/sources/
   ```

3. **Check memory usage** in Cloud Monitoring

4. **Test FFmpeg manually** in container:
   ```bash
   docker run --rm -it us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest sh
   ffmpeg -version
   ffmpeg -i /path/to/test.mp4 -t 5 output.mp4
   ```

5. **Review FFmpeg logs**:
   ```bash
   gcloud logging read 'jsonPayload.message:"ffmpeg"' --limit=20
   ```

### Container crashes

**Symptom**: Cloud Run service restarts frequently

**Common causes**:
- Out of memory (OOM kill)
- Panic/segfault
- Goroutine leaks
- Infinite loop

**Solutions**:

1. **Check crash logs** (look for "panic" or "SIGKILL"):
   ```bash
   gcloud logging read 'resource.type="cloud_run_revision" AND (textPayload:"panic" OR textPayload:"SIGKILL")' --limit=20
   ```

2. **Monitor memory** in Cloud Monitoring:
   - Check memory utilization over time
   - Look for gradual increase (leak) vs sudden spike

3. **Check for goroutine leaks**:
   - Add goroutine count logging
   - Review WebSocket session cleanup

4. **Increase memory**:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --memory=1Gi
   ```

## Performance Issues

### High latency

**Symptom**: Requests take > 1 second

**Common causes**:
- Cold starts
- FFmpeg processing time
- GCS download latency
- Insufficient instances

**Solutions**:

1. **Check cold start frequency**:
   ```bash
   gcloud logging read 'textPayload:"Cold start"' --limit=10
   ```

2. **Increase min instances**:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --min-instances=1
   ```

3. **Enable CPU boost** (faster cold starts):
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --cpu-boost
   ```

4. **Increase max instances**:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --max-instances=20
   ```

5. **Profile critical paths**:
   - Add timing logs to identify bottlenecks
   - Use Go profiling tools (pprof)

### Slow uploads

**Symptom**: Media uploads take too long

**Common causes**:
- Network bandwidth limitations
- GCS region mismatch
- Large file sizes
- Client connection issues

**Solutions**:

1. **Verify GCS bucket region** matches Cloud Run (us-central1)

2. **Check upload logs**:
   ```bash
   gcloud logging read 'jsonPayload.message:"upload"' --limit=10
   ```

3. **Test upload speed**:
   ```bash
   time curl -F file=@test.mp4 https://YOUR-SERVICE.run.app/api/v1/media/upload
   ```

4. **Consider direct client-to-GCS uploads** with signed URLs (future enhancement)

### Slow renders

**Symptom**: Video rendering takes too long

**Common causes**:
- Complex EDL (many clips, filters)
- High-resolution source files
- Limited CPU
- Sequential processing

**Solutions**:

1. **Profile FFmpeg execution**:
   ```bash
   # Check render duration logs
   gcloud logging read 'jsonPayload.message:"render"' --limit=20
   ```

2. **Increase CPU**:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --cpu=2
   ```

3. **Optimize EDL**:
   - Reduce number of clips
   - Simplify filter chains
   - Use proxy files instead of sources for previews

4. **Parallel processing** (future enhancement):
   - Split EDL into segments
   - Render segments in parallel
   - Concatenate results

## Security Issues

### Unauthorized access

**Symptom**: Unexpected access to service

**Common causes**:
- Service is `--allow-unauthenticated`
- Leaked credentials (future: when auth added)

**Solutions**:

1. **Add authentication** (see Milestone 7)

2. **Use Cloud IAM** for service-to-service:
   ```bash
   gcloud run services update cloudcut-media-server \
     --region=us-central1 \
     --no-allow-unauthenticated
   ```

3. **Review Cloud Audit Logs**:
   ```bash
   gcloud logging read "logName=projects/PROJECT_ID/logs/cloudaudit.googleapis.com%2Factivity" \
     --limit=50
   ```

### Secret access denied

**Symptom**: Service cannot read secrets

**Common causes**:
- Service account missing `secretAccessor` role
- Secret doesn't exist
- IAM binding not propagated

**Solutions**:

1. **Check IAM binding**:
   ```bash
   gcloud secrets get-iam-policy SECRET_NAME
   ```

2. **Grant access**:
   ```bash
   ./scripts/manage-secrets.sh grant SECRET_NAME
   ```

3. **Wait for propagation** (1-2 minutes)

4. **Verify secret exists**:
   ```bash
   gcloud secrets list
   ```

## Data Issues

### Media files not found

**Symptom**: API returns 404 for media

**Common causes**:
- File not uploaded to GCS
- Incorrect bucket name
- IAM permissions missing

**Solutions**:

1. **Verify file exists**:
   ```bash
   gsutil ls gs://cloudcut-media-PROJECT_ID/sources/MEDIA_ID.mp4
   ```

2. **Check bucket name** in environment:
   ```bash
   gcloud run services describe cloudcut-media-server --region=us-central1 \
     --format='get(spec.template.spec.containers[0].env[].value)'
   ```

3. **Verify service account** has `storage.objectAdmin`:
   ```bash
   gsutil iam get gs://cloudcut-media-PROJECT_ID
   ```

### Proxies not generated

**Symptom**: Proxy files missing after upload

**Common causes**:
- FFmpeg error during generation
- GCS write failure
- Async job not started

**Solutions**:

1. **Check logs for FFmpeg errors**:
   ```bash
   gcloud logging read 'jsonPayload.message:"proxy"' --limit=20
   ```

2. **Verify GCS write permissions**

3. **Test FFmpeg locally** with same input:
   ```bash
   docker run --rm -v $(pwd):/data us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest \
     ffmpeg -i /data/test.mp4 -vf scale=-2:720 /data/proxy.mp4
   ```

4. **Check goroutine started**:
   ```bash
   gcloud logging read 'jsonPayload.message:"proxy_generation_started"' --limit=10
   ```

## Related Documents

- [Architecture](architecture.md)
- [Deployment Guide](deployment.md)
- [Operations Runbook](runbook.md)
- [Monitoring](monitoring.md)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
