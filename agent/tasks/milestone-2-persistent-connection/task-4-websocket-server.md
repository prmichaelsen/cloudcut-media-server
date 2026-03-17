# Task 4: WebSocket Server

**Milestone**: [M2 - Persistent Connection & EDL Processing](../../milestones/milestone-2-persistent-connection.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 3-4 hours
**Dependencies**: [Task 1: Go Project Setup](../milestone-1-project-foundation/task-1-go-project-setup.md)
**Status**: Not Started

---

## Objective

Implement a persistent WebSocket server with heartbeat, session management, and a JSON message protocol for bidirectional communication between the server and cloudcut.media frontend.

---

## Context

The WebSocket connection is the real-time communication backbone. It carries EDL submissions, progress updates, job status notifications, and control messages. The server uses a goroutine-per-connection model which Go handles efficiently.

---

## Steps

### 1. Add WebSocket Dependency

```bash
go get nhooyr.io/websocket
```

(nhooyr/websocket is modern, context-aware, and well-maintained. Alternative: gorilla/websocket.)

### 2. Implement Connection Manager

`internal/ws/server.go`:
- ConnectionManager — tracks active connections by session ID
- Register/Unregister connections
- Broadcast to all connections (for future use)
- Send to specific session

### 3. Implement Connection Handler

`internal/ws/connection.go`:
- HandleWebSocket(w, r) — upgrade HTTP to WebSocket
- Goroutine for reading messages (readPump)
- Goroutine for writing messages (writePump with channel)
- Heartbeat via ping/pong (configurable interval, default 30s)
- Connection timeout on missed pongs

### 4. Define Message Protocol

`internal/ws/message.go`:
- Base message: `{ "type": string, "id": string, "payload": object }`
- Message types:
  - `ping` / `pong` — heartbeat
  - `edl.submit` — client sends EDL for rendering
  - `edl.ack` — server acknowledges receipt
  - `job.progress` — server sends render/transcode progress
  - `job.complete` — server sends completion with download URL
  - `job.error` — server sends error details
  - `media.status` — server sends media status update

### 5. Implement Session Management

`internal/ws/session.go`:
- Session struct: ID, connection, media context, active jobs
- Session creation on connect
- Session recovery on reconnect (match by session ID in query param or header)
- Session cleanup on disconnect (with grace period for reconnection)

### 6. Wire into HTTP Router

- GET /ws — WebSocket upgrade endpoint
- Query param: `?session_id=xxx` for reconnection

---

## Verification

- [ ] WebSocket connection establishes successfully
- [ ] Heartbeat ping/pong works (connection stays alive)
- [ ] Server detects disconnected clients (missed pongs)
- [ ] JSON messages sent and received correctly
- [ ] Session persists across brief disconnection/reconnection
- [ ] Multiple concurrent connections handled
- [ ] Connection manager tracks active sessions accurately

---

**Next Task**: [Task 5: EDL Schema & Parsing](task-5-edl-schema-parsing.md)
