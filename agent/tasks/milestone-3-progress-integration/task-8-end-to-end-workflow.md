# Task 8: End-to-End Workflow

**Milestone**: [M3 - Real-Time Progress & Integration](../../milestones/milestone-3-progress-integration.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 3-4 hours
**Dependencies**: [Task 7: Progress Streaming](task-7-progress-streaming.md)
**Status**: Not Started

---

## Objective

Wire up the complete upload-to-download pipeline with job management, ensuring the full MVP workflow works end-to-end and is documented for frontend integration.

---

## Context

All the individual pieces exist — this task connects them into a cohesive workflow and adds the job management layer to track operations through their lifecycle. This is also where we document the API contract for the cloudcut.media frontend team.

---

## Steps

### 1. Implement Job Manager

`internal/jobs/manager.go`:
- JobManager — tracks all active and recent jobs
- Job struct: ID, Type (proxy/export), SessionID, MediaID, Status, Progress, CreatedAt, CompletedAt, Error, ResultURL
- Status enum: queued, downloading, rendering, uploading, complete, failed
- CreateJob, UpdateJob, GetJob, ListJobsForSession

### 2. Implement Job Worker

`internal/jobs/worker.go`:
- Process jobs from queue
- Configurable concurrency (default: 2 concurrent jobs)
- Job types: proxy_generation, export_render
- Each job type calls appropriate pipeline (proxy.go or pipeline.go)
- Update job status at each stage

### 3. Wire Complete Workflow

Connect all components:
1. **Upload**: POST /api/v1/media/upload → GCS → create proxy job
2. **Proxy**: Job worker generates proxy → WebSocket `media.status` update
3. **Edit**: Client works with proxy (client-side, no server involvement)
4. **Submit EDL**: WebSocket `edl.submit` → validate → create export job
5. **Render**: Job worker renders export → WebSocket `job.progress` updates
6. **Download**: `job.complete` with signed URL → client downloads

### 4. Add Status Endpoints

REST endpoints for polling (fallback when WebSocket unavailable):
- GET /api/v1/media/{id} — media status and metadata
- GET /api/v1/jobs/{id} — job status and progress
- GET /api/v1/sessions/{id}/jobs — list jobs for session

### 5. Document API Contract

Create API documentation covering:
- All REST endpoints (method, path, request/response schemas)
- WebSocket message protocol (all message types)
- EDL schema with examples
- Authentication expectations (placeholder for future)
- Error response format

---

## Verification

- [ ] Full workflow works: upload → proxy → EDL → render → download
- [ ] Job manager correctly tracks job lifecycle
- [ ] WebSocket receives all status updates throughout workflow
- [ ] REST fallback endpoints return correct job/media status
- [ ] Concurrent jobs execute without interference
- [ ] API documentation is complete and accurate

---

**Next Task**: [Task 9: Error Handling & Resilience](task-9-error-handling-resilience.md)
