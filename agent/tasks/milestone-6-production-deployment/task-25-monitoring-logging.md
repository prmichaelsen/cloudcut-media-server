# Task 25: Add Monitoring and Logging

**Status**: Not Started
**Milestone**: M6 - Production Deployment
**Estimated Hours**: 3-4
**Priority**: Medium

---

## Objective

Set up structured logging, error tracking, and monitoring dashboards to observe production health, performance, and errors.

---

## Context

Production services need observability to:
- **Debug issues**: Trace requests through logs
- **Track errors**: Aggregate and alert on errors
- **Monitor performance**: Track latency, throughput, resource usage
- **Business metrics**: Monitor upload count, render jobs, proxy generation

Google Cloud provides integrated observability:
- **Cloud Logging**: Structured logs with filtering/search
- **Error Reporting**: Automatic error aggregation
- **Cloud Monitoring**: Metrics, dashboards, alerts
- **Cloud Trace**: Distributed tracing (future enhancement)

---

## Steps

### 1. Implement Structured Logging

**Action**: Add structured logging to `internal/logger/logger.go`

```go
package logger

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type Logger struct {
	env string
}

func New(env string) *Logger {
	return &Logger{env: env}
}

type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

func (l *Logger) log(level Level, msg string, ctx map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Severity:  string(level),
		Message:   msg,
		Context:   ctx,
	}

	if l.env == "production" {
		// Structured JSON for Cloud Logging
		json.NewEncoder(os.Stdout).Encode(entry)
	} else {
		// Human-readable for development
		log.Printf("[%s] %s %v", level, msg, ctx)
	}
}

func (l *Logger) Debug(msg string, ctx map[string]interface{}) {
	l.log(LevelDebug, msg, ctx)
}

func (l *Logger) Info(msg string, ctx map[string]interface{}) {
	l.log(LevelInfo, msg, ctx)
}

func (l *Logger) Warn(msg string, ctx map[string]interface{}) {
	l.log(LevelWarn, msg, ctx)
}

func (l *Logger) Error(msg string, ctx map[string]interface{}) {
	l.log(LevelError, msg, ctx)
}
```

### 2. Add Request Logging Middleware

**Action**: Create `internal/middleware/logging.go`

```go
package middleware

import (
	"net/http"
	"time"

	"github.com/prmichaelsen/cloudcut-media-server/internal/logger"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.bytes += len(b)
	return rw.ResponseWriter.Write(b)
}

func RequestLogging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			log.Info("http_request", map[string]interface{}{
				"method":      r.Method,
				"path":        r.URL.Path,
				"status":      rw.statusCode,
				"duration_ms": duration.Milliseconds(),
				"bytes":       rw.bytes,
				"user_agent":  r.UserAgent(),
				"remote_addr": r.RemoteAddr,
			})
		})
	}
}
```

**Action**: Update `cmd/server/main.go` to use middleware

```go
import "github.com/prmichaelsen/cloudcut-media-server/internal/middleware"

// In main():
log := logger.New(cfg.Env)
router := api.NewRouter(handlers)
handler := middleware.RequestLogging(log)(router)
server := &http.Server{
	Addr:    ":" + strconv.Itoa(cfg.Port),
	Handler: handler,
}
```

### 3. Add Error Tracking

**Action**: Cloud Error Reporting automatically captures logs with severity ERROR or higher. Ensure errors are logged with context:

```go
// Example in render job:
if err := job.Execute(); err != nil {
	log.Error("render_job_failed", map[string]interface{}{
		"job_id":     job.ID,
		"project_id": job.ProjectID,
		"error":      err.Error(),
	})
	return err
}
```

### 4. Add Custom Metrics

**Action**: Create `internal/metrics/metrics.go`

```go
package metrics

import (
	"context"
	"log"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	metricpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Metrics struct {
	projectID string
	client    *monitoring.MetricClient
}

func New(projectID string) (*Metrics, error) {
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		projectID: projectID,
		client:    client,
	}, nil
}

func (m *Metrics) RecordUpload(size int64) {
	// Record upload count and size
	m.recordInt64("uploads_total", 1)
	m.recordInt64("upload_bytes", size)
}

func (m *Metrics) RecordRenderJob(durationMs int64, success bool) {
	m.recordInt64("render_jobs_total", 1)
	m.recordInt64("render_duration_ms", durationMs)
	if success {
		m.recordInt64("render_jobs_success", 1)
	} else {
		m.recordInt64("render_jobs_failed", 1)
	}
}

func (m *Metrics) recordInt64(metricType string, value int64) {
	// Implementation simplified for brevity
	log.Printf("Metric: %s = %d", metricType, value)
}

func (m *Metrics) Close() error {
	return m.client.Close()
}
```

