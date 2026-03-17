# Task 7: Progress Streaming

**Milestone**: [M3 - Real-Time Progress & Integration](../../milestones/milestone-3-progress-integration.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 2-3 hours
**Dependencies**: [Task 4: WebSocket Server](../milestone-2-persistent-connection/task-4-websocket-server.md), [Task 6: Server-Side Rendering](../milestone-2-persistent-connection/task-6-server-side-rendering.md)
**Status**: Not Started

---

## Objective

Parse FFmpeg's progress output in real-time and stream percentage/ETA updates to the client over WebSocket during proxy generation and final export rendering.

---

## Context

Without progress feedback, the client has no idea if a render will take 10 seconds or 10 minutes. FFmpeg outputs progress data to stderr that can be parsed to calculate completion percentage. This applies to both proxy generation (M1) and export rendering (M2).

---

## Steps

### 1. Implement FFmpeg Progress Parser

`internal/progress/parser.go`:
- Parse FFmpeg stderr output line by line
- Extract: frame, fps, time, bitrate, speed
- Calculate percentage from `time` vs total duration
- Estimate remaining time from speed multiplier
- Use `-progress pipe:1` flag for structured progress output (alternative to stderr parsing)

### 2. Implement Progress Reporter

`internal/progress/reporter.go`:
- ProgressReporter — receives parsed progress, sends to client
- Throttle updates (max 2 per second to avoid flooding WebSocket)
- Format progress message:
  ```json
  {
    "type": "job.progress",
    "payload": {
      "jobId": "xxx",
      "percent": 45.2,
      "fps": 120,
      "speed": "2.1x",
      "eta": 23,
      "stage": "rendering"
    }
  }
  ```

### 3. Integrate with FFmpeg Orchestration

- Modify FFmpeg execution in `internal/media/ffmpeg.go` to pipe stderr to progress parser
- Wire ProgressReporter into proxy generation (Task 3)
- Wire ProgressReporter into export rendering (Task 6)
- Report stage transitions: downloading → rendering → uploading → complete

### 4. Handle Edge Cases

- FFmpeg outputs no progress for initial seconds (report "starting...")
- Very short videos may complete before first progress update
- Progress should reset to 0 if a new render starts

---

## Verification

- [ ] Client receives progress updates during proxy generation
- [ ] Client receives progress updates during export rendering
- [ ] Percentage is reasonably accurate (within 5%)
- [ ] ETA updates as render progresses
- [ ] Updates throttled to max 2/second
- [ ] Stage transitions reported (downloading, rendering, uploading)

---

**Next Task**: [Task 8: End-to-End Workflow](task-8-end-to-end-workflow.md)
