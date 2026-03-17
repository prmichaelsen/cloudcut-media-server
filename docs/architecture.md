# Architecture

## Overview

CloudCut Media Server is a Go-based video editing backend deployed on Google Cloud Run. It handles media uploads, proxy video generation via FFmpeg, Edit Decision List (EDL) validation, video rendering, and WebSocket connections for real-time updates.

## System Architecture

```
┌─────────────┐
│   Client    │
│  (Browser)  │
└──────┬──────┘
       │
       │ HTTPS / WebSocket
       ▼
┌─────────────────────────────────┐
│     Cloud Run Service           │
│  (cloudcut-media-server)        │
│                                 │
│  ┌──────────────────────────┐  │
│  │  Go HTTP Server          │  │
│  │  - REST API              │  │
│  │  - WebSocket Server      │  │
│  │  - Request Logging       │  │
│  └──────────────────────────┘  │
│                                 │
│  ┌──────────────────────────┐  │
│  │  Render Engine           │  │
│  │  - FFmpeg Integration    │  │
│  │  - Job Queue             │  │
│  │  - Progress Tracking     │  │
│  └──────────────────────────┘  │
└────────┬────────────┬───────────┘
         │            │
         │            │
    ┌────▼─────┐  ┌──▼──────────┐
    │   GCS    │  │  Secret     │
    │  Bucket  │  │  Manager    │
    └──────────┘  └─────────────┘
```

## Components

### Cloud Run Service

- **Image**: `us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server`
- **Region**: us-central1
- **Instances**: 0-10 (auto-scaling based on load)
- **Memory**: 512Mi per instance
- **CPU**: 1 vCPU per instance
- **Timeout**: 60s (HTTP), 60 minutes (WebSocket)
- **Concurrency**: 80 requests per instance

**Key Features**:
- Automatic HTTPS with managed certificates
- Auto-scaling from 0 to handle traffic spikes
- Graceful shutdown and health checks
- Structured logging to Cloud Logging
- Error reporting to Error Reporting

### Google Cloud Storage (GCS)

- **Bucket**: `cloudcut-media-PROJECT_ID`
- **Location**: us-central1
- **Storage Class**: STANDARD

**Directory Structure**:
```
gs://cloudcut-media-PROJECT_ID/
├── sources/      # Original uploaded videos
│   └── {uuid}.mp4
├── proxies/      # Low-resolution proxy files
│   └── {uuid}-proxy.mp4
└── exports/      # Rendered outputs (auto-deleted after 30 days)
    └── {project-id}-{timestamp}.mp4
```

**Lifecycle Policy**:
- Exports automatically deleted after 30 days to save costs
- Sources and proxies retained indefinitely

### Artifact Registry

- **Repository**: `cloudcut`
- **Location**: us-central1-docker.pkg.dev/PROJECT_ID/cloudcut
- **Format**: Docker

**Images**:
- `cloudcut-media-server:latest` - Latest build (updated on main branch pushes)
- `cloudcut-media-server:{commit-sha}` - Version-specific builds for rollback

### Secret Manager

- **Secrets**: JWT keys, API keys (future)
- **Access**: Via service account with `secretAccessor` role
- **Rotation**: Manual for MVP, automatic rotation planned

## Data Flow

### Upload Flow

1. **Client uploads video** → POST `/api/v1/media/upload`
2. **Server generates UUID** for media item
3. **File streamed to GCS** `/sources/{uuid}.mp4`
4. **FFmpeg generates proxy** asynchronously
   - Low-resolution (720p) proxy created
   - Saved to `/proxies/{uuid}-proxy.mp4`
5. **Metadata stored** in memory (future: database)
6. **Client receives response** with media ID and signed URLs

**Timeline**: ~1-5 seconds for upload + 10-60 seconds for proxy generation (background)

### Render Flow

1. **Client sends EDL** → WebSocket `/ws` or REST `/api/v1/render`
2. **Server validates EDL**:
   - Media references exist
   - Time ranges valid
   - Clip order correct
   - Output format supported
3. **Render job queued** (in-memory queue, future: Cloud Tasks)
4. **FFmpeg processes EDL**:
   - Concatenates clips from GCS
   - Applies filters (brightness, contrast, crop, text, plugins)
   - Encodes to output format
5. **Output uploaded to GCS** `/exports/{project-id}-{timestamp}.mp4`
6. **Client notified** via WebSocket (progress updates during render)
7. **Client downloads** via signed URL (time-limited, secure)

**Timeline**: Varies based on video length and complexity (1-10 minutes typical)

### WebSocket Flow

1. **Client connects** → WebSocket `/ws`
2. **Server creates session** with UUID
3. **Heartbeat every 30s** to keep connection alive
4. **Server sends updates**:
   - Render progress (percent, FPS, ETA)
   - Job completion
   - Errors
5. **Client sends EDL submissions**
6. **Session cleanup** on disconnect (automatic goroutine cleanup)

**Max connection time**: 60 minutes (Cloud Run WebSocket limit)

## Scaling

### Auto-Scaling

**Metrics**:
- Primary: Concurrent requests
- Target: 80 requests per instance
- Min instances: 0 (scale to zero for cost savings)
- Max instances: 10 (prevent runaway costs)

**Scaling behavior**:
- **Cold start**: 2-5 seconds with `--cpu-boost`
- **Scale up**: New instances spin up when current instances reach 80% capacity
- **Scale down**: Instances shut down after 15 minutes of idle time

