# WebSocket Protocol Specification

**Version**: 1.0
**Transport**: WebSocket (RFC 6455)
**Encoding**: JSON
**Endpoint**: `GET /ws?session_id={optional_session_id}`

## Connection

### Establishing a Connection

```
GET /ws HTTP/1.1
Upgrade: websocket
Connection: Upgrade
```

### Reconnection

To reconnect to an existing session and receive buffered messages:

```
GET /ws?session_id=<previous-session-id> HTTP/1.1
Upgrade: websocket
Connection: Upgrade
```

Sessions are retained for 5 minutes after disconnection. Messages sent during
disconnection are buffered and delivered on reconnection.

## Message Envelope

All messages use a common envelope format:

```json
{
  "type": "<message_type>",
  "id": "<optional_correlation_id>",
  "payload": { ... }
}
```

| Field     | Type   | Required | Description                          |
|-----------|--------|----------|--------------------------------------|
| `type`    | string | Yes      | Message type identifier              |
| `id`      | string | No       | Correlation ID for request/response  |
| `payload` | object | No       | Type-specific payload data           |

## Message Types

### Client -> Server

#### `ping`

Heartbeat message. Server responds with `pong`.

```json
{
  "type": "ping",
  "id": "ping-1"
}
```

#### `edl.submit`

Submit an EDL for server-side rendering.

```json
{
  "type": "edl.submit",
  "id": "submit-1",
  "payload": {
    "version": "1.0",
    "projectId": "project-123",
    "timeline": {
      "duration": 30.0,
      "tracks": [
        {
          "id": "video-1",
          "type": "video",
          "clips": [
            {
              "mediaId": "media-uuid",
              "startTime": 0,
              "duration": 15.0,
              "inPoint": 5.0,
              "outPoint": 20.0,
              "filters": []
            }
          ]
        }
      ]
    },
    "output": {
      "format": "mp4",
      "quality": "preview"
    }
  }
}
```

**Server Response**: `edl.ack` on success, `job.error` on validation failure.

### Server -> Client

#### `pong`

Response to `ping`.

```json
{
  "type": "pong",
  "id": "ping-1"
}
```

#### `edl.ack`

Acknowledgement that an EDL was validated and a render job was created.

```json
{
  "type": "edl.ack",
  "payload": {
    "projectId": "project-123",
    "jobId": "job-uuid"
  }
}
```

#### `job.progress`

Periodic progress update during rendering. Throttled to max 2 updates per second.

```json
{
  "type": "job.progress",
  "payload": {
    "jobId": "job-uuid",
    "percent": 45.2,
    "fps": 60,
    "speed": "2.1x",
    "eta": 15,
    "stage": "rendering"
  }
}
```

| Field     | Type   | Description                                      |
|-----------|--------|--------------------------------------------------|
| `jobId`   | string | Job identifier                                   |
| `percent` | number | Completion percentage (0-100)                    |
| `fps`     | int    | Current encoding frames per second               |
| `speed`   | string | Encoding speed multiplier (e.g., "2.1x")         |
| `eta`     | int    | Estimated time remaining in seconds              |
| `stage`   | string | Current stage: downloading, rendering, uploading |

#### `job.complete`

Sent when a render job finishes successfully.

```json
{
  "type": "job.complete",
  "payload": {
    "jobId": "job-uuid",
    "url": "https://storage.googleapis.com/..."
  }
}
```

| Field   | Type   | Description                              |
|---------|--------|------------------------------------------|
| `jobId` | string | Job identifier                           |
| `url`   | string | Signed URL to download the rendered file |

#### `job.error`

Sent when a job fails or EDL validation fails.

```json
{
  "type": "job.error",
  "payload": {
    "code": "RENDER_FAILED",
    "message": "FFmpeg encoding failed: invalid input format",
    "jobId": "job-uuid",
    "retryable": false
  }
}
```

| Field       | Type   | Description                                |
|-------------|--------|--------------------------------------------|
| `code`      | string | Machine-readable error code                |
| `message`   | string | Human-readable error description           |
| `jobId`     | string | Job identifier (omitted for non-job errors)|
| `retryable` | bool   | Whether the client should retry            |

#### `media.status`

Sent when a media file's processing status changes.

```json
{
  "type": "media.status",
  "payload": {
    "mediaId": "media-uuid",
    "status": "ready",
    "error": ""
  }
}
```

| Field     | Type   | Description                              |
|-----------|--------|------------------------------------------|
| `mediaId` | string | Media identifier                         |
| `status`  | string | New status: uploading, processing, ready, error |
| `error`   | string | Error message if status is "error"       |

## Session Lifecycle

```
Client                          Server
  |                               |
  |-- GET /ws ------------------>|  Connection established
  |<-- session_id assigned ------|  Session created
  |                               |
  |-- edl.submit --------------->|  EDL validated, job created
  |<-- edl.ack ------------------|  Job ID returned
  |                               |
  |<-- job.progress -------------|  Periodic updates (2/sec)
  |<-- job.progress -------------|
  |<-- job.progress -------------|
  |                               |
  |<-- job.complete -------------|  Render finished
  |                               |
  |-- [disconnect] ------------->|  Session retained (5min)
  |                               |
  |-- GET /ws?session_id=X ----->|  Reconnected
  |<-- buffered messages --------|  Missed messages delivered
  |                               |
```

## Error Codes

| Code                 | Description                         | Retryable |
|----------------------|-------------------------------------|-----------|
| `VALIDATION_FAILED`  | EDL validation failed               | No        |
| `RENDER_FAILED`      | FFmpeg rendering failed             | No        |
| `UPLOAD_FAILED`      | GCS upload failed                   | Yes       |
| `DOWNLOAD_FAILED`    | GCS download failed                 | Yes       |
| `JOB_QUEUE_FULL`     | Job queue at capacity               | Yes       |
| `INTERNAL_ERROR`     | Unexpected server error             | No        |

## Rate Limiting

- Progress updates are throttled to maximum 2 per second per job
- No rate limiting on client messages (ping, edl.submit)
- Connection limit: one WebSocket per session

## Heartbeat

- Client should send `ping` every 30 seconds
- Server responds with `pong` immediately
- Server closes connections idle for >60 seconds
