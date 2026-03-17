# Task 3: FFmpeg Proxy Generation

**Milestone**: [M1 - Project Foundation & Media Storage](../../milestones/milestone-1-project-foundation.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 3-4 hours
**Dependencies**: [Task 2: GCS Integration](task-2-gcs-integration.md)
**Status**: Not Started

---

## Objective

Implement FFmpeg orchestration to automatically generate low-resolution proxy videos when media is uploaded. Proxies are used by the WASM frontend for responsive editing without downloading full-resolution source files.

---

## Context

The proxy editing model is a core architecture decision. Clients edit with lightweight proxies; the server renders at full resolution. This task builds the FFmpeg orchestration layer that will also be reused by the rendering pipeline in M2.

---

## Steps

### 1. Implement FFmpeg Command Builder

`internal/media/ffmpeg.go`:
- FFmpeg struct wrapping os/exec.Cmd
- BuildProxyCommand(inputPath, outputPath, resolution, bitrate) — construct FFmpeg args
- Proxy settings: scale to target height (e.g., 720p), fast encoding preset, low bitrate
- Example command: `ffmpeg -i input.mp4 -vf scale=-2:720 -c:v libx264 -preset fast -b:v 1M -c:a aac -b:a 128k output.mp4`

### 2. Implement Proxy Generation Pipeline

`internal/media/proxy.go`:
- GenerateProxy(ctx, mediaID) — orchestrate full pipeline:
  1. Download source from GCS to temp file (or stream via pipe)
  2. Run FFmpeg to generate proxy
  3. Upload proxy to GCS at `proxies/{mediaID}/proxy.mp4`
  4. Update media status to "ready"
  5. Clean up temp files

### 3. Trigger Proxy Generation on Upload

Wire into upload handler:
- After successful upload, kick off proxy generation in a goroutine
- Media status transitions: uploading → processing → ready (or error)
- Client can poll media status or receive notification via WebSocket (M2)

### 4. Handle FFmpeg Errors

- Capture stderr for error reporting
- Timeout for hung FFmpeg processes (configurable, default 10 minutes)
- Set media status to "error" with error message on failure
- Log FFmpeg output for debugging

### 5. Support Common Input Formats

Test and handle: mp4, mov, webm, mkv, avi. FFmpeg handles most formats natively — just ensure error messages are clear when format is unsupported.

---

## Verification

- [ ] Upload triggers automatic proxy generation
- [ ] Proxy appears in GCS at correct path
- [ ] Proxy is playable and at target resolution/bitrate
- [ ] Media status correctly transitions through uploading → processing → ready
- [ ] FFmpeg errors are caught and reported (media status → error)
- [ ] Temp files cleaned up after processing
- [ ] Works for mp4, mov, webm inputs

---

**Next Task**: [Task 4: WebSocket Server](../milestone-2-persistent-connection/task-4-websocket-server.md)
