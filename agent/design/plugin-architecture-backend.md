# Backend Plugin Architecture

**Concept**: Server-side extension system supporting client plugin architecture
**Created**: 2026-03-17
**Status**: Design Specification

---

## Overview

This document defines how cloudcut-media-server supports the client plugin architecture described in `cloudcut.media/agent/design/local.plugin-architecture.md`. We consider two deployment models:

1. **Managed Backend (Backend Runs Plugins)**: cloudcut.media's hosted backend with server-side plugin support
2. **Custom Backend (User Brings Their Own)**: Users run their own backend implementations

---

## Problem Statement

Client plugins need backend support for:
- **Custom video effects**: Server must render plugin-defined effects in EDL
- **AI features**: Server must execute AI analysis/generation plugins
- **Export formats**: Server must support plugin-defined codecs/containers
- **Storage integrations**: Server must connect to plugin-specified storage backends
- **Performance**: Complex effects require server-side GPU/CPU processing

**Two deployment scenarios**:
1. **Managed**: Users expect plugin support on cloudcut.media's hosted backend
2. **Self-hosted**: Users want to run custom backends with proprietary plugins

---

## Solution Overview

### Architecture Principles

1. **EDL as Universal Protocol**: EDL JSON is the contract between client and server
2. **Plugin-Agnostic Core**: Server core validates EDL structure, plugins handle semantics
3. **Dual Deployment Support**: Same codebase runs managed or custom backends
4. **Stable API Contract**: Custom backends implement well-defined HTTP/WebSocket API

---

## Case 1: Managed Backend (Backend Runs Plugins)

### Extension Points

Server-side extension points mirror client extension points where backend processing is required:

| Extension Point | Purpose | Implementation |
|-----------------|---------|----------------|
| `effects.video` | Server-side video effects | FFmpeg filters, GLSL shaders, custom Go code |
| `effects.audio` | Audio processing | FFmpeg audio filters, custom DSP |
| `export.formats` | Custom export codecs | FFmpeg output format handlers |
| `ai.analysis` | Video analysis services | Scene detection, transcription, object tracking |
| `ai.generation` | Content generation | Background removal, voice clone, upscaling |
| `rendering.pipeline` | Custom render pipelines | Alternative to FFmpeg (e.g., MLT, GStreamer) |
| `storage.backends` | Storage providers | S3, Azure Blob, Wasabi, on-prem NAS |

### Plugin Types

#### 1. Go Plugins (Native Performance)

Compiled Go plugins using `plugin` package:

```go
// plugin/effects/film-grain/main.go
package main

import "github.com/prmichaelsen/cloudcut-media-server/pkg/plugin"

type FilmGrainEffect struct{}

func (e *FilmGrainEffect) ID() string {
    return "film-grain"
}

func (e *FilmGrainEffect) BuildFFmpegFilter(params map[string]interface{}) string {
    intensity := params["intensity"].(float64)
    return fmt.Sprintf("noise=alls=%d:allf=t", int(intensity*100))
}

// Export symbol for plugin loader
var Effect plugin.VideoEffect = &FilmGrainEffect{}
```

**Pros**: Native performance, full Go ecosystem access
**Cons**: Must be compiled for target platform, not sandboxed

#### 2. WASM Plugins (Portable & Sandboxed)

WebAssembly modules for portable, sandboxed plugins:

```go
// Plugin loader uses wazero runtime
import "github.com/tetratelabs/wazero"

func (r *Renderer) loadWASMPlugin(path string) (*WASMPlugin, error) {
    runtime := wazero.NewRuntime(ctx)
    module, err := runtime.InstantiateWithConfig(wasmBytes, moduleConfig)
    // Call WASM exports for filter building
}
```

**Pros**: Portable, sandboxed, any language (Rust, C++, AssemblyScript)
**Cons**: Performance overhead, limited host API access

#### 3. Container Plugins (Microservices)

