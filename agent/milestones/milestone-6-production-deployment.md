# Milestone 6: Production Deployment

**Status**: Not Started
**Estimated Duration**: 2-3 weeks
**Priority**: High

---

## Goal

Deploy cloudcut-media-server to Google Cloud Platform (Cloud Run) with production-ready infrastructure, monitoring, and automated CI/CD pipeline.

---

## Overview

This milestone takes the locally-developed server and deploys it to production on GCP, making it accessible via public URL with proper security, scalability, and observability. We'll containerize the application, set up GCP infrastructure (GCS bucket, service accounts), deploy to Cloud Run for auto-scaling, configure secrets management, add monitoring/logging, and implement CI/CD for automated deployments.

**Key principle**: Infrastructure as code, automated deployments, zero-downtime updates.

---

## Deliverables

1. **Docker Container**
   - Multi-stage Dockerfile optimized for Go
   - Small image size (<50MB with Alpine)
   - Includes FFmpeg binary
   - Health check endpoint configured

2. **GCP Infrastructure**
   - GCS bucket for media storage (sources, proxies, exports)
   - Service account with least-privilege IAM roles
   - Secrets Manager for sensitive configuration
   - Cloud Build triggers configured

3. **Cloud Run Service**
   - Deployed container with public URL
   - Auto-scaling (0-10 instances)
   - Environment variables configured
   - Custom domain (optional)

4. **CI/CD Pipeline**
   - GitHub Actions workflow (or Cloud Build)
   - Automated tests on PR
   - Automated deployment on main branch merge
   - Rollback capability

5. **Monitoring & Logging**
   - Cloud Logging integration
   - Error reporting
   - Uptime checks
   - Performance metrics

6. **Documentation**
   - Deployment guide
   - Infrastructure diagram
   - Runbook for common operations

---

## Success Criteria

- [ ] Docker image builds successfully and runs locally
- [ ] Cloud Run service accessible via public HTTPS URL
- [ ] Health endpoint returns 200 OK
- [ ] Media upload works (file saved to GCS)
- [ ] WebSocket connections establish successfully
- [ ] Logs visible in Cloud Logging
- [ ] Automated deployment on git push works
- [ ] Zero-downtime deployment verified
- [ ] All environment variables configured via Secrets Manager
- [ ] Service account has minimal required permissions

---

## Context

The server currently runs locally with in-memory storage and no GCS integration. To make it production-ready, we need:

**Infrastructure decisions**:
- **Cloud Run** (not GCE/GKE): Simpler, auto-scales to zero, pay-per-request
- **Container Registry**: Artifact Registry (newer than Container Registry)
- **Secrets**: Secret Manager (not env vars in Cloud Run config)
- **CI/CD**: GitHub Actions (simpler than Cloud Build for GitHub repos)

**Architecture**:
```
GitHub → GitHub Actions → Build Docker → Push to Artifact Registry → Deploy to Cloud Run
                ↓
         Run tests, lint

Cloud Run ← Fetch secrets from Secret Manager
         ↓
    Access GCS bucket for media storage
```

---

## Dependencies

**Upstream**:
- M2 (Server implementation complete)
- GCP account with billing enabled
- GitHub repository (already created)

**Downstream**:
- M3 (Progress streaming) will work in production
- M4 (API docs) will document production endpoints

---

## Tasks

1. **Task 20**: Create Dockerfile and Docker Compose (3-4 hours)
2. **Task 21**: Set Up GCP Infrastructure (4-5 hours)
3. **Task 22**: Deploy to Cloud Run (3-4 hours)
4. **Task 23**: Configure Secrets Management (2-3 hours)
5. **Task 24**: Set Up CI/CD Pipeline (4-5 hours)
6. **Task 25**: Add Monitoring and Logging (3-4 hours)
7. **Task 26**: Write Deployment Documentation (2-3 hours)

**Total estimated**: 21-28 hours

---

## Risks & Mitigations

**Risk 1**: FFmpeg binary too large for Cloud Run container
- **Mitigation**: Use Alpine Linux base, compile FFmpeg with only needed codecs, or use separate FFmpeg service

**Risk 2**: WebSocket connections timeout on Cloud Run
- **Mitigation**: Cloud Run supports WebSockets with 60min timeout, configure keep-alive properly

**Risk 3**: Cold start latency affects user experience
- **Mitigation**: Set min instances to 1 (costs ~$10/month), or accept cold starts for MVP

**Risk 4**: GCS upload bandwidth limits
- **Mitigation**: Use streaming uploads, chunked transfer, monitor quotas

**Risk 5**: Secrets leaked in logs or error messages
- **Mitigation**: Sanitize logs, use Secret Manager, never log credentials

---

## Cost Estimates (Monthly)

**Development/MVP**:
- Cloud Run: $5-10 (with min instances: 1)
- GCS Storage: $1-5 (for 10-50GB media)
- Artifact Registry: $0.10 (for Docker images)
- Secret Manager: $0.06 (6 secrets)
- Logging: $0.50 (50GB logs)
- **Total**: ~$7-16/month

**Production (with traffic)**:
- Cloud Run: $20-50 (with auto-scaling)
- GCS Storage: $20-100 (for 100-500GB media)
- Artifact Registry: $0.50
- Secret Manager: $0.06
- Logging: $5-10
- **Total**: ~$45-160/month

---

## Out of Scope (Deferred to Future)

- **Load balancer**: Cloud Run provides built-in LB
- **CDN**: Defer to M7 (Performance Optimization)
- **Multi-region deployment**: Single region for MVP
- **Database**: Using in-memory storage for MVP
- **Custom domain**: Can be added later
- **SSL certificates**: Cloud Run provides managed certs

---

## Related Documents

- [`agent/design/requirements.md`](../design/requirements.md) - GCP architecture decisions
- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Deploying Go to Cloud Run](https://cloud.google.com/run/docs/quickstarts/build-and-deploy/deploy-go-service)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
