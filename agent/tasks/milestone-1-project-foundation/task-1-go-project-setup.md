# Task 1: Go Project Setup

**Milestone**: [M1 - Project Foundation & Media Storage](../../milestones/milestone-1-project-foundation.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 2-3 hours
**Dependencies**: None
**Status**: Not Started

---

## Objective

Initialize a Go module with organized package layout, configuration management, HTTP server skeleton, Dockerfile, and Makefile. This creates the scaffolding for all subsequent development.

---

## Context

This is the first task — everything else depends on having a buildable Go project with a running HTTP server. The project uses Go for its concurrency model, I/O performance, and single-binary deployment. The structure follows standard Go project layout conventions.

---

## Steps

### 1. Initialize Go Module

```bash
go mod init github.com/prmichaelsen/cloudcut-media-server
```

### 2. Create Directory Structure

```bash
mkdir -p cmd/server
mkdir -p internal/{config,storage,media,api,ws,edl,render,progress,jobs}
mkdir -p pkg/models
```

### 3. Create Configuration Management

`internal/config/config.go` — Load config from environment variables with sensible defaults:
- PORT (default 8080)
- ENV (default "development")
- GCP_PROJECT_ID
- GCS_BUCKET_NAME
- GCS_SIGNED_URL_EXPIRY (default 3600)
- FFMPEG_PATH (default "ffmpeg")
- PROXY_RESOLUTION (default 720)
- PROXY_BITRATE (default "1M")

### 4. Create HTTP Server Entry Point

`cmd/server/main.go` — Minimal main that:
- Loads config
- Sets up router with health check endpoint
- Starts HTTP server with graceful shutdown (os.Signal handling)

### 5. Create API Router

`internal/api/router.go` — HTTP router (use standard library `net/http` mux or chi router):
- GET /health — returns 200 OK
- Middleware: request logging, CORS, recovery

### 6. Create Dockerfile

Multi-stage build:
- Stage 1: Go builder (compile binary)
- Stage 2: Runtime with FFmpeg installed (debian-slim + ffmpeg)
- Copy binary, expose port, set entrypoint

### 7. Create Makefile

Common targets:
- `build` — go build
- `run` — go run cmd/server/main.go
- `test` — go test ./...
- `docker-build` — docker build
- `docker-run` — docker run
- `lint` — golangci-lint (if available)

### 8. Create .env.example

Document all environment variables with example values.

### 9. Create .gitignore

Go-specific ignores (binary, vendor/, .env, etc.)

---

## Verification

- [ ] `go build ./cmd/server` produces binary without errors
- [ ] `go run ./cmd/server` starts server on configured port
- [ ] GET /health returns 200 OK with JSON body
- [ ] `docker build .` succeeds
- [ ] `make build` and `make test` work
- [ ] .env.example documents all required env vars
- [ ] Directory structure matches specification

---

## Expected Output

```
cloudcut-media-server/
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── .env.example
├── .gitignore
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   └── api/
│       ├── router.go
│       └── middleware.go
└── pkg/
    └── models/
        └── media.go
```

---

**Next Task**: [Task 2: GCS Integration](task-2-gcs-integration.md)