Docker containers exposing HTTP/gRPC APIs:

```yaml
# plugin.yaml
name: ai-background-removal
type: container
image: plugins/bg-removal:1.0.0
api:
  type: http
  port: 8080
  endpoints:
    - path: /process
      method: POST
```

Server invokes via HTTP:
```go
func (r *Renderer) applyContainerPlugin(pluginID string, input []byte) ([]byte, error) {
    plugin := r.pluginRegistry.GetContainer(pluginID)
    resp, err := http.Post(plugin.URL+"/process", "video/mp4", bytes.NewReader(input))
    return io.ReadAll(resp.Body)
}
```

**Pros**: Language-agnostic, fully isolated, scalable (K8s)
**Cons**: Network overhead, deployment complexity

#### 4. Script Plugins (Rapid Development)

Embedded scripting via Lua/JavaScript:

```lua
-- plugins/custom-fade/effect.lua
function build_filter(params)
    local duration = params.duration or 1.0
    return string.format("fade=in:st=0:d=%.2f", duration)
end
```

**Pros**: Easy to write, no compilation, hot reload
**Cons**: Limited performance, restricted API access

### Plugin Manifest (Server)

Server plugins declare capabilities in `plugin.yaml`:

```yaml
name: vintage-film-effects
version: 1.0.0
type: go-plugin  # or wasm, container, script
runtime:
  path: ./plugin.so
  entrypoint: Effect

contributes:
  effects.video:
    - id: film-grain
      name: Film Grain
      parameters:
        - id: intensity
          type: float
          default: 0.5
          min: 0.0
          max: 1.0
      implementation:
        type: ffmpeg-filter
        template: "noise=alls={{intensity|mul:100}}:allf=t"

  ai.analysis:
    - id: scene-detection
      name: Scene Detection
      implementation:
        type: http-service
        url: http://localhost:8081/detect-scenes
        timeout: 30s
```

### EDL Plugin References

Client EDL references plugins by ID, server resolves implementations:

```json
{
  "version": "1.0",
  "projectId": "proj-123",
  "timeline": {
    "tracks": [{
      "clips": [{
        "id": "clip-1",
        "mediaId": "media-1",
        "filters": [
          {
            "type": "film-grain",  // Plugin ID
            "params": {
              "intensity": 0.7
            }
          }
        ]
      }]
    }]
  }
}
```

### Plugin Registry

Server maintains plugin registry:

```go
// internal/plugin/registry.go
type PluginRegistry struct {
    effects   map[string]VideoEffectPlugin
    aiTools   map[string]AIPlugin
    exporters map[string]ExportPlugin
}

func (r *PluginRegistry) LoadPlugins(pluginDir string) error {
    manifests, err := r.discover(pluginDir)
    for _, manifest := range manifests {
        if err := r.validate(manifest); err != nil {
            return err
        }
        plugin, err := r.loadPlugin(manifest)
        r.register(plugin)
    }
}

func (r *PluginRegistry) GetEffect(id string) (VideoEffectPlugin, error) {
    effect, ok := r.effects[id]
    if !ok {
        return nil, fmt.Errorf("effect plugin not found: %s", id)
    }
    return effect, nil
}
```

### Rendering Pipeline with Plugins

Modified rendering flow:

```go
// internal/render/ffmpeg.go
func (f *FFmpegRenderer) buildVideoFilter(tracks []edl.Track, parsedEDL *edl.EDL) (string, error) {
    for _, clip := range track.Clips {
        for _, filter := range clip.Filters {
            // Check if filter is a plugin
            if plugin, err := f.pluginRegistry.GetEffect(filter.Type); err == nil {
                // Plugin builds FFmpeg filter
                filterStr := plugin.BuildFFmpegFilter(filter.Params)
                filterChain += "," + filterStr
            } else {
                // Built-in filter
                filterStr := buildFilterString(filter)
                filterChain += "," + filterStr
            }
        }
    }
}
```

