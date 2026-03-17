# Task 6: Server-Side Rendering

**Milestone**: [M2 - Persistent Connection & EDL Processing](../../milestones/milestone-2-persistent-connection.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 4-6 hours
**Dependencies**: [Task 3: FFmpeg Proxy Generation](../milestone-1-project-foundation/task-3-ffmpeg-proxy-generation.md), [Task 5: EDL Schema & Parsing](task-5-edl-schema-parsing.md)
**Status**: Not Started

---

## Objective

Build the FFmpeg rendering pipeline that translates an EDL into a complex filter graph and produces a final full-resolution export video.

---

## Context

This is the core value of the server — the client edits with proxies, but the server renders at full resolution using the original source files. The EDL describes the edit; this task translates that description into FFmpeg commands. This is the most complex task in M2 and reuses the FFmpeg orchestration from Task 3.

---

## Steps

### 1. Implement Filter Graph Builder

`internal/render/ffmpeg_filter.go`:
- EDLToFilterGraph(edl) — translate EDL into FFmpeg complex filter graph
- Handle operations:
  - **Cut/Trim**: `-ss` and `-to` input options or trim filter
  - **Concatenate**: concat filter for joining clips
  - **Text overlay**: drawtext filter
  - **Basic color**: eq filter for brightness/contrast
- Build input list (download sources from GCS to temp dir)
- Generate filter_complex string

### 2. Implement Render Pipeline

`internal/render/pipeline.go`:
- RenderFromEDL(ctx, edl, sessionID) — full pipeline:
  1. Download all referenced source media from GCS to temp directory
  2. Build FFmpeg command with filter graph
  3. Execute FFmpeg
  4. Upload rendered output to GCS at `exports/{sessionID}/{timestamp}.mp4`
  5. Generate signed URL for download
  6. Clean up temp files
- Return export metadata (GCS path, signed URL, file size, duration)

### 3. Implement Export Configuration

`internal/render/export.go`:
- Map EDL Output config to FFmpeg encoding params
- Presets:
  - high: libx264 -crf 18 -preset slow
  - medium: libx264 -crf 23 -preset medium
  - low: libx264 -crf 28 -preset fast
- Support custom resolution (scale filter)

### 4. Wire into WebSocket Flow

When EDL is validated (from Task 5):
1. Create render job
2. Start rendering in background goroutine
3. On completion, send `job.complete` with signed download URL
4. On failure, send `job.error` with details

### 5. Handle Large Projects

- Limit concurrent renders per session (default: 1)
- Queue additional render requests
- Timeout for long renders (configurable, default 30 minutes)

---

## Verification

- [ ] Simple EDL (single clip, trimmed) renders correctly
- [ ] Multi-clip EDL (concatenation) renders correctly
- [ ] Text overlay filter works
- [ ] Output matches requested resolution and quality
- [ ] Rendered file uploaded to GCS and accessible via signed URL
- [ ] FFmpeg errors reported back to client
- [ ] Temp files cleaned up after render
- [ ] Concurrent render limit enforced

---

**Next Task**: [Task 7: Progress Streaming](../milestone-3-progress-integration/task-7-progress-streaming.md)
