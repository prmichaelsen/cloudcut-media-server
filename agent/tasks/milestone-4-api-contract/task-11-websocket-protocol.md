# Task 11: Document WebSocket Protocol

**Status**: Not Started
**Milestone**: M4 - Stable API Contract
**Estimated Hours**: 3-4
**Priority**: High

---

## Objective

Create comprehensive documentation of the WebSocket protocol including message types, payload schemas, connection lifecycle, and error handling to enable custom backend implementations.

---

## Context

The server uses WebSocket for real-time bidirectional communication between client and server. Custom backends must implement the same protocol to be compatible with the cloudcut.media client.

**Current implementation**: `internal/ws/` (server.go, connection.go, message.go, session.go)

**Design reference**: `agent/design/plugin-architecture-backend.md` § WebSocket Protocol

---

## Steps

### 1. Create Protocol Documentation File

**Action**: Create `docs/websocket-protocol.md`

Structure:
- Overview
- Connection Lifecycle
- Message Format
- Message Types
- Error Handling
- Examples

### 2. Document Connection Lifecycle

**Phases to document**:
1. **Connection** - Client opens WebSocket to `/ws`
2. **Session Creation** - Server assigns session ID, returns in first message
3. **Authentication** - (Future) API key verification
4. **Active** - Bidirectional message exchange
5. **Disconnection** - Client disconnects, server maintains session for 5min grace period
6. **Reconnection** - Client reconnects with session ID, server restores message buffer

**Action**: Add Connection Lifecycle section with state diagram

### 3. Define Base Message Format

Extract from `internal/ws/message.go`:

```json
{
  "type": "message-type",
  "id": "optional-correlation-id",
  "payload": <type-specific payload>
}
```

**Fields**:
- `type` (string, required): Message type identifier
- `id` (string, optional): Correlation ID for request/response matching
- `payload` (json, optional): Type-specific payload

**Action**: Document base message structure

### 4. Document Client → Server Messages

**Message types** (from `internal/ws/message.go`):

1. **ping**
   - Payload: none
   - Purpose: Keepalive
   - Response: pong

2. **edl.submit**
   - Payload: EDL JSON object
   - Purpose: Submit render job
   - Response: edl.ack or job.error

**Action**: Add Client → Server Messages section

### 5. Document Server → Client Messages

**Message types**:

1. **pong**
   - Payload: none
   - Purpose: Keepalive response

2. **edl.ack**
   - Payload: `{"projectId": "string", "jobId": "string"}`
   - Purpose: Acknowledge EDL submission, job created

3. **job.progress**
   - Payload: `{"jobId": "string", "percent": float, "fps": int, "speed": "string", "eta": int, "stage": "string"}`
   - Purpose: Report rendering progress

4. **job.complete**
   - Payload: `{"jobId": "string", "url": "string"}`
   - Purpose: Notify job completion, provide download URL

5. **job.error**
   - Payload: `{"code": "string", "message": "string", "jobId": "string", "retryable": bool}`
   - Purpose: Report job error

6. **media.status**
   - Payload: `{"mediaId": "string", "status": "string", "error": "string"}`
   - Purpose: Notify media processing status changes

**Action**: Add Server → Client Messages section

### 6. Document Error Handling

**Error scenarios**:
- Invalid JSON message → close connection with 1003 (unsupported data)
- Unknown message type → send job.error with code "UNKNOWN_MESSAGE_TYPE"
- EDL validation failure → send job.error with validation details
- Render failure → send job.error with failure reason

**Reconnection**:
- Client should reconnect with exponential backoff (1s, 2s, 4s, 8s max)
- Server buffers messages during disconnection (5min grace period)
- On reconnect, server replays buffered messages

**Action**: Add Error Handling section

### 7. Add Connection Examples

**Example 1: Upload and Render Workflow**

```
Client → Server: (WebSocket connection established)
Server → Client: {"type": "session.created", "payload": {"sessionId": "abc123"}}

Client → Server: {"type": "edl.submit", "payload": {EDL JSON}}
Server → Client: {"type": "edl.ack", "payload": {"projectId": "proj-1", "jobId": "job-123"}}

Server → Client: {"type": "job.progress", "payload": {"jobId": "job-123", "percent": 25, ...}}
Server → Client: {"type": "job.progress", "payload": {"jobId": "job-123", "percent": 50, ...}}
Server → Client: {"type": "job.progress", "payload": {"jobId": "job-123", "percent": 75, ...}}
Server → Client: {"type": "job.complete", "payload": {"jobId": "job-123", "url": "https://..."}}
```

**Example 2: Error Scenario**

```
Client → Server: {"type": "edl.submit", "payload": {invalid EDL}}
Server → Client: {"type": "job.error", "payload": {"message": "2 validation error(s): projectId: projectId is required", "retryable": true}}
```

**Action**: Add Examples section with full workflows

### 8. Document Heartbeat Mechanism

**Current implementation** (from `internal/ws/connection.go`):
- Server sends ping every 30s
- Client must respond with pong within 10s
- Failure to respond closes connection

**Action**: Document heartbeat parameters and behavior

---

## Verification

- [ ] `docs/websocket-protocol.md` exists
- [ ] All message types from `internal/ws/message.go` documented
- [ ] Connection lifecycle documented with state transitions
- [ ] Error handling and reconnection strategy documented
- [ ] Examples provided for success and error scenarios
- [ ] Heartbeat mechanism documented
- [ ] Message payload schemas match actual implementations
- [ ] Custom backend developer can implement protocol from docs alone

---

## Definition of Done

- WebSocket protocol documented in `docs/websocket-protocol.md`
- All message types and payloads specified
- Connection lifecycle and error handling covered
- Examples provided
- Committed to repository

---

## Dependencies

**Blocking**:
- M2 complete (WebSocket server exists)

**Required Files**:
- `internal/ws/message.go` - Message type definitions
- `internal/ws/server.go` - Connection handling
- `internal/ws/session.go` - Session management

---

## Notes

- Keep documentation in sync with code (consider generating from code later)
- Include JSON schema for each payload type
- Provide code examples in multiple languages (JS, Python, Go)
- Document any deviations from standard WebSocket behavior

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § WebSocket Protocol
- [WebSocket RFC 6455](https://tools.ietf.org/html/rfc6455)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