### AI Plugin Invocation

Background removal example:

```go
// internal/plugin/ai.go
func (p *AIBackgroundRemovalPlugin) Process(ctx context.Context, input VideoClip) (VideoClip, error) {
    // 1. Download clip from GCS
    data, err := p.gcs.Download(ctx, input.Path)

    // 2. Invoke container plugin
    resp, err := http.Post(p.serviceURL+"/remove-bg", "video/mp4", data)

    // 3. Upload result to GCS
    outputPath := "temp/" + uuid.New().String() + ".mp4"
    p.gcs.Upload(ctx, outputPath, resp.Body)

    return VideoClip{Path: outputPath}, nil
}
```

### Security Model

**Sandboxing Strategy**:

1. **Go plugins**: Run in same process (trusted plugins only)
2. **WASM plugins**: Sandboxed by WASM runtime, limited syscall access
3. **Container plugins**: Isolated by Docker/K8s, network policies
4. **Script plugins**: Restricted API surface, no filesystem access

**Plugin Verification**:
- Code signing for official plugins
- Marketplace review process
- User warnings for third-party plugins
- Resource limits (CPU, memory, execution time)

---

## Case 2: Custom Backend (User Brings Their Own)

### API Contract

Define stable HTTP/WebSocket API that custom backends must implement:

#### REST Endpoints

```
POST   /api/v1/media/upload           # Upload media files
GET    /api/v1/media/{id}             # Get media metadata
GET    /api/v1/media/{id}/url         # Get signed URL for source
GET    /api/v1/media/{id}/proxy/url   # Get signed URL for proxy
GET    /api/v1/jobs/{id}              # Get job status
GET    /api/v1/jobs/{id}/output       # Download rendered output
GET    /health                        # Health check
```

#### WebSocket Protocol

```
Client → Server:
  {type: "edl.submit", payload: <EDL>}
  {type: "ping"}

Server → Client:
  {type: "edl.ack", payload: {projectId, jobId}}
  {type: "job.progress", payload: {jobId, percent, fps, speed, eta, stage}}
  {type: "job.complete", payload: {jobId, url}}
  {type: "job.error", payload: {jobId, code, message, retryable}}
  {type: "media.status", payload: {mediaId, status, error}}
  {type: "pong"}
```

### OpenAPI Specification

Publish OpenAPI 3.0 spec for custom backend implementations:

```yaml
# api/openapi.yaml
openapi: 3.0.0
info:
  title: CloudCut Media Server API
  version: 1.0.0
  description: Standard API contract for custom backends

paths:
  /api/v1/media/upload:
    post:
      summary: Upload media file
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file:
                  type: string
                  format: binary
      responses:
        201:
          description: Upload successful
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Media'
```

### EDL Schema Versioning

EDL schema is versioned for backward compatibility:

```json
{
  "version": "1.0",  // EDL schema version
  "projectId": "proj-123",
  "extensions": {     // Optional custom backend extensions
    "custom.ai-upscale": {
      "model": "realesrgan-x4",
      "targetResolution": "3840x2160"
    }
  }
}
```

Custom backends can:
1. **Support standard EDL v1.0**: Must handle core schema
2. **Extend with custom fields**: Use `extensions` namespace
3. **Ignore unknown extensions**: Graceful degradation

### Client Configuration

Client configured to use custom backend:

```typescript
// cloudcut.media config
{
  "backend": {
    "type": "custom",
    "url": "https://my-backend.example.com",
    "wsUrl": "wss://my-backend.example.com/ws",
    "apiKey": "user-api-key"
  }
}
```

### Custom Backend Implementation Example

Reference implementation in different languages:

