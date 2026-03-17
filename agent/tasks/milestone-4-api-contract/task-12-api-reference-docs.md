# Task 12: Generate API Reference Docs

**Status**: Not Started
**Milestone**: M4 - Stable API Contract
**Estimated Hours**: 2-3
**Priority**: Medium

---

## Objective

Generate HTML API reference documentation from the OpenAPI specification and publish it for custom backend developers.

---

## Context

With the OpenAPI spec complete (Task 10), we can auto-generate interactive API documentation that includes:
- Endpoint explorer with try-it-out functionality
- Request/response examples
- Schema definitions
- Code snippets in multiple languages

**Design reference**: `agent/design/plugin-architecture-backend.md` § API Reference Documentation

---

## Steps

### 1. Choose Documentation Generator

**Options**:
- **Swagger UI** - Standard OpenAPI viewer, interactive
- **Redoc** - Clean, modern, responsive design
- **Stoplight Elements** - Embeddable, customizable
- **Docusaurus + OpenAPI plugin** - Full documentation site

**Recommendation**: Start with Redoc (simple, looks professional) or Swagger UI (most standard).

**Action**: Choose generator and install tooling

### 2. Set Up Documentation Build

**For Redoc**:
```bash
npm install -g redoc-cli
redoc-cli bundle api/openapi.yaml -o docs/api-reference.html
```

**For Swagger UI**:
```bash
npm install -g swagger-ui-dist
# Copy swagger-ui files, point to openapi.yaml
```

**Action**: Create build script in `scripts/build-docs.sh`

### 3. Add Code Examples

Enhance OpenAPI spec with code examples for common operations:

**Example: Upload Media (curl)**
```bash
curl -X POST http://localhost:8080/api/v1/media/upload \
  -F "file=@video.mp4"
```

**Example: Upload Media (TypeScript)**
```typescript
const formData = new FormData();
formData.append('file', fileBlob);
const response = await fetch('http://localhost:8080/api/v1/media/upload', {
  method: 'POST',
  body: formData
});
```

**Action**: Add code examples to openapi.yaml for key endpoints

### 4. Generate Documentation

**Action**:
- Run documentation generator
- Output to `docs/api-reference.html` (or `docs/api/` directory)
- Verify it renders correctly

### 5. Add Navigation and Context

If using static HTML (Redoc/Swagger UI):
- Add introduction section explaining API purpose
- Link to WebSocket protocol docs
- Link to authentication guide (when available)
- Link to example custom backend implementations

**Action**: Create `docs/index.html` as landing page, link to api-reference.html

### 6. Set Up Hosting

**Options**:
- **GitHub Pages** - Host from `docs/` directory
- **Netlify/Vercel** - Continuous deployment
- **GCS Bucket** - Static site hosting
- **Docs as code** - Check into repo

**Recommendation**: Check into repo under `docs/`, enable GitHub Pages later.

**Action**: Commit generated docs to `docs/` directory

### 7. Add Build to CI

**Action**:
- Add docs build step to CI pipeline (if exists)
- Regenerate docs on OpenAPI spec changes
- Validate docs build successfully

---

## Verification

- [ ] API reference HTML generated from OpenAPI spec
- [ ] Documentation includes all endpoints from openapi.yaml
- [ ] Code examples provided for key endpoints (upload, retrieve, render)
- [ ] Documentation is viewable in browser
- [ ] All schemas and response types rendered correctly
- [ ] Navigation works (if multi-page)
- [ ] Links to WebSocket protocol docs
- [ ] Build script created and documented

---

## Definition of Done

- API reference documentation generated and viewable
- Committed to `docs/` directory
- Build script created
- README updated with link to docs

---

## Dependencies

**Blocking**:
- Task 10 (OpenAPI spec must be complete)

**Optional**:
- Task 11 (WebSocket docs to link to)

---

## Notes

- Keep generated docs in git (or add build step to CI)
- Update docs whenever OpenAPI spec changes
- Consider versioning docs (v1, v2) if API evolves
- Add search functionality if docs grow large

---

## Related Documents

- [`agent/design/plugin-architecture-backend.md`](../../design/plugin-architecture-backend.md) § API Reference Documentation
- [Redoc Documentation](https://redocly.com/redoc)
- [Swagger UI Documentation](https://swagger.io/tools/swagger-ui/)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
