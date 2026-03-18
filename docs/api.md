# API Documentation

## Overview

The cloudcut-media-server provides a REST API for media upload/download and a WebSocket API for real-time EDL submission and progress updates.

Base URL: `https://YOUR-SERVICE-URL` or `http://localhost:8080` (local)

## REST API Endpoints

### Health Check

**GET /health**

Check server health status.

**Response:**
```json
{
  "status": "ok"
}
```

---

### Upload Media

**POST /api/v1/media/upload**

Upload a video file to the server. The server will automatically generate a proxy in the background.

**Request:**
- Content-Type: `multipart/form-data`
- Body: `file` field with video file (mp4, mov, webm, mkv, avi)
- Max size: 5GB

**Response:**
```json
{
  "id": "media-uuid",
  "filename": "video.mp4",
  "size": 1048576,
  "status": "processing",
  "gcsPath": "sources/media-uuid/original.mp4"
}
```

---

### Get Media Status

**GET /api/v1/media/{id}**

Get the status and metadata of an uploaded media file.

**Response:**
```json
{
  "id": "media-uuid",
  "originalFilename": "video.mp4",
  "contentType": "video/mp4",
  "size": 1048576,
  "gcsSourcePath": "sources/media-uuid/original.mp4",
  "gcsProxyPath": "proxies/media-uuid/proxy.mp4",
  "status": "ready",
  "createdAt": "2026-03-18T00:00:00Z"
}
```

**Status values:**
- `uploading` - File is being uploaded
- `processing` - Proxy generation in progress
- `ready` - Media is ready for editing
- `error` - An error occurred

---

### Get Source URL

**GET /api/v1/media/{id}/url**

Get a signed URL to download the original source file.

**Response:**
```json
{
  "url": "https://storage.googleapis.com/..."
}
```

---

### Get Proxy URL

**GET /api/v1/media/{id}/proxy/url**

Get a signed URL to download the proxy file.

**Response:**
```json
{
  "url": "https://storage.googleapis.com/..."
}
```

---

### Get Job Status

**GET /api/v1/jobs/{id}**

Get the status of a specific job (proxy generation or render).

**Response:**
```json
{
  "id": "job-uuid",
  "type": "export_render",
  "sessionID": "session-uuid",
  "mediaID": "media-uuid",
  "status": "rendering",
  "progress": 45.2,
  "fps": 120,
  "speed": "2.1x",
  "eta": 23,
  "stage": "rendering",
  "resultURL": "",
  "createdAt": "2026-03-18T00:00:00Z",
  "startedAt": "2026-03-18T00:00:05Z"
}
```

**Job status values:**
- `queued` - Job is waiting to start
- `downloading` - Downloading source media from GCS
- `rendering` - FFmpeg is rendering
- `uploading` - Uploading result to GCS
- `complete` - Job finished successfully
- `failed` - Job encountered an error

---

### List Session Jobs

**GET /api/v1/sessions/{id}/jobs**

Get all jobs for a specific session.

**Response:**
```json
{
  "jobs": [
    {
      "id": "job-uuid",
      "type": "export_render",
      "status": "complete",
      ...
    }
  ]
}
```

---

## WebSocket API

### Connect

**WS /ws**

Connect to the WebSocket server. Optionally provide `session_id` query parameter to reconnect to an existing session.

**URL:** `wss://YOUR-SERVICE-URL/ws` or `ws://localhost:8080/ws`

**Example:**
```javascript
// New connection
const ws = new WebSocket('wss://YOUR-SERVICE-URL/ws');

// Reconnect to existing session
const ws = new WebSocket('wss://YOUR-SERVICE-URL/ws?session_id=SESSION-UUID');
```

---

### Message Types

All WebSocket messages follow this format:

```json
{
  "type": "message_type",
  "id": "optional-message-id",
  "payload": {}
}
```

---

### Ping / Pong

Keep the connection alive with heartbeat messages.

**Client → Server (ping):**
```json
{
  "type": "ping",
  "id": "ping-1"
}
```

**Server → Client (pong):**
```json
{
  "type": "pong",
  "id": "ping-1"
}
```

---

### Submit EDL