**Python (FastAPI)**:
```python
# custom-backend/main.py
from fastapi import FastAPI, WebSocket
import edl_validator

app = FastAPI()

@app.post("/api/v1/media/upload")
async def upload_media(file: UploadFile):
    # Store media, generate proxy
    return {"id": media_id, "status": "processing"}

@app.websocket("/ws")
async def websocket_endpoint(websocket: WebSocket):
    await websocket.accept()
    async for message in websocket.iter_json():
        if message["type"] == "edl.submit":
            # Validate EDL
            edl = edl_validator.parse(message["payload"])
            # Submit render job
            job = await render_service.submit(edl)
            await websocket.send_json({
                "type": "edl.ack",
                "payload": {"jobId": job.id}
            })
```

**Node.js (Express)**:
```javascript
// custom-backend/server.js
const express = require('express');
const ws = require('ws');

app.post('/api/v1/media/upload', upload.single('file'), (req, res) => {
  // Process upload
});

wss.on('connection', (socket) => {
  socket.on('message', async (data) => {
    const msg = JSON.parse(data);
    if (msg.type === 'edl.submit') {
      const edl = validateEDL(msg.payload);
      const job = await renderService.submit(edl);
      socket.send(JSON.stringify({
        type: 'edl.ack',
        payload: {jobId: job.id}
      }));
    }
  });
});
```

### Benefits of Custom Backend Support

1. **On-Premises Deployment**: Enterprise customers keep data in-house
2. **Custom Integrations**: Connect to proprietary DAM systems, approval workflows
3. **Specialized Hardware**: Leverage on-prem GPU farms, custom render nodes
4. **Cost Control**: Users pay for their own compute/storage
5. **Compliance**: Meet data residency, security, audit requirements

---

## Implementation Strategy

### Phase 1: Stable API Contract (M3)

**Goal**: Define and document the HTTP/WebSocket API

**Actions**:
1. Create OpenAPI specification
2. Document WebSocket protocol
3. Publish API reference documentation
4. Provide reference client (TypeScript SDK)

**Validation**:
- API contract doesn't change between releases (semver)
- Reference client can connect to both managed and custom backends

### Phase 2: Plugin Registry (Post-MVP)

**Goal**: Support server-side plugins on managed backend

**Actions**:
1. Implement `internal/plugin/registry.go`
2. Add plugin discovery and loading
3. Extend EDL validator to check plugin references
4. Update FFmpeg builder to invoke plugins

**Recommended Plugin Type for MVP**: **Go plugins** (native performance, simpler than containers)

**Validation**:
- Load sample plugin from `plugins/` directory
- Render EDL with plugin-defined effect
- Plugin error doesn't crash server

### Phase 3: Plugin Marketplace (P10)

**Goal**: Distribute and monetize plugins

**Actions**:
1. Web UI for plugin discovery
2. Plugin submission and review process
3. Automated security scanning
4. Payment integration for premium plugins

---

## EDL Extensions

Current EDL schema already supports plugin references via `filters[].type`:

```json
{
  "filters": [
    {
      "type": "plugin-id",      // Could be built-in or plugin
      "params": {
        "param1": "value1"
      }
    }
  ]
}
```

**Server behavior**:
1. Check if `type` is built-in filter → use `buildFilterString()`
2. Check if `type` is registered plugin → invoke `plugin.BuildFFmpegFilter()`
3. Neither found → validation error

**No EDL schema changes required** — plugins integrate seamlessly.

---

## Testing Strategy

### Plugin System Tests

```go
// internal/plugin/registry_test.go
func TestPluginRegistry_LoadAndResolve(t *testing.T) {
    registry := NewPluginRegistry()
    registry.LoadPlugins("testdata/plugins/")

    plugin, err := registry.GetEffect("test-effect")
    assert.NoError(t, err)
    assert.Equal(t, "test-effect", plugin.ID())
}

func TestPluginRegistry_InvalidPlugin(t *testing.T) {
    registry := NewPluginRegistry()
    err := registry.LoadPlugins("testdata/invalid-plugin/")
    assert.Error(t, err)
}
```

### Custom Backend Compatibility Tests

