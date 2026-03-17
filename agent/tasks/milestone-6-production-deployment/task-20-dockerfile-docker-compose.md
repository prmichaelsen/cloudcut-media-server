# Task 20: Create Dockerfile and Docker Compose

**Status**: Not Started
**Milestone**: M6 - Production Deployment
**Estimated Hours**: 3-4
**Priority**: High

---

## Objective

Create a production-ready multi-stage Dockerfile that builds a minimal Go container with FFmpeg, and a Docker Compose configuration for local testing.

---

## Context

The server needs to be containerized for deployment to Cloud Run. The container must:
- Build the Go binary
- Include FFmpeg for video processing
- Be as small as possible (Cloud Run has size limits)
- Support health checks
- Run as non-root user

---

## Steps

### 1. Create Multi-Stage Dockerfile

**Action**: Create `Dockerfile` in project root

**Dockerfile structure**:

```dockerfile
# Stage 1: Build Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary (CGO disabled for static linking)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Stage 2: Runtime image
FROM alpine:latest

# Install FFmpeg and ca-certificates
RUN apk add --no-cache ffmpeg ca-certificates

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run server
CMD ["./server"]
```

**Action**: Create Dockerfile

### 2. Create .dockerignore

**Action**: Create `.dockerignore` to exclude unnecessary files

```
.git
.github
.claude
agent/
*.md
Dockerfile
docker-compose.yml
.gitignore
.env
*.test
testdata/
```

### 3. Create Docker Compose for Local Testing

**Action**: Create `docker-compose.yml`

```yaml
version: '3.8'

services:
  server:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - ENV=development
      - GCP_PROJECT_ID=${GCP_PROJECT_ID}
      - GCS_BUCKET_NAME=${GCS_BUCKET_NAME:-cloudcut-media-dev}
      - FFMPEG_PATH=/usr/bin/ffmpeg
    volumes:
      # Mount GCP credentials for local testing
      - ~/.config/gcloud:/home/appuser/.config/gcloud:ro
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 10s
      timeout: 3s
      retries: 3
```

### 4. Build Docker Image Locally

**Action**: Test build process

```bash
docker build -t cloudcut-media-server:local .
```

**Verify**:
- Build completes successfully
- Image size is reasonable (<200MB)
- No build errors

### 5. Run Container Locally

**Action**: Test container execution

```bash
docker-compose up
```

**Verify**:
- Container starts without errors
- Health endpoint accessible: `curl http://localhost:8080/health`
- Logs show "cloudcut-media-server starting on :8080"

### 6. Optimize Image Size

**Options to reduce size**:
- Use `alpine:latest` base (already done)
- Strip Go binary with `-ldflags="-w -s"` (already done)
- Use `scratch` base (requires static FFmpeg build, complex)
- Compile FFmpeg with only needed codecs (advanced, defer to later)

**Action**: Measure image size

```bash
docker images cloudcut-media-server:local
```

**Target**: < 200MB

### 7. Add Build Script

**Action**: Create `scripts/build-docker.sh`

```bash
#!/bin/bash
set -e

VERSION=${1:-latest}
IMAGE_NAME="cloudcut-media-server"

echo "Building Docker image: ${IMAGE_NAME}:${VERSION}"

docker build -t ${IMAGE_NAME}:${VERSION} .

echo "Build complete!"
echo "Image size:"
docker images ${IMAGE_NAME}:${VERSION} --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"

echo ""
echo "To run locally:"
echo "  docker run -p 8080:8080 ${IMAGE_NAME}:${VERSION}"
echo ""
echo "To test with docker-compose:"
echo "  docker-compose up"
```

**Action**: Make executable and test

```bash
chmod +x scripts/build-docker.sh
./scripts/build-docker.sh
```

---

## Verification

- [ ] Dockerfile created with multi-stage build
- [ ] .dockerignore created
- [ ] docker-compose.yml created
- [ ] Docker image builds successfully
- [ ] Image size < 200MB
- [ ] Container runs locally
- [ ] Health endpoint returns 200 OK
- [ ] Server starts without errors in logs
- [ ] FFmpeg available in container (`docker exec <container> ffmpeg -version`)
- [ ] Container runs as non-root user
- [ ] Build script created and works

---

## Definition of Done

- Dockerfile and docker-compose.yml created
- Image builds successfully locally
- Container runs and passes health checks
- Build script created
- Committed to repository

---

## Dependencies

**Blocking**:
- Go server built and tested (M2 complete)

**Required**:
- Docker installed locally
- Docker Compose installed

---

## Notes

- FFmpeg in Alpine is pre-compiled with common codecs
- Cloud Run requires images in Artifact Registry
- Non-root user improves security
- Health checks required for Cloud Run graceful shutdown
- Static linking (CGO_ENABLED=0) avoids glibc dependencies

---

## Related Documents

- [Dockerfile best practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [Cloud Run container requirements](https://cloud.google.com/run/docs/container-contract)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
