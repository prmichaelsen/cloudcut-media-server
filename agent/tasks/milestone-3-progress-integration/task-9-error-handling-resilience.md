# Task 9: Error Handling & Resilience

**Milestone**: [M3 - Real-Time Progress & Integration](../../milestones/milestone-3-progress-integration.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 2-3 hours
**Dependencies**: [Task 8: End-to-End Workflow](task-8-end-to-end-workflow.md)
**Status**: Not Started

---

## Objective

Harden all error paths, implement reconnection with state recovery, and ensure the server degrades gracefully under failure conditions.

---

## Context

This is the final MVP task. The workflow works end-to-end, but real-world usage involves dropped connections, FFmpeg crashes, GCS timeouts, and malformed input. This task ensures the server is resilient enough for actual use without being over-engineered.

---

## Steps

### 1. FFmpeg Error Recovery

- Detect FFmpeg crashes (non-zero exit code, signal)
- Retry once on transient failures (e.g., temp file system full)
- Report failure clearly to client with actionable error message
- Clean up partial output files on failure
- Prevent zombie FFmpeg processes (context cancellation, process group kill)

### 2. Connection Drop Handling

- When WebSocket disconnects mid-job:
  - Job continues processing (don't cancel on disconnect)
  - Buffer recent progress/status messages
  - On reconnect (same session ID), replay buffered messages
  - Grace period before session cleanup (configurable, default 5 minutes)
- Client reconnection flow:
  1. Connect with `?session_id=xxx`
  2. Server recognizes session, replays missed messages
  3. Client is back in sync

### 3. GCS Error Handling

- Retry with exponential backoff for transient GCS errors (5xx, timeout)
- Clear error messages for auth failures (403), not found (404)
- Handle upload interruption (resumable uploads for large files)

### 4. Input Validation Hardening

- Max upload size limit (configurable, default 5GB)
- Content-type validation (reject non-video uploads)
- EDL size limit (prevent DoS via massive EDL)
- Rate limiting on upload and render endpoints (basic, per-session)

### 5. Structured Error Responses

Consistent error format across REST and WebSocket:
```json
{
  "error": {
    "code": "RENDER_FAILED",
    "message": "FFmpeg exited with code 1: unsupported codec",
    "details": { "jobId": "xxx", "ffmpegExit": 1 },
    "retryable": false
  }
}
```

### 6. Graceful Server Shutdown

- On SIGTERM/SIGINT:
  - Stop accepting new connections
  - Wait for active renders to complete (with timeout)
  - Close all WebSocket connections with close frame
  - Clean up temp files

---

## Verification

- [ ] FFmpeg crash doesn't crash the server
- [ ] Client receives clear error message on render failure
- [ ] Disconnected client can reconnect and get current job status
- [ ] Jobs continue processing during client disconnection
- [ ] GCS transient errors retried automatically
- [ ] Oversized uploads rejected with 413
- [ ] Invalid content types rejected
- [ ] Server shuts down gracefully (no orphan processes, no data loss)
- [ ] All error responses follow structured format

---

**Next Task**: Post-MVP features
