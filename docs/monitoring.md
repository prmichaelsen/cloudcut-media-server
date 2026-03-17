# Monitoring and Observability

## Overview

Production observability is provided through Google Cloud's integrated monitoring stack:
- **Cloud Logging** - Structured logs with search and filtering
- **Cloud Monitoring** - Metrics, dashboards, and alerting
- **Error Reporting** - Automatic error aggregation and grouping

## Structured Logging

### Implementation

The application uses structured JSON logging in production for automatic parsing by Cloud Logging.

**Production** (JSON):
```json
{
  "timestamp": "2026-03-17T23:22:00Z",
  "severity": "INFO",
  "message": "http_request",
  "context": {
    "method": "POST",
    "path": "/api/v1/media/upload",
    "status": 200,
    "duration_ms": 1523,
    "bytes": 1048576
  }
}
```

**Development** (human-readable):
```
[INFO] http_request map[method:POST path:/api/v1/media/upload status:200]
```

### Log Levels

| Level | Use Case | Example |
|-------|----------|---------|
| `DEBUG` | Detailed debugging info | Variable values, function entry/exit |
| `INFO` | Normal operations | HTTP requests, job completions |
| `WARN` | Warning conditions | Deprecated API usage, retry attempts |
| `ERROR` | Error conditions | Failed operations, exceptions |

### Usage in Code

```go
import "github.com/prmichaelsen/cloudcut-media-server/internal/logger"

// Initialize logger
log := logger.New(cfg.Env)

// Log with context
log.Info("media_uploaded", map[string]interface{}{
    "media_id": mediaID,
    "size_bytes": fileSize,
    "format": format,
})

log.Error("ffmpeg_failed", map[string]interface{}{
    "job_id": jobID,
    "error": err.Error(),
})
```

## Request Logging Middleware

Automatically logs all HTTP requests:

```go
import (
    "github.com/prmichaelsen/cloudcut-media-server/internal/middleware"
    "github.com/prmichaelsen/cloudcut-media-server/internal/logger"
)

log := logger.New(cfg.Env)
router := api.NewRouter(handlers)
handler := middleware.RequestLogging(log)(router)
```

Logs include:
- HTTP method and path
- Response status code
- Request duration (milliseconds)
- Response size (bytes)
- User agent
- Remote address

## Cloud Monitoring Dashboards

### Default Metrics

Cloud Run automatically collects:
- **Request count** - Requests per second
- **Request latency** - p50, p95, p99
- **Error rate** - 4xx and 5xx responses
- **Instance count** - Active containers
- **Memory utilization** - Per instance
- **CPU utilization** - Per instance
- **Billable time** - Instance uptime

### Custom Dashboard

Create a monitoring dashboard:

```bash
# Using gcloud
gcloud monitoring dashboards create --config-from-file=monitoring/dashboard.json
```

See `agent/tasks/milestone-6-production-deployment/task-25-monitoring-logging.md` for complete dashboard JSON configuration.

## Viewing Logs

### Cloud Console

- **URL**: https://console.cloud.google.com/logs
- **Filter by service**:
  ```
  resource.type="cloud_run_revision"
  resource.labels.service_name="cloudcut-media-server"
  ```

### gcloud CLI

```bash
# Real-time logs
gcloud run services logs tail cloudcut-media-server --region=us-central1

# Recent logs
gcloud run services logs read cloudcut-media-server --region=us-central1 --limit=100

# Filter by severity
gcloud logging read "resource.type=cloud_run_revision AND severity>=ERROR" --limit=50

# Filter by message
gcloud logging read 'resource.type="cloud_run_revision" AND jsonPayload.message="http_request"' --limit=20
```

## Error Reporting

### Automatic Error Capture

Cloud Error Reporting automatically captures logs with severity `ERROR` or higher.

**Access**: https://console.cloud.google.com/errors

**Features**:
- Error grouping by stack trace
- Occurrence frequency
- First/last seen timestamps
- Affected users (future: when auth added)

### Manual Error Reporting

```go
log.Error("render_job_failed", map[string]interface{}{
    "job_id": jobID,
    "project_id": projectID,
    "error": err.Error(),
    "stack_trace": string(debug.Stack()), // Include stack trace
})
```

## Alerting

### Setting Up Alerts

Create alert policies for critical conditions:

