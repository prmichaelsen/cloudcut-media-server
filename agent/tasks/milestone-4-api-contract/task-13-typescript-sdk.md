# Task 13: Build TypeScript SDK

**Status**: Not Started
**Milestone**: M4 - Stable API Contract
**Estimated Hours**: 6-8
**Priority**: Medium

---

## Objective

Create a TypeScript SDK that provides typed HTTP and WebSocket clients for the cloudcut-media-server API, enabling frontend and custom backend developers to interact with the API safely.

---

## Context

The TypeScript SDK serves multiple purposes:
1. **Reference implementation** for custom backends to understand API behavior
2. **Type safety** for TypeScript/JavaScript clients
3. **Validation** that the API contract works end-to-end
4. **Developer experience** with auto-complete and inline documentation

**Design reference**: `agent/design/plugin-architecture-backend.md` § TypeScript SDK

---

## Steps

### 1. Set Up SDK Package

**Action**: Create new package directory
```bash
mkdir -p sdk/typescript
cd sdk/typescript
npm init -y
```

**Package structure**:
```
sdk/typescript/
├── package.json
├── tsconfig.json
├── src/
│   ├── index.ts
│   ├── client.ts
│   ├── websocket.ts
│   ├── types.ts
│   └── errors.ts
├── tests/
│   ├── client.test.ts
│   └── websocket.test.ts
└── README.md
```

### 2. Generate Types from OpenAPI Spec

**Option A**: Use openapi-typescript
```bash
npm install -D openapi-typescript
npx openapi-typescript ../../api/openapi.yaml -o src/types.generated.ts
```

**Option B**: Manual type definitions (fallback if codegen fails)

**Action**: Generate types and verify they match server responses

### 3. Implement HTTP Client

**Features**:
- Typed methods for each endpoint (upload, getMedia, getSignedURL, etc.)
- Error handling with typed error responses
- Request/response interceptors for logging/auth
- Configurable base URL

**Example API**:
```typescript
const client = new CloudCutClient({
  baseURL: 'http://localhost:8080',
  apiKey: 'optional-api-key'
});

// Upload media
const media = await client.media.upload(fileBlob);

// Get media
const mediaInfo = await client.media.get(mediaId);

// Get signed URL
const { url } = await client.media.getSignedURL(mediaId);
```

**Action**: Implement `src/client.ts` with all REST methods

### 4. Implement WebSocket Client

**Features**:
- Type-safe message sending/receiving
- Automatic reconnection with exponential backoff
- Session management and restoration
- Event emitter pattern for message handlers
- Heartbeat handling (respond to ping with pong)

**Example API**:
```typescript
const ws = new CloudCutWebSocket('ws://localhost:8080/ws');

// Connection lifecycle
ws.on('connected', (sessionId) => console.log('Connected:', sessionId));
ws.on('disconnected', () => console.log('Disconnected'));
ws.on('error', (err) => console.error('Error:', err));

// Message handlers
ws.on('edl.ack', (payload) => {
  console.log('Job created:', payload.jobId);
});

ws.on('job.progress', (payload) => {
  console.log('Progress:', payload.percent);
});

ws.on('job.complete', (payload) => {
  console.log('Complete:', payload.url);
});

// Send messages
await ws.submitEDL(edlObject);
```

**Action**: Implement `src/websocket.ts`

### 5. Add Error Handling

**Custom error classes**:
- `CloudCutError` - Base error class
- `APIError` - HTTP errors (400, 404, 500)
- `ValidationError` - EDL validation failures
- `WebSocketError` - WS connection/message errors

**Action**: Implement `src/errors.ts` with typed errors

### 6. Write Tests

**Test coverage**:
- HTTP client unit tests (mock fetch)
- WebSocket client unit tests (mock WebSocket)
- Integration tests against running server

**Action**: Implement tests in `tests/`

### 7. Add Documentation

**README.md sections**:
- Installation (`npm install @cloudcut/media-server-sdk`)
- Quick start example (upload → render workflow)
- API reference (or link to generated typedoc)
- Configuration options
- Error handling
- WebSocket reconnection behavior

**Action**: Write comprehensive README

### 8. Build and Publish

**Build configuration**:
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "declaration": true,
    "outDir": "dist",
    "strict": true
  }
}
```

**Build outputs**:
- ESM (`dist/index.mjs`)
- CJS (`dist/index.cjs`)
- Type definitions (`dist/index.d.ts`)

**Action**:
- Configure tsconfig.json
- Add build scripts to package.json
- Build and verify outputs

### 9. Publish to npm (Optional)

**For MVP**: Keep as reference implementation in repo
**For production**: Publish as `@cloudcut/media-server-sdk`

**Action**: Publish to npm or document as reference SDK

---

## Verification

- [ ] SDK package builds without errors
- [ ] Types generated from OpenAPI spec
- [ ] HTTP client methods implemented for all endpoints
- [ ] WebSocket client with typed message handlers
- [ ] Automatic reconnection works
- [ ] Error classes defined and used
- [ ] Unit tests passing
- [ ] Integration test against server succeeds (upload → render workflow)
- [ ] README with examples
- [ ] Types exported correctly

---

## Definition of Done

- TypeScript SDK implemented in `sdk/typescript/`
- All REST endpoints wrapped with typed methods
- WebSocket client with reconnection
- Tests written and passing
- README documentation complete
- Built and ready for use

---

## Dependencies

**Blocking**:
- Task 10 (OpenAPI spec for type generation)
- Task 11 (WebSocket protocol for client implementation)

**Required**:
- Server running for integration tests

---

## Notes

- Use fetch API (built-in in Node 18+, polyfill for older)
- Use WebSocket API (built-in in browsers, use `ws` package for Node)
- Consider adding request retry logic with exponential backoff
- Add request timeout configuration
- Support both browser and Node.js environments

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § TypeScript SDK
- [openapi-typescript](https://github.com/drwpow/openapi-typescript)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