**Client → Server (edl.submit):**

Submit an Edit Decision List for rendering.

```json
{
  "type": "edl.submit",
  "payload": {
    "projectId": "my-project",
    "version": "1.0",
    "timeline": {
      "duration": 10.5,
      "tracks": [
        {
          "type": "video",
          "clips": [
            {
              "mediaId": "media-uuid",
              "startTime": 0,
              "duration": 5.0,
              "inPoint": 0,
              "outPoint": 5.0
            }
          ]
        }
      ]
    },
    "output": {
      "format": "mp4",
      "codec": "h264",
      "quality": "high",
      "resolution": "1920x1080"
    }
  }
}
```

**Server → Client (edl.ack):**
```json
{
  "type": "edl.ack",
  "payload": {
    "projectId": "my-project",
    "jobId": "job-uuid"
  }
}
```

---

### Job Progress

**Server → Client (job.progress):**

Receive real-time progress updates during rendering.

```json
{
  "type": "job.progress",
  "payload": {
    "jobId": "job-uuid",
    "percent": 45.2,
    "fps": 120,
    "speed": "2.1x",
    "eta": 23,
    "stage": "rendering"
  }
}
```

**Stage values:**
- `downloading` - Downloading source media
- `rendering` - FFmpeg is processing
- `uploading` - Uploading result
- `complete` - Job finished

---

### Job Complete

**Server → Client (job.complete):**

Job finished successfully with download URL.

```json
{
  "type": "job.complete",
  "payload": {
    "jobId": "job-uuid",
    "url": "https://storage.googleapis.com/..."
  }
}
```

---

### Job Error

**Server → Client (job.error):**

Job encountered an error.

```json
{
  "type": "job.error",
  "payload": {
    "jobId": "job-uuid",
    "code": "RENDER_FAILED",
    "message": "FFmpeg exited with code 1: unsupported codec",
    "retryable": false
  }
}
```

---

### Media Status

**Server → Client (media.status):**

Media processing status update (e.g., proxy generation complete).

```json
{
  "type": "media.status",
  "payload": {
    "mediaId": "media-uuid",
    "status": "ready"
  }
}
```

---

## Complete Workflow Example

### 1. Upload Video

```bash
curl -X POST -F file=@video.mp4 \
  https://YOUR-SERVICE-URL/api/v1/media/upload
```

Response:
```json
{
  "id": "abc123",
  "status": "processing"
}
```

### 2. Connect WebSocket

```javascript
const ws = new WebSocket('wss://YOUR-SERVICE-URL/ws');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log('Received:', msg.type, msg.payload);
};
```

### 3. Wait for Proxy

The server will send a `media.status` message when the proxy is ready:

```json
{
  "type": "media.status",
  "payload": {
    "mediaId": "abc123",
    "status": "ready"
  }
}
```

### 4. Submit EDL

```javascript
ws.send(JSON.stringify({
  type: 'edl.submit',
  payload: {
    projectId: 'my-project',
    version: '1.0',
    timeline: { /* ... */ },
    output: { /* ... */ }
  }
}));
```

### 5. Receive Progress Updates

```json
{
  "type": "job.progress",
  "payload": {
    "jobId": "job-456",
    "percent": 50.0,
    "stage": "rendering"
  }
}
```

### 6. Download Result

```json
{
  "type": "job.complete",
  "payload": {
    "jobId": "job-456",
    "url": "https://storage.googleapis.com/..."
  }
}
```

---

## Error Handling

All errors follow this format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  }
}
```

**Common error codes:**
- `UPLOAD_TOO_LARGE` - File exceeds 5GB limit
- `INVALID_CONTENT_TYPE` - Unsupported video format
- `NOT_FOUND` - Resource not found
- `JOB_NOT_FOUND` - Job ID doesn't exist
- `RENDER_FAILED` - FFmpeg rendering failed

---

## Rate Limiting

Currently no rate limiting is enforced (MVP).

---

## CORS

CORS is enabled for all origins (`Access-Control-Allow-Origin: *`).

---

## Authentication

Currently no authentication is required (MVP). Authentication will be added in a future milestone.

---

**Last Updated:** 2026-03-18