```bash
# High error rate alert (> 5%)
gcloud alpha monitoring policies create \
  --notification-channels=CHANNEL_ID \
  --display-name="High Error Rate" \
  --condition-display-name="Error rate > 5%" \
  --condition-threshold-value=0.05 \
  --condition-threshold-duration=300s \
  --condition-filter='resource.type="cloud_run_revision" AND metric.type="run.googleapis.com/request_count" AND metric.labels.response_code_class="5xx"'

# High latency alert (p95 > 1000ms)
gcloud alpha monitoring policies create \
  --notification-channels=CHANNEL_ID \
  --display-name="High Latency" \
  --condition-display-name="p95 latency > 1s" \
  --condition-threshold-value=1000 \
  --condition-threshold-duration=300s \
  --condition-filter='resource.type="cloud_run_revision" AND metric.type="run.googleapis.com/request_latencies"'
```

### Notification Channels

Create email notification channel:

```bash
gcloud alpha monitoring channels create \
  --display-name="Email Alerts" \
  --type=email \
  --channel-labels=email_address=your-email@example.com
```

Get channel ID for use in alert policies:

```bash
gcloud alpha monitoring channels list
```

## Key Metrics to Monitor

### Request Metrics

- **Request Rate**: < 1000 req/s (Cloud Run limit)
- **Error Rate**: < 1% normal, alert at 5%
- **Latency p50**: < 200ms
- **Latency p95**: < 500ms, alert at 1000ms
- **Latency p99**: < 1000ms

### Resource Metrics

- **Instance Count**: Should scale 0-10 based on load
- **Memory Utilization**: < 80% (512Mi allocated)
- **CPU Utilization**: < 80% (1 vCPU allocated)
- **Cold Starts**: Track frequency, optimize if high

### Business Metrics (logged)

- **Uploads**: Count and total size
- **Render Jobs**: Count, duration, success rate
- **Proxy Generation**: Count and average time

## Log Queries

### Recent Errors

```
resource.type="cloud_run_revision"
resource.labels.service_name="cloudcut-media-server"
severity>=ERROR
```

### Slow Requests (> 1 second)

```
resource.type="cloud_run_revision"
jsonPayload.message="http_request"
jsonPayload.context.duration_ms>1000
```

### Failed Render Jobs

```
jsonPayload.message="render_job_failed"
```

### WebSocket Connections

```
jsonPayload.message="websocket_connected" OR
jsonPayload.message="websocket_disconnected"
```

## Troubleshooting Guides

### High Error Rate

1. **Check Error Reporting** for error details and stack traces
2. **Review recent deployments** - may need rollback
3. **Check resource limits** - memory/CPU may be exhausted
4. **Verify external dependencies** - GCS connectivity, Secret Manager

### High Latency

1. **Check instance count** - may need more instances
2. **Review FFmpeg job queue** - jobs may be backing up
3. **Check cold start frequency** - consider min-instances=1
4. **Profile slow requests** - identify bottlenecks

### Service Crashes

1. **Check logs for panics** or SIGKILL events
2. **Review memory usage** - OOM kills common issue
3. **Check goroutine leaks** - WebSocket sessions not cleaned up
4. **Increase memory allocation** if consistently hitting limits

### Missing Logs

1. **Verify structured logging** - check JSON format in production
2. **Check log sampling** - Cloud Run may sample high-volume logs
3. **Verify service account permissions** - needs logging.logWriter role

## Cost

### Cloud Logging

- **Free tier**: 50 GB/month
- **Pricing**: $0.50/GB above free tier
- **Retention**: 30 days default (configurable)

**Estimated**: ~$0-5/month (within free tier for MVP)

### Cloud Monitoring

- **Free tier**: First 150 MB of metrics
- **Pricing**: $0.2580/MB above free tier

**Estimated**: ~$0/month (default Cloud Run metrics within free tier)

### Error Reporting

**Free**

### Total Monitoring Cost

**Estimated**: **~$0-5/month**

## Best Practices

### DO ✅

- **Use structured logging** (JSON) in production
- **Include context** in all log messages (IDs, values)
- **Log errors with stack traces** for debugging
- **Set up alerts** for critical metrics
- **Monitor error rates** and latency regularly
- **Use appropriate log levels** (don't spam INFO)

### DON'T ❌

- **Never log secrets** or sensitive data (passwords, tokens)
- **Don't log at DEBUG** level in production (high volume)
- **Don't ignore warnings** - they often precede errors
- **Don't alert on everything** - alert fatigue is real
- **Don't forget to check logs** after deployments

## Related Documents

- [Cloud Logging](https://cloud.google.com/logging/docs)
- [Cloud Monitoring](https://cloud.google.com/monitoring/docs)
- [Error Reporting](https://cloud.google.com/error-reporting/docs)
- [Cloud Run Observability](https://cloud.google.com/run/docs/logging)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
