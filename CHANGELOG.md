# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
