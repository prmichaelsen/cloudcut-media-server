# Documentation

## Quick Links

- [Architecture Overview](architecture.md) - System design and components
- [Deployment Guide](deployment.md) - How to deploy the service
- [Operations Runbook](runbook.md) - Day-to-day operational tasks
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
- [Monitoring](monitoring.md) - Logs, metrics, and alerting

## Getting Started

### For Developers

1. Read [Architecture](architecture.md) to understand the system
2. Follow [Deployment Guide](deployment.md) to deploy
3. Review [Monitoring](monitoring.md) for observability

### For Operators

1. Bookmark [Operations Runbook](runbook.md) for common tasks
2. Set up alerts from [Monitoring](monitoring.md)
3. Keep [Troubleshooting](troubleshooting.md) handy

## Document Index

### Architecture & Design

- **[Architecture Overview](architecture.md)** - System components, data flows, scaling, security, performance
- **[Requirements](../agent/design/requirements.md)** - Original MVP requirements and scope
- **[Plugin Architecture](../agent/design/plugin-architecture-backend.md)** - Server-side extension system

### Operations

- **[Deployment Guide](deployment.md)** - Initial setup, manual deployment, automated deployment via CI/CD
- **[Operations Runbook](runbook.md)** - Common tasks, incident response, maintenance procedures
- **[Troubleshooting Guide](troubleshooting.md)** - Issue diagnosis and resolution steps
- **[Monitoring & Observability](monitoring.md)** - Structured logging, metrics dashboards, alerting

### Infrastructure

- **[GCP Infrastructure](infrastructure.md)** - GCS bucket, service account, Artifact Registry, IAM roles
- **[Secrets Management](secrets.md)** - Secret Manager configuration and rotation
- **[CI/CD Pipeline](cicd.md)** - GitHub Actions workflows, Workload Identity Federation

### Development

- **[API Documentation](../api/)** - REST API endpoints (future: OpenAPI spec)
- **[WebSocket Protocol](../agent/design/websocket-protocol.md)** - WebSocket message types and flows (future)
- **[Testing Strategy](../agent/testing.md)** - Unit tests, integration tests (future)

### Project Management

- **[Milestones](../agent/milestones/)** - Project phases and timelines
- **[Tasks](../agent/tasks/)** - Detailed implementation tasks
- **[Progress Tracking](../agent/progress.yaml)** - Current project status

## Documentation Standards

### Structure

All docs follow this structure:
1. **Overview** - Brief description
2. **Prerequisites** - What you need before starting
3. **Steps** - Detailed instructions with code examples
4. **Verification** - How to confirm it worked
5. **Troubleshooting** - Common issues
6. **Related Documents** - Links to relevant docs

### Code Examples

All code examples are:
- **Tested** - Verified to work
- **Complete** - Include all necessary context
- **Explained** - Comments where needed

### Maintenance

- **Update frequency**: After each deployment
- **Ownership**: DevOps team
- **Review cycle**: Quarterly

## Contributing

To improve documentation:

1. Identify gaps or errors
2. Create issue or PR
3. Follow documentation standards
4. Update related documents

## Search Tips

### Finding Information

**By topic**:
- Deployment: See [Deployment Guide](deployment.md)
- Operations: See [Operations Runbook](runbook.md)
- Issues: See [Troubleshooting](troubleshooting.md)
- Monitoring: See [Monitoring](monitoring.md)

**By task**:
- "How do I deploy?" → [Deployment Guide](deployment.md)
- "How do I rollback?" → [Operations Runbook](runbook.md) > Rollback section
- "Why is it slow?" → [Troubleshooting](troubleshooting.md) > Performance Issues
- "How do I view logs?" → [Operations Runbook](runbook.md) > View Logs

**By error**:
- Build errors → [Troubleshooting](troubleshooting.md) > Build Issues
- Deployment errors → [Troubleshooting](troubleshooting.md) > Deployment Issues
- Runtime errors → [Troubleshooting](troubleshooting.md) > Runtime Issues

## External Resources

### Google Cloud

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Cloud Storage Documentation](https://cloud.google.com/storage/docs)
- [Cloud Logging](https://cloud.google.com/logging/docs)
- [Secret Manager](https://cloud.google.com/secret-manager/docs)

### Tools

- [GitHub Actions](https://docs.github.com/en/actions)
- [Docker](https://docs.docker.com/)
- [FFmpeg](https://ffmpeg.org/documentation.html)
- [Go](https://go.dev/doc/)

### Agent Context Protocol

- [ACP Repository](https://github.com/prmichaelsen/agent-context-protocol)
- [AGENT.md](../AGENT.md) - ACP methodology for this project

---

**Last Updated**: 2026-03-17
