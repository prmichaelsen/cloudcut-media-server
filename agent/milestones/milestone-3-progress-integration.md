# Milestone 3: Real-Time Progress & Integration

**Goal**: Stream processing progress to clients and deliver end-to-end MVP workflow
**Duration**: 1 week
**Dependencies**: M2 - Persistent Connection & EDL Processing
**Status**: Not Started

---

## Overview

This milestone closes the loop on the MVP. Transcoding and rendering progress streams to the client in real-time over WebSocket. The full workflow — upload, proxy generation, editing via EDL, export rendering, and download — works end-to-end. Error handling and graceful degradation are hardened for a usable product.

---

## Deliverables

### 1. Progress Streaming
- FFmpeg progress parsing (frame, fps, time, bitrate, speed)
- Real-time progress events pushed to client over WebSocket
- Progress for both proxy generation and final export

### 2. End-to-End Workflow
- Complete pipeline: upload → proxy → edit (EDL) → export → download
- API documentation for client integration
- Workflow state management (job status tracking)

### 3. Error Handling & Resilience
- Graceful error responses for all failure modes
- FFmpeg crash recovery (retry, report failure)
- Connection drop handling (job continues, client can reconnect and get status)
- Basic request validation and rate limiting

---

## Success Criteria

- [ ] Client receives real-time transcoding progress (percentage, ETA)
- [ ] Full workflow works: upload → proxy → EDL submit → render → download
- [ ] Server handles FFmpeg failures gracefully (reports error, doesn't crash)
- [ ] Disconnected client can reconnect and get current job status
- [ ] API documented for cloudcut.media frontend integration

---

## Key Files to Create

```
internal/
├── progress/
│   ├── parser.go
│   └── reporter.go
├── jobs/
│   ├── manager.go
│   ├── status.go
│   └── worker.go
└── api/
    └── docs.go
```

---

## Tasks

1. [Task 7: Progress Streaming](../tasks/milestone-3-progress-integration/task-7-progress-streaming.md) - Parse FFmpeg progress and push to client via WebSocket
2. [Task 8: End-to-End Workflow](../tasks/milestone-3-progress-integration/task-8-end-to-end-workflow.md) - Wire up complete upload-to-download pipeline with job management
3. [Task 9: Error Handling & Resilience](../tasks/milestone-3-progress-integration/task-9-error-handling-resilience.md) - Harden error paths, reconnection, and graceful degradation

---

## Testing Requirements

- [ ] Progress parsing unit tests (FFmpeg output samples)
- [ ] Job lifecycle tests (queued → processing → complete/failed)
- [ ] Reconnection + status recovery integration test
- [ ] End-to-end smoke test (upload real video, get export back)

---

## Risks and Mitigation

| Risk | Impact | Probability | Mitigation Strategy |
|------|--------|-------------|---------------------|
| FFmpeg progress output format varies | Medium | Medium | Test against multiple FFmpeg versions, parse defensively |
| Job state lost on server restart | High | Medium | Persist job status to disk/database; acceptable loss for MVP |
| End-to-end test flakiness | Low | High | Use small test videos, deterministic FFmpeg settings |

---

**Next Milestone**: Post-MVP (AI features, WebRTC preview, autoscaling)
**Blockers**: Requires M2 (WebSocket and rendering pipeline)
**Notes**: This milestone makes the server usable by the cloudcut.media frontend
