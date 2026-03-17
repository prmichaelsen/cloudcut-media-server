# Task 2: GCS Integration

**Milestone**: [M1 - Project Foundation & Media Storage](../../milestones/milestone-1-project-foundation.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 3-4 hours
**Dependencies**: [Task 1: Go Project Setup](task-1-go-project-setup.md)
**Status**: Not Started

---

## Objective

Implement Google Cloud Storage integration for uploading, storing, and serving video assets with signed URLs and chunk-based access.

---

## Context

GCS is the primary media storage backend. All video files (source and proxy) live in GCS. The client never gets raw bucket URLs — only time-limited signed URLs. This task establishes the storage layer that the proxy generator and rendering pipeline will depend on.

---

## Steps

### 1. Add GCS Dependencies

```bash
go get cloud.google.com/go/storage
```

### 2. Implement GCS Client

`internal/storage/gcs.go`:
- NewGCSClient(config) — initialize with project ID and bucket name
- Upload(ctx, objectPath, reader) — stream upload to GCS
- Download(ctx, objectPath) — return io.ReadCloser for streaming download
- SignedURL(objectPath, expiry) — generate signed URL for client access
- Delete(ctx, objectPath) — remove object
- Organize paths: `sources/{mediaID}/original.{ext}`, `proxies/{mediaID}/proxy.mp4`

### 3. Implement Upload Handler

`internal/api/handlers.go`:
- POST /api/v1/media/upload — accept multipart upload
- Generate unique media ID (UUID)
- Stream to GCS (don't buffer entire file in memory)
- Return media ID and metadata (size, duration if detectable, GCS path)

### 4. Implement Signed URL Handler

- GET /api/v1/media/{id}/url — return signed URL for source
- GET /api/v1/media/{id}/proxy/url — return signed URL for proxy
- Query param for expiry override (within server max)

### 5. Create Media Model

`pkg/models/media.go`:
- Media struct: ID, OriginalFilename, ContentType, Size, GCSSourcePath, GCSProxyPath, Status, CreatedAt
- Status enum: uploading, processing, ready, error

### 6. Add Range Request Support

Ensure signed URLs support HTTP range requests (GCS does this natively). Document for frontend how to use range headers for chunk-based playback.

---

## Verification

- [ ] Upload endpoint accepts video file and stores in GCS
- [ ] Media ID returned and can be used to retrieve signed URLs
- [ ] Signed URLs are time-limited and actually serve the video
- [ ] GCS paths are organized (sources/, proxies/)
- [ ] Large files stream without buffering entire file in memory
- [ ] Error handling for invalid uploads (wrong content type, too large)

---

**Next Task**: [Task 3: FFmpeg Proxy Generation](task-3-ffmpeg-proxy-generation.md)
