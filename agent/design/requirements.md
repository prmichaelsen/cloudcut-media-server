# Project Requirements

**Project Name**: cloudcut-media-server
**Created**: 2026-03-17
**Status**: Draft

---

## Overview

A cloud-backed media processing server that serves as the GCP backend for the CloudCut video editor (`cloudcut.media`). It handles intensive video operations — transcoding, rendering, AI-powered features — that exceed browser capabilities, maintaining a persistent connection with the WebAssembly-based frontend for real-time collaboration between client and server.

---

## Problem Statement

Browser-based video editing is constrained by WASM memory limits (4GB per module), lack of direct GPU access, sandboxed file systems, and uneven codec support across browsers. A dedicated server is needed to offload heavy operations while keeping the editing experience responsive and interactive.

---

## Goals and Objectives

### Primary Goals
1. Provide a persistent backend for heavy video processing operations (transcoding, encoding, full-resolution rendering)
2. Enable real-time bidirectional communication with the WASM frontend for status updates and control
3. Manage media assets in GCS with efficient chunk-based streaming to the client

### Secondary Goals
1. Support AI-powered video features (scene detection, auto-captions, object tracking) via Vertex AI
2. Enable proxy editing workflow — serve low-res proxies for client-side editing, full-res rendering server-side
3. Autoscale compute resources to balance cost and responsiveness

---

## Functional Requirements

### Core Features
1. **Media Asset Management**: Upload, store, and serve video assets via GCS with signed URLs for secure chunk-based streaming
2. **Transcoding/Encoding**: Convert between video formats, produce final exports at full resolution
3. **Persistent Connection**: Maintain WebSocket (or gRPC-Web) connection with the frontend for bidirectional status updates and control messages
4. **Proxy Generation**: Generate low-resolution proxy files from uploaded media for client-side editing
5. **Server-Side Rendering**: Composite multi-track timelines at full resolution based on the edit decision list (EDL)

### Additional Features
1. **AI Features**: Scene detection, auto-captioning, and object tracking via Vertex AI
2. **Real-Time Preview Streaming**: Stream rendered frames back to client via WebRTC for low-latency preview
3. **Progress Reporting**: Push transcoding/rendering progress to client via persistent connection

---

## Non-Functional Requirements

### Performance
- Interactive preview operations: < 100ms round-trip latency (requires warm instances, not cold-start)
- Transcoding throughput: support concurrent jobs per user session
- Chunk-based media streaming with minimal buffering

### Security
- Signed URLs for all GCS media access (time-limited, scoped)
- Authentication required for all API endpoints
- No raw media URLs exposed to clients

### Scalability
- Autoscaling compute for burst transcoding workloads
- Idle shutdown policies for persistent GPU instances to manage cost
- Support multiple concurrent editing sessions

### Reliability
- Graceful degradation: client should function for basic edits if connection drops
- Job recovery: resume interrupted transcoding/rendering jobs
- State sync: lightweight EDL/timeline JSON document as source of truth, synced via WebSocket

---

## Technical Requirements

### Technology Stack
- **Language**: Go
- **Video Processing**: FFmpeg (server-side, native — orchestrated via os/exec pipes)
- **Infrastructure**: Google Cloud Platform
- **Persistent Connection**: WebSocket (gorilla/websocket or nhooyr/websocket), with gRPC as alternative (native Go ecosystem)
- **Media Storage**: Google Cloud Storage (GCS)
- **Compute**: GCE or GKE for persistent GPU instances; Cloud Run Jobs or Batch for burst transcoding
- **AI/ML**: Vertex AI for AI-powered features
- **CDN**: Cloud CDN for preview delivery