**Note**: For MVP, use Cloud Logging metrics instead of custom metrics API (simpler, free tier).

### 5. Create Monitoring Dashboard

**Action**: Create dashboard configuration file `monitoring/dashboard.json`

```json
{
  "displayName": "CloudCut Media Server",
  "mosaicLayout": {
    "columns": 12,
    "tiles": [
      {
        "width": 6,
        "height": 4,
        "widget": {
          "title": "Request Rate",
          "xyChart": {
            "dataSets": [{
              "timeSeriesQuery": {
                "timeSeriesFilter": {
                  "filter": "resource.type=\"cloud_run_revision\" resource.labels.service_name=\"cloudcut-media-server\"",
                  "aggregation": {
                    "alignmentPeriod": "60s",
                    "perSeriesAligner": "ALIGN_RATE"
                  }
                }
              }
            }]
          }
        }
      },
      {
        "xPos": 6,
        "width": 6,
        "height": 4,
        "widget": {
          "title": "Request Latency (p95)",
          "xyChart": {
            "dataSets": [{
              "timeSeriesQuery": {
                "timeSeriesFilter": {
                  "filter": "resource.type=\"cloud_run_revision\" metric.type=\"run.googleapis.com/request_latencies\"",
                  "aggregation": {
                    "alignmentPeriod": "60s",
                    "perSeriesAligner": "ALIGN_DELTA",
                    "crossSeriesReducer": "REDUCE_PERCENTILE_95"
                  }
                }
              }
            }]
          }
        }
      },
      {
        "yPos": 4,
        "width": 6,
        "height": 4,
        "widget": {
          "title": "Error Rate",
          "xyChart": {
            "dataSets": [{
              "timeSeriesQuery": {
                "timeSeriesFilter": {
                  "filter": "resource.type=\"cloud_run_revision\" metric.type=\"logging.googleapis.com/user/error_count\"",
                  "aggregation": {
                    "alignmentPeriod": "60s",
                    "perSeriesAligner": "ALIGN_RATE"
                  }
                }
              }
            }]
          }
        }
      },
      {
        "xPos": 6,
        "yPos": 4,
        "width": 6,
        "height": 4,
        "widget": {
          "title": "Instance Count",
          "xyChart": {
            "dataSets": [{
              "timeSeriesQuery": {
                "timeSeriesFilter": {
                  "filter": "resource.type=\"cloud_run_revision\" metric.type=\"run.googleapis.com/container/instance_count\"",
                  "aggregation": {
                    "alignmentPeriod": "60s",
                    "perSeriesAligner": "ALIGN_MEAN"
                  }
                }
              }
            }]
          }
        }
      }
    ]
  }
}
```

**Action**: Create dashboard via gcloud

```bash
gcloud monitoring dashboards create --config-from-file=monitoring/dashboard.json
```

### 6. Set Up Alerts

**Action**: Create alert policies for critical issues

```bash
# Alert on error rate > 5%
gcloud alpha monitoring policies create \
  --notification-channels=CHANNEL_ID \
  --display-name="High Error Rate" \
  --condition-display-name="Error rate > 5%" \
  --condition-threshold-value=0.05 \
  --condition-threshold-duration=300s \
  --condition-filter='resource.type="cloud_run_revision" AND metric.type="run.googleapis.com/request_count" AND metric.labels.response_code_class="5xx"'

# Alert on high latency (p95 > 1000ms)
gcloud alpha monitoring policies create \
  --notification-channels=CHANNEL_ID \
  --display-name="High Latency" \
  --condition-display-name="p95 latency > 1s" \
  --condition-threshold-value=1000 \
  --condition-threshold-duration=300s \
  --condition-filter='resource.type="cloud_run_revision" AND metric.type="run.googleapis.com/request_latencies"'
```

**Setup notification channel**:

```bash
# Create email notification channel
gcloud alpha monitoring channels create \
  --display-name="Email Alerts" \
  --type=email \
  --channel-labels=email_address=your-email@example.com
```

