# Task 5: EDL Schema & Parsing

**Milestone**: [M2 - Persistent Connection & EDL Processing](../../milestones/milestone-2-persistent-connection.md)
**Design Reference**: [Requirements](../../design/requirements.md)
**Estimated Time**: 2-3 hours
**Dependencies**: [Task 4: WebSocket Server](task-4-websocket-server.md)
**Status**: Not Started

---

## Objective

Define the edit decision list (EDL) JSON schema that describes video timeline edits, and implement parsing and validation so the server can process client editing instructions.

---

## Context

The EDL is the lightweight JSON document synced between client and server. It describes what the client edited — cuts, trims, concatenations, text overlays — without containing actual media data. The server translates the EDL into FFmpeg filter graphs for rendering. The schema must be versioned from day 1 to handle evolution.

---

## Steps

### 1. Define EDL Schema

`internal/edl/schema.go`:

```go
type EDL struct {
    Version   string    `json:"version"`   // "1.0"
    ProjectID string    `json:"projectId"`
    Timeline  Timeline  `json:"timeline"`
    Output    Output    `json:"output"`
}

type Timeline struct {
    Duration float64  `json:"duration"` // total duration in seconds
    Tracks   []Track  `json:"tracks"`
}

type Track struct {
    ID    string  `json:"id"`
    Type  string  `json:"type"` // "video", "audio", "text"
    Clips []Clip  `json:"clips"`
}

type Clip struct {
    ID        string   `json:"id"`
    MediaID   string   `json:"mediaId"`   // references uploaded media
    StartTime float64  `json:"startTime"` // position on timeline (seconds)
    Duration  float64  `json:"duration"`
    InPoint   float64  `json:"inPoint"`   // source start (trim)
    OutPoint  float64  `json:"outPoint"`  // source end (trim)
    Filters   []Filter `json:"filters,omitempty"`
}

type Filter struct {
    Type   string                 `json:"type"` // "text", "crop", "brightness", etc.
    Params map[string]interface{} `json:"params"`
}

type Output struct {
    Format     string `json:"format"`     // "mp4"
    Resolution string `json:"resolution"` // "1920x1080", "source"
    Codec      string `json:"codec"`      // "h264"
    Quality    string `json:"quality"`    // "high", "medium", "low"
}
```

### 2. Implement Validation

`internal/edl/validate.go`:
- Validate version is supported
- Validate all mediaIDs reference existing uploaded media
- Validate time ranges (no negative values, inPoint < outPoint)
- Validate clips don't extend beyond source media duration
- Validate output format is supported
- Return structured validation errors

### 3. Implement Parser

`internal/edl/parser.go`:
- ParseEDL(jsonBytes) — unmarshal and validate
- Return parsed EDL or list of validation errors
- Handle malformed JSON gracefully

### 4. Wire into WebSocket Message Handler

When `edl.submit` message received:
1. Parse and validate EDL
2. If invalid, send `job.error` with validation details
3. If valid, send `edl.ack` and queue for rendering (Task 6)

---

## Verification

- [ ] Valid EDL JSON parses correctly into Go structs
- [ ] Invalid EDL returns clear validation errors
- [ ] MediaID references validated against storage
- [ ] Time range validation catches edge cases
- [ ] Schema version checked and unsupported versions rejected
- [ ] Malformed JSON handled gracefully (no panics)

---

**Next Task**: [Task 6: Server-Side Rendering](task-6-server-side-rendering.md)
