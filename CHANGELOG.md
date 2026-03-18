# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2026-03-18

### Added
- Plugin system foundation with registry, manifest parser, and Go plugin loader
- Plugin interface: Plugin, VideoEffectPlugin, AudioEffectPlugin
- Thread-safe plugin registry with lifecycle management (Register, Activate, Deactivate)
- YAML plugin manifest schema with parameter type validation (float, int, string, bool)
- Go plugin loader with dynamic .so loading and NewPlugin symbol resolution
- Built-in plugin registration for plugins compiled into the server binary
- Plugin-aware EDL validation (ValidateWithPlugins) checks filter types against registry
- FFmpeg builder plugin invocation: falls back to plugin registry for unknown filter types
- Sample film-grain plugin demonstrating full plugin lifecycle
- Film-grain effect: maps intensity (0.0-1.0) to FFmpeg noise filter (strength 0-30)
- Plugin manifest discovery from `plugins/` directory
- 21+ unit tests across plugin system packages

## [1.0.0] - 2026-03-18

### Added
- TypeScript SDK (`sdk/typescript/`) with REST client and WebSocket client
- `CloudCutClient` class for REST API operations (upload, media status, job polling)
- `CloudCutWS` class for WebSocket communication with auto-reconnect
- Full type definitions for all API types (Media, Job, EDL, etc.)
- `waitForJob()` and `waitForMedia()` polling helpers
- Event-driven WebSocket with typed listeners (progress, complete, error)
- OpenAPI 3.0 specification (`api/openapi.yaml`)
- WebSocket protocol specification (`api/websocket-protocol.md`)
- Updated API reference docs with cross-links and quick reference table

## [0.8.0] - 2026-03-18

### Added
- Structured error types (`internal/errors/errors.go`) with code, message, details, retryable fields
- Exponential backoff retry logic (`internal/retry/retry.go`) with configurable attempts and wait times
- Input validation package (`internal/validation/validation.go`) for upload size/type checks
- Retry on transient GCS errors (5xx, timeout, connection reset) in upload/download
- Structured error responses across all REST endpoints
- GCS transient error detection (500, 502, 503, 504, timeout)
- FFmpeg transient error detection (disk full, temporary failures)
- Validation for upload size (max 5GB) and content type (mp4, mov, webm, mkv, avi)
- Consistent error format: `{"error": {"code": "...", "message": "...", "retryable": bool}}`

### Changed
- GCS upload now retries up to 3 times on transient errors with exponential backoff
- GCS download now retries up to 3 times on transient errors
- Upload handler uses structured validation and error responses
- Moved content type inference to validation package

## [0.7.0] - 2026-03-18

### Added
- Job manager (`internal/jobs/manager.go`) with CRUD operations for tracking processing jobs
- Job worker pool (`internal/jobs/worker.go`) with configurable concurrency (default: 2 workers)
- Complete end-to-end workflow: upload → proxy generation → EDL submission → render → download
- REST endpoints: `GET /api/v1/jobs/{id}`, `GET /api/v1/sessions/{id}/jobs`
- Job types: proxy_generation, export_render
- Job statuses: queued, downloading, rendering, uploading, complete, failed
- Real-time job progress reporting via WebSocket integrated with worker pool
- API documentation (`docs/api.md`) covering REST and WebSocket protocols
- Graceful worker shutdown on server stop
- Background proxy generation now uses job system with progress tracking

### Changed
- Refactored proxy generation to use job worker instead of standalone goroutine
- EDL submission now creates jobs managed by worker pool
- WebSocket progress updates now sourced from job manager state

## [0.6.0] - 2026-03-18

### Added
- FFmpeg progress parser (`internal/progress/parser.go`) with stderr parsing
- Progress reporter with throttling to 2 updates/second (`internal/progress/reporter.go`)
- Progress callback support in `RunFFmpegWithProgress()` for proxy generation
- Progress tracking in render pipeline via `FFmpegRenderer.Run()`
- Stage transitions: downloading, rendering, uploading, complete
- Real-time progress streaming over WebSocket with percent, fps, speed, eta fields
- Unit tests for progress parser covering valid/invalid FFmpeg output lines

## [0.5.0] - 2026-03-17

### Added
- WebSocket server with persistent connections (nhooyr.io/websocket)
- Session management with reconnection support and message buffering
- Connection lifecycle: heartbeat ping/pong, read/write pumps, graceful close
- JSON message protocol (ping, pong, edl.submit, job.progress, job.complete, job.error, media.status)
- Session cleanup for expired disconnected sessions (5min grace period)
- GET /ws endpoint for WebSocket upgrade with ?session_id= for reconnection

## [0.4.1] - 2026-03-17

### Added
- Unit tests for config loading (defaults, env overrides, invalid int fallback)
- Unit tests for FFmpeg command builder (proxy args, custom resolution)
- Unit tests for GCS path helpers (SourcePath, ProxyPath, ExportPath)
- Unit tests for HTTP health endpoint (response, content-type, CORS preflight)
- Unit tests for content type inference (mp4, mov, webm, mkv, avi)
- 11 tests across 4 packages, all passing

## [0.4.0] - 2026-03-17

### Added
- FFmpeg command builder for proxy generation (libx264, configurable resolution/bitrate)
- Proxy generation pipeline: download source → FFmpeg transcode → upload proxy to GCS
- Automatic proxy trigger on upload (background goroutine)
- Media status lifecycle: uploading → processing → ready (or error)
- ffprobe integration for media duration detection
- Temp file cleanup after proxy generation
- FFmpeg timeout support (default 10 minutes)

### Changed
- Upload handler now triggers background proxy generation
- Handlers accept ProxyGenerator dependency

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