```go
// test/e2e/custom_backend_test.go
func TestCustomBackendCompatibility(t *testing.T) {
    // Start reference custom backend
    backend := startTestBackend()
    defer backend.Stop()

    // Client submits EDL
    client := NewClient(backend.URL)
    job, err := client.SubmitEDL(testEDL)
    assert.NoError(t, err)

    // Wait for completion
    result := waitForCompletion(job.ID)
    assert.Equal(t, "complete", result.Status)
}
```

---

## Trade-offs

### Managed Backend Plugins

**Pros**:
- Users access powerful features without custom backend
- Plugin marketplace creates ecosystem
- Revenue opportunity (premium plugins)

**Cons**:
- Security risk (malicious plugins)
- Performance overhead (plugin loading, execution)
- Maintenance burden (breaking changes affect plugins)
- Testing complexity (core + all plugin combinations)

**Mitigation**:
- Start with Go plugins (trusted, performant)
- Add sandboxing later (WASM, containers)
- Versioned plugin API with deprecation warnings

### Custom Backend Support

**Pros**:
- Enterprise customers get on-prem deployment
- Users customize to exact needs
- Reduces managed backend load

**Cons**:
- Support burden (users running buggy custom backends)
- Feature fragmentation (custom backends lag behind)
- Lost revenue (users don't use managed backend)

**Mitigation**:
- Clear API contract with semver
- Reference implementation + SDK
- Premium support tier for custom backends

---

## Dependencies

- **Phase 1** (API contract): OpenAPI codegen, API documentation generator
- **Phase 2** (plugin system): Go `plugin` package or wazero (WASM), Docker SDK (containers)
- **Phase 3** (marketplace): Web service, payment gateway, code scanning tools

---

## Future Considerations

1. **Plugin Sandboxing**: Isolate plugins (WASM, containers) for security
2. **Hot Reload**: Update plugins without server restart
3. **Plugin Dependencies**: Plugins depend on other plugins
4. **Distributed Plugins**: Plugin runs on separate service/GPU cluster
5. **Plugin Metrics**: Track usage, performance, error rates
6. **A/B Testing**: Test plugin versions before rollout
7. **Plugin SDK**: Go package with types, testing utilities, examples
8. **Code Signing**: Verify plugin authenticity
9. **Native Bindings**: Plugins call CUDA, Metal for GPU acceleration

---

## Decision Matrix

| Requirement | Managed Backend | Custom Backend | Notes |
|-------------|-----------------|----------------|-------|
| Client plugin effects work | ✅ Via server plugins | ✅ Custom impl | Server must render client effects |
| AI features (bg removal, etc.) | ✅ Via AI plugins | ✅ Custom impl | Container plugins ideal for AI |
| Custom export formats | ✅ Via export plugins | ✅ Custom impl | FFmpeg wrapper plugins |
| On-prem deployment | ❌ | ✅ | Enterprise requirement |
| Data privacy | ⚠️ (managed infra) | ✅ | HIPAA, GDPR use cases |
| Cost control | ❌ (metered) | ✅ | Users control infra costs |
| Maintenance | ✅ (we handle) | ⚠️ (user handles) | |
| Plugin marketplace | ✅ | ⚠️ (manual install) | |

---

## Recommendation

**Phase 1 (M3)**: Publish stable API contract for custom backends
- Focus on core HTTP/WebSocket API
- Create OpenAPI spec + reference TypeScript client
- Enable custom backend development in parallel

**Phase 2 (Post-MVP)**: Add server plugin system
- Start with Go plugins for effects/exports
- Container plugins for AI features (GPU isolation)
- Defer marketplace to P10

**Phase 3 (P10+)**: Plugin marketplace
- Discovery, ratings, automated updates
- Premium plugin revenue model

---

**Status**: Design Specification
**Next Steps**:
1. Review API contract with frontend team
2. Create OpenAPI specification
3. Implement extension points in rendering pipeline