### Concurrency Model

- **Goroutines** for WebSocket sessions (1 per connection)
- **Goroutines** for FFmpeg jobs (1 per render)
- **Graceful shutdown** waits for in-flight jobs to complete (up to 10 seconds)

**Capacity**:
- **Per instance**: ~50-100 concurrent WebSocket connections
- **Cluster**: 500-1000 concurrent connections (10 instances)

## Security

### Authentication

- **MVP**: Unauthenticated (public endpoints)
- **Future**: JWT-based authentication with Secret Manager

### Authorization

**Service Account**: `cloudcut-server@PROJECT_ID.iam.gserviceaccount.com`

**IAM Roles** (minimal privileges):
- `storage.objectAdmin` - Read/write GCS objects (scoped to cloudcut-media bucket)
- `secretmanager.secretAccessor` - Read secrets (future auth keys)
- `logging.logWriter` - Write structured logs
- `errorreporting.writer` - Report errors

**No permissions for**:
- Compute instance creation
- IAM role modification
- Billing access
- Other GCS buckets

### Network Security

- **HTTPS only** - Cloud Run auto-generates TLS certificates
- **WebSocket over TLS** - wss:// protocol
- **Signed URLs** - Time-limited access to GCS media (15 minutes default)
- **CORS** - Configured for browser uploads (restrict origin in production)

## Performance

### Latency

- **Health check**: < 10ms
- **Media upload** (1GB): ~5-15 seconds (network-bound)
- **Proxy generation**: 10-60 seconds (depends on video length)
- **EDL validation**: < 100ms
- **Render submission**: < 500ms
- **Render execution**: 1-10 minutes (depends on complexity)

### Throughput

- **Uploads**: Limited by GCS bandwidth (~100 MB/s per region)
- **Concurrent renders**: ~10-50 simultaneous (FFmpeg CPU-bound)
- **WebSocket messages**: ~1000/second cluster-wide

## Cost Breakdown

### MVP (Low Traffic)

| Service | Usage | Cost/Month |
|---------|-------|------------|
| Cloud Run | 100 hours/month | $1-3 |
| GCS Storage | 5 GB | $0.10 |
| GCS Operations | 10k ops | Free tier |
| Artifact Registry | 500 MB | Free tier |
| Secret Manager | 1 secret | Free tier |
| Cloud Logging | 5 GB | Free tier |
| **Total** | | **$1-5/month** |

### Production (Moderate Traffic)

| Service | Usage | Cost/Month |
|---------|-------|------------|
| Cloud Run | 2000 hours/month | $20-50 |
| GCS Storage | 100 GB | $2-5 |
| GCS Bandwidth | 500 GB egress | $40-80 |
| Cloud Logging | 20 GB | $5-10 |
| Secret Manager | 3 secrets | $0.18 |
| **Total** | | **$70-150/month** |

**Cost optimization**:
- Scale to zero when not in use
- Delete exports after 30 days
- Use proxy files for client previews (smaller bandwidth)
- Batch render jobs to reduce instance cycling

## Dependencies

### Runtime Dependencies

- **FFmpeg** 8.0.1 - Video transcoding and rendering
- **Go** 1.25 - Application runtime
- **Alpine Linux** - Base container OS

### External Services

- **Google Cloud Storage API** - Media storage
- **Secret Manager API** - Credentials (future)
- **Cloud Logging API** - Structured logging
- **Error Reporting API** - Error aggregation

### Internal Packages

- `internal/api` - HTTP REST API handlers
- `internal/ws` - WebSocket server and sessions
- `internal/render` - FFmpeg rendering engine
- `internal/edl` - EDL parsing and validation
- `internal/storage` - GCS client wrapper
- `internal/media` - Proxy generation
- `internal/config` - Environment configuration
- `internal/logger` - Structured logging
- `internal/middleware` - Request logging middleware

## Disaster Recovery

### Backups

- **GCS**: 99.999999999% durability (11 9's)
- **Versioning**: Enabled on bucket (30-day retention)
- **Artifact Registry**: Images retained indefinitely
- **Metadata**: In-memory only (future: database with automated backups)

### Rollback

**Scenarios**:
1. **Bad deployment** → Re-run previous GitHub Actions workflow
2. **Performance regression** → Deploy previous commit SHA
3. **Data corruption** → Restore GCS object versions

**RTO** (Recovery Time Objective): < 5 minutes
**RPO** (Recovery Point Objective): 0 (no data loss for GCS)

### High Availability

- **Cloud Run**: Regional (us-central1) with automatic failover
- **GCS**: Multi-region replication (future upgrade)
- **Health checks**: Automatic instance replacement on failure

## Future Enhancements

### Near-term (M7+)

- JWT authentication
- Persistent database (Cloud SQL)
- Plugin system for custom effects
- WebSocket reconnection with session recovery

### Long-term

- Multi-region deployment
- CDN for media delivery
- Real-time collaborative editing
- Vertex AI for smart video analysis
- Kubernetes for more complex orchestration

## Related Documents

- [Deployment Guide](deployment.md)
- [Operations Runbook](runbook.md)
- [Monitoring](monitoring.md)
- [Infrastructure](infrastructure.md)
- [CI/CD Pipeline](cicd.md)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
