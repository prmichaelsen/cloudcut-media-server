# Milestone 2: Persistent Connection & EDL Processing

**Goal**: Implement WebSocket server and server-side rendering from edit decision lists
**Duration**: 1-2 weeks
**Dependencies**: M1 - Project Foundation & Media Storage
**Status**: Not Started

---

## Overview

This milestone adds the real-time communication layer and the core editing pipeline. The server maintains persistent WebSocket connections with clients, receives edit decision lists (EDL) describing timeline edits, and renders final exports via FFmpeg based on those instructions. This is where the server becomes an interactive editing backend rather than just a storage service.

---

## Deliverables

### 1. WebSocket Server
- Persistent WebSocket connections with goroutine-per-connection model
- Heartbeat/ping-pong for connection health monitoring
- Reconnection support (client can resume session)
- Message protocol (JSON-based command/response)

### 2. EDL Schema & Parsing
- EDL JSON schema definition (timeline, tracks, clips, transitions)
- EDL validation and parsing
- Support for basic operations: cut, trim, concatenate, overlay text

### 3. Server-Side Rendering
- FFmpeg complex filter graph construction from EDL
- Full-resolution rendering pipeline
- Output format configuration (mp4/h264 default)
- Rendered output stored in GCS with signed URL for download

---

## Success Criteria

- [ ] WebSocket connection established and maintained with heartbeat
- [ ] Client can send EDL over WebSocket
- [ ] Server validates and parses EDL correctly
- [ ] Server renders final video from EDL via FFmpeg
- [ ] Rendered output accessible via signed GCS URL
- [ ] Connection survives brief network interruptions (reconnect)

---

## Key Files to Create

```
internal/
├── ws/
│   ├── server.go
│   ├── connection.go
│   ├── message.go
│   └── session.go
├── edl/
│   ├── schema.go
│   ├── parser.go
│   └── validate.go
└── render/
    ├── pipeline.go
    ├── ffmpeg_filter.go
    └── export.go
```

---

## Tasks

1. [Task 4: WebSocket Server](../tasks/milestone-2-persistent-connection/task-4-websocket-server.md) - Implement WebSocket with heartbeat and session management
2. [Task 5: EDL Schema & Parsing](../tasks/milestone-2-persistent-connection/task-5-edl-schema-parsing.md) - Define and implement the edit decision list format
3. [Task 6: Server-Side Rendering](../tasks/milestone-2-persistent-connection/task-6-server-side-rendering.md) - Build FFmpeg rendering pipeline from EDL

---

## Testing Requirements

- [ ] WebSocket connection lifecycle tests (connect, heartbeat, disconnect, reconnect)
- [ ] EDL parsing tests (valid, invalid, edge cases)
- [ ] FFmpeg filter graph generation tests (unit, no FFmpeg needed)
- [ ] End-to-end render test (requires FFmpeg)

---

## Risks and Mitigation

| Risk | Impact | Probability | Mitigation Strategy |
|------|--------|-------------|---------------------|
| Complex FFmpeg filter graphs | High | Medium | Start with simple operations (cut/concat), add complexity incrementally |
| WebSocket scaling | Medium | Low | Goroutine-per-connection is fine for MVP; consider connection pooling later |
| EDL schema evolution | Medium | Medium | Version the schema from day 1, validate on receipt |

---

**Next Milestone**: [Milestone 3: Real-Time Progress & Integration](milestone-3-progress-integration.md)
**Blockers**: Requires M1 (GCS and FFmpeg infrastructure)
**Notes**: EDL schema should be designed with the cloudcut.media frontend in mind