### 7. Add Health Check Endpoint with Details

**Action**: Enhance health endpoint in `internal/api/handlers.go`

```go
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0", // Add from build flags
		"checks": map[string]interface{}{
			"storage": h.checkStorage(),
			"ffmpeg":  h.checkFFmpeg(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (h *Handlers) checkStorage() string {
	// Verify GCS connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := h.storage.Stat(ctx, "health-check")
	if err == nil {
		return "ok"
	}
	return "degraded"
}

func (h *Handlers) checkFFmpeg() string {
	// Verify FFmpeg available
	cmd := exec.Command("ffmpeg", "-version")
	if err := cmd.Run(); err != nil {
		return "unavailable"
	}
	return "ok"
}
```

### 8. Document Monitoring

**Action**: Create `docs/monitoring.md`

```markdown
# Monitoring and Observability

## Dashboards

### Cloud Monitoring Dashboard
- URL: https://console.cloud.google.com/monitoring/dashboards
- Metrics: Request rate, latency, errors, instance count

### Cloud Logging
- URL: https://console.cloud.google.com/logs
- Filter by severity: `severity>=ERROR`
- Filter by service: `resource.labels.service_name="cloudcut-media-server"`

### Error Reporting
- URL: https://console.cloud.google.com/errors
- Automatic error aggregation and grouping

## Key Metrics

### Request Metrics
- **Request Rate**: Requests per second
- **Latency**: p50, p95, p99 response times
- **Error Rate**: 4xx and 5xx response codes
- **Instance Count**: Active Cloud Run instances

### Business Metrics (logged)
- **Uploads**: Media upload count and size
- **Render Jobs**: Job count, duration, success rate
- **Proxy Generation**: Proxy creation time

## Alerts

### Critical
- Error rate > 5% for 5 minutes
- p95 latency > 1000ms for 5 minutes
- Service unavailable

### Warning
- Error rate > 1% for 10 minutes
- p95 latency > 500ms for 10 minutes

## Log Queries

### Recent errors
```
resource.type="cloud_run_revision"
resource.labels.service_name="cloudcut-media-server"
severity>=ERROR
```

### Slow requests
```
resource.type="cloud_run_revision"
jsonPayload.duration_ms>1000
```

### Failed render jobs
```
jsonPayload.message="render_job_failed"
```

## Troubleshooting

### High error rate
1. Check Error Reporting for error details
2. Review recent deployments
3. Check resource limits (memory, CPU)
4. Verify GCS connectivity

### High latency
1. Check Cloud Monitoring instance count
2. Review FFmpeg job queue
3. Check cold start frequency
4. Consider increasing `--min-instances`

### Service crashes
1. Check Cloud Run logs for crash dumps
2. Review memory usage (OOM kills)
3. Check goroutine leaks (WebSocket sessions)
```

---

## Verification

- [ ] Structured logging implemented
- [ ] Request logging middleware added
- [ ] Errors logged with context
- [ ] Cloud Monitoring dashboard created
- [ ] Alert policies configured
- [ ] Notification channels set up
- [ ] Health endpoint includes dependency checks
- [ ] Logs visible in Cloud Logging
- [ ] Errors appear in Error Reporting
- [ ] Dashboard shows metrics
- [ ] Documentation created

---

## Definition of Done

- Structured logging in production
- Request/response logging enabled
- Monitoring dashboard created
- Alerts configured and tested
- Health checks include dependencies
- Documentation complete
- Team trained on monitoring tools

---

## Dependencies

**Blocking**:
- Task 22 (Cloud Run deployed)

**Required**:
- Cloud Logging API enabled
- Cloud Monitoring API enabled
- Error Reporting API enabled

---

## Notes

- Cloud Logging free tier: 50 GB/month
- Cloud Monitoring free tier: First 150 MB of metrics
- Error Reporting: Free
- Use log-based metrics instead of custom metrics API for MVP (simpler, free)
- Structured JSON logs automatically parsed by Cloud Logging
- Severity field (`DEBUG`, `INFO`, `WARN`, `ERROR`) determines Error Reporting

**Cost**: ~$0-5/month (within free tier for MVP)

---

## Related Documents

- [Cloud Logging](https://cloud.google.com/logging/docs)
- [Cloud Monitoring](https://cloud.google.com/monitoring/docs)
- [Error Reporting](https://cloud.google.com/error-reporting/docs)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
