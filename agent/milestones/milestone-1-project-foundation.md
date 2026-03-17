# Milestone 1: Project Foundation & Media Storage

**Goal**: Establish Go project structure with GCS integration and FFmpeg-based proxy generation
**Duration**: 1-2 weeks
**Dependencies**: None
**Status**: Not Started

---

## Overview

This milestone creates the foundational Go server with media asset management. By the end, the server can accept video uploads, store them in GCS, generate low-resolution proxy files via FFmpeg, and serve media chunks via signed URLs. This is the storage and processing backbone that all subsequent features build upon.

---

## Deliverables

### 1. Go Project Structure
- Go module with organized package layout (cmd/, internal/, pkg/)
- Dockerfile for containerized deployment
- Makefile for common operations (build, test, run, docker)
- Environment configuration (.env.example)

### 2. GCS Integration
- Upload endpoint accepting multipart video uploads
- GCS bucket operations (upload, download, list, delete)
- Signed URL generation for time-limited media access
- Chunk-based serving for large files

### 3. FFmpeg Proxy Generation
- FFmpeg orchestration via os/exec pipes
- Automatic proxy generation on upload (low-res copy for client editing)
- Support for common input formats (mp4, mov, webm, mkv)

---

## Success Criteria

- [ ] `go build ./...` compiles without errors
- [ ] Video upload to GCS works via HTTP endpoint
- [ ] Signed URLs generated and serve media correctly
- [ ] FFmpeg generates proxy video from uploaded source
- [ ] Proxy and source files stored in organized GCS paths
- [ ] Health check endpoint responds

---

## Key Files to Create

```
cloudcut-media-server/
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── .env.example
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── storage/
│   │   └── gcs.go
│   ├── media/
│   │   ├── ffmpeg.go
│   │   └── proxy.go
│   └── api/
│       ├── router.go
│       ├── handlers.go
│       └── middleware.go
└── pkg/
    └── models/
        └── media.go
```

---

## Tasks

1. [Task 1: Go Project Setup](../tasks/milestone-1-project-foundation/task-1-go-project-setup.md) - Initialize Go module, directory structure, config, Dockerfile, Makefile
2. [Task 2: GCS Integration](../tasks/milestone-1-project-foundation/task-2-gcs-integration.md) - Implement upload, signed URLs, and chunk-based serving
3. [Task 3: FFmpeg Proxy Generation](../tasks/milestone-1-project-foundation/task-3-ffmpeg-proxy-generation.md) - Orchestrate FFmpeg for proxy video creation on upload

---

## Environment Variables

```env
# Server
PORT=8080
ENV=development

# GCS
GCP_PROJECT_ID=your-project-id
GCS_BUCKET_NAME=cloudcut-media
GCS_SIGNED_URL_EXPIRY=3600

# FFmpeg
FFMPEG_PATH=/usr/bin/ffmpeg
PROXY_RESOLUTION=720
PROXY_BITRATE=1M
```

---

## Testing Requirements

- [ ] Unit tests for GCS client (mock storage)
- [ ] Unit tests for FFmpeg command builder
- [ ] Integration test for upload → proxy pipeline (requires FFmpeg + GCS emulator or real bucket)
- [ ] API endpoint tests (health, upload, signed URL)

---

## Risks and Mitigation

| Risk | Impact | Probability | Mitigation Strategy |
|------|--------|-------------|---------------------|
| GCS auth complexity in dev | Medium | Medium | Use ADC (Application Default Credentials) + emulator for local dev |
| FFmpeg not available in container | High | Low | Include FFmpeg in Dockerfile (multi-stage build with ffmpeg base) |
| Large upload handling | Medium | Medium | Use streaming/multipart upload, set reasonable size limits |

---

**Next Milestone**: [Milestone 2: Persistent Connection & EDL Processing](milestone-2-persistent-connection.md)
**Blockers**: None
**Notes**: FFmpeg must be available on the host/container for proxy generation
