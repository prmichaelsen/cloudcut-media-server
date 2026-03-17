# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-03-17

### Added
- GCS client (upload, download, signed URL generation, delete)
- Media upload endpoint (POST /api/v1/media/upload) with streaming to GCS
- Media status endpoint (GET /api/v1/media/{id})
- Signed URL endpoints for source and proxy media
- Path conventions: sources/{id}/original.{ext}, proxies/{id}/proxy.mp4, exports/{session}/{ts}.mp4
- Content-type validation (mp4, mov, webm, mkv, avi)
- 5GB max upload size enforcement
- Structured error response format

## [0.2.0] - 2026-03-17

### Added
- Go project structure (cmd/server, internal/, pkg/)
- HTTP server with graceful shutdown
- Health check endpoint (GET /health)
- Request logging, CORS, and panic recovery middleware
- Configuration management from environment variables
- Media model with status lifecycle
- Dockerfile (multi-stage build with FFmpeg)
- Makefile (build, run, test, docker-build, docker-run)

## [0.1.0] - 2026-03-17

### Added
- MVP project plan: 3 milestones, 9 tasks, ~34 estimated hours
  - M1: Project Foundation & Media Storage (Go setup, GCS, FFmpeg proxy)
  - M2: Persistent Connection & EDL Processing (WebSocket, EDL schema, rendering)
  - M3: Real-Time Progress & Integration (progress streaming, e2e workflow, resilience)
- Project requirements and architecture design document
- Architecture decisions: proxy editing model, chunk-based streaming, EDL state sync, warm instances, client/server boundary
- GCP service mapping (GCE/GKE, Cloud Run, GCS, Vertex AI, Cloud CDN)
- Go selected as server language for concurrency, I/O performance, and gRPC ecosystem
- MVP success criteria defined