### Why Go
- First-class concurrency (goroutines) for managing concurrent WebSocket connections and streaming pipes
- Excellent I/O performance for streaming chunks between GCS, FFmpeg, and clients
- Low memory overhead for concurrent video sessions
- Clean FFmpeg process orchestration via os/exec pipe management
- Single binary deployment simplifies Cloud Run / GKE containerization
- gRPC is native to Go (Go is gRPC's primary language)

### GCP Service Mapping

| Need | Service |
|------|---------|
| Persistent compute | GCE or GKE (GPU instances for rendering) |
| Burst transcoding | Cloud Run Jobs or Batch |
| Asset storage | Cloud Storage |
| Low-latency signaling | Cloud Run + WebSockets |
| AI features | Vertex AI |
| CDN for previews | Cloud CDN |

### Integrations
- **cloudcut.media frontend**: Primary client — WASM-based browser video editor
- **Google Cloud Storage**: Media asset storage and signed-URL serving
- **Vertex AI**: AI-powered video analysis features

---

## Architecture Decisions

### 1. Proxy Editing Model
Edit with low-res proxies on the client, final render on the server. This is standard in professional video tools and sidesteps WASM memory limits (4GB per module).

### 2. Chunk-Based Streaming
Never load full videos into client memory. Stream segments from GCS, decode on demand. The server manages chunking and serves via signed URLs.

### 3. EDL as State
The edit decision list (timeline state) is a lightweight JSON document synced via WebSocket. Heavy media stays in GCS — only metadata and edit instructions travel over the wire.

### 4. Warm Instances for Interactive Preview
Interactive preview requires < 100ms round-trip. Use persistent GCE/GKE instances rather than cold-starting Cloud Run for the persistent connection path.

### 5. Client/Server Boundary
- **Client (WASM)**: Timeline UI, scrubbing, preview playback, lightweight frame manipulation (crops, color adjustments, text overlays), audio waveform rendering, real-time preview filters (WASM + WebGL/WebGPU)
- **Server (this project)**: Transcoding/encoding, AI features, multi-track full-res rendering, large asset management, proxy generation

---

## Constraints

### Technical Constraints
- Browser codec support is uneven — WebCodecs API is Chromium-only; WASM FFmpeg is the fallback but slower. Server must be prepared to do more codec work for non-Chromium clients.
- WASM 4GB memory limit means the server must handle all large-buffer operations
- Cross-origin isolation headers required for SharedArrayBuffer + Web Workers on the client (affects deployment config)

### Cost Constraints
- Persistent GPU instances on GCP are expensive — autoscaling and idle shutdown policies are essential
- Media storage costs scale with user base — implement retention policies and tiered storage

---

## Risks

| Risk | Impact | Probability | Mitigation Strategy |
|------|--------|-------------|---------------------|
| GPU instance costs exceed budget | High | Medium | Autoscaling, idle shutdown, burst-only GPU for non-interactive work |
| WebSocket connection instability | Medium | Medium | Reconnection logic, EDL state recovery, offline-capable basic editing |
| Browser codec fragmentation | Medium | High | Server-side fallback transcoding for non-Chromium browsers |
| Latency exceeding interactive threshold | High | Low | Warm instances, regional deployment, connection quality monitoring |
| Large media upload failures | Medium | Medium | Resumable uploads via GCS, chunk-based upload protocol |

---

## Out of Scope

1. **Frontend/WASM client**: Handled by `cloudcut.media` project
2. **User authentication/accounts**: Separate concern, to be integrated later
3. **Collaborative editing**: Single-user sessions for MVP
4. **Mobile support**: Desktop browser focus for MVP
5. **On-premise deployment**: GCP-only

---

## Success Criteria

### MVP Success Criteria
- [ ] Upload video to GCS and generate proxy for client playback
- [ ] Persistent WebSocket connection with heartbeat and reconnection
- [ ] Accept EDL from client, render final export server-side via FFmpeg
- [ ] Stream transcoding progress to client in real-time
- [ ] Chunk-based media serving via signed GCS URLs

### Full Release Success Criteria
- [ ] AI-powered scene detection and auto-captioning
- [ ] Real-time preview streaming via WebRTC
- [ ] Multi-track compositing at full resolution
- [ ] Autoscaling compute with cost controls
- [ ] Sub-100ms interactive preview latency

---

## Related Projects

- **cloudcut.media**: The WASM-based browser frontend that connects to this server
- **gcloud-mcp**: Google Cloud MCP server — useful for deployment and monitoring tooling

---

**Status**: Draft
**Last Updated**: 2026-03-17
**Next Review**: TBD
