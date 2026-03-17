# CI/CD Pipeline

## Overview

Automated testing and deployment using GitHub Actions with Google Cloud Workload Identity Federation for secure, keyless authentication.

## Workflows

### Test (`test.yml`)

**Triggers**:
- Pull requests to `main`
- Pushes to `main`

**Jobs**:
1. **test** - Run Go tests with race detector and coverage
2. **lint** - Run golangci-lint for code quality

**Steps**:
- Checkout code
- Set up Go 1.25
- Install dependencies
- Run tests with race detector (`-race`)
- Generate coverage report
- Upload coverage to Codecov (optional)
- Run golangci-lint

### Deploy (`deploy.yml`)

**Triggers**:
- Pushes to `main` (automatic)
- Manual workflow dispatch

**Steps**:
1. Checkout code
2. Authenticate to Google Cloud via Workload Identity Federation
3. Set up Cloud SDK
4. Configure Docker for Artifact Registry
5. Build Docker image (tagged with commit SHA and `latest`)
6. Push images to Artifact Registry
7. Deploy to Cloud Run
8. Run health check to verify deployment

**Environment Variables**:
- `PROJECT_ID` - GCP project ID (from secrets)
- `REGION` - us-central1
- `SERVICE_NAME` - cloudcut-media-server

## Setup

### Prerequisites

- GCP project with billing enabled
- GitHub repository
- Docker installed locally (for testing)

### Automated Setup

Run the setup script to configure Workload Identity Federation:

```bash
./scripts/setup-github-actions.sh
```

This script will:
1. Create Workload Identity Pool
2. Create Workload Identity Provider (OIDC with GitHub)
3. Create service account for deployments
4. Grant required IAM roles (run.admin, iam.serviceAccountUser, artifactregistry.writer)
5. Configure impersonation for GitHub repository
6. Display secret values to add to GitHub

### Manual Setup

If you prefer manual setup, see [Task 24 documentation](../agent/tasks/milestone-6-production-deployment/task-24-cicd-pipeline.md) for detailed steps.

### Add GitHub Secrets

Go to: `https://github.com/YOUR_USERNAME/YOUR_REPO/settings/secrets/actions`

Add the following secrets (values provided by setup script):

| Secret Name | Description | Required |
|-------------|-------------|----------|
| `GCP_PROJECT_ID` | Your GCP project ID | Yes |
| `WIF_PROVIDER` | Workload Identity Provider resource name | Yes |
| `WIF_SERVICE_ACCOUNT` | Service account email for deployments | Yes |
| `CODECOV_TOKEN` | Codecov upload token | No |

## Deployment Process

### Automatic Deployment

1. Create feature branch:
   ```bash
   git checkout -b feature/my-feature
   ```

2. Make changes and commit:
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

3. Push branch and create PR:
   ```bash
   git push origin feature/my-feature
   gh pr create --title "Add new feature" --body "Description"
   ```

4. **Tests run automatically** on PR creation
   - View status in PR checks
   - Fix any failing tests

5. After review, merge PR:
   ```bash
   gh pr merge --squash
   ```

6. **Deployment triggers automatically** on merge to `main`
   - Docker image built and pushed
   - Cloud Run service updated
   - Health check verifies deployment

### Manual Deployment

Trigger deployment manually via GitHub UI:

1. Go to Actions tab
2. Select "Deploy to Cloud Run" workflow
3. Click "Run workflow"
4. Select branch (usually `main`)
5. Click "Run workflow"

Or via CLI:

```bash
gh workflow run deploy.yml
```

## Monitoring Workflows

### View Workflow Runs

GitHub UI:
- https://github.com/YOUR_USERNAME/YOUR_REPO/actions

CLI:
```bash
# List recent runs
gh run list

# View specific run
gh run view RUN_ID

# Watch run in progress
gh run watch
```

### View Logs

```bash
# View logs for latest run
gh run view --log

# View logs for specific job
gh run view RUN_ID --job=JOB_ID --log
```

## Rollback

### Via GitHub Actions

1. Go to Actions tab
2. Find last successful "Deploy to Cloud Run" run
3. Click "Re-run jobs"
4. Confirm re-run

### Via gcloud

```bash
# List revisions
gcloud run revisions list \
  --service=cloudcut-media-server \
  --region=us-central1

# Rollback to specific revision
gcloud run services update-traffic cloudcut-media-server \
  --region=us-central1 \
  --to-revisions=REVISION_NAME=100
```

### Via Script

Use the deployment script with a specific image:

```bash
# Get previous commit SHA
PREV_SHA=$(git log -2 --format=%H | tail -1)

# Deploy previous version
docker pull us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:$PREV_SHA
docker tag us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:$PREV_SHA \
           us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest
docker push us-central1-docker.pkg.dev/PROJECT_ID/cloudcut/cloudcut-media-server:latest
```

## Branch Protection

### Recommended Rules

For `main` branch:

- ✅ **Require pull request before merging**
  - Require approvals: 1
- ✅ **Require status checks to pass**
  - test (Go tests)
  - lint (golangci-lint)
- ✅ **Require branches to be up to date**
- ✅ **Do not allow bypassing**
- ❌ **Do not allow force pushes**
- ❌ **Do not allow deletions**

Configure at: `https://github.com/YOUR_USERNAME/YOUR_REPO/settings/branches`

## Status Badges

Add to README.md:

```markdown
[![Test](https://github.com/prmichaelsen/cloudcut-media-server/workflows/Test/badge.svg)](https://github.com/prmichaelsen/cloudcut-media-server/actions/workflows/test.yml)
[![Deploy](https://github.com/prmichaelsen/cloudcut-media-server/workflows/Deploy%20to%20Cloud%20Run/badge.svg)](https://github.com/prmichaelsen/cloudcut-media-server/actions/workflows/deploy.yml)
[![codecov](https://codecov.io/gh/prmichaelsen/cloudcut-media-server/branch/main/graph/badge.svg)](https://codecov.io/gh/prmichaelsen/cloudcut-media-server)
```

## Troubleshooting

### Workflow Permission Denied

**Symptom**: "Error: google-github-actions/auth failed with: retry function failed after ... attempts"

**Causes**:
- Workload Identity Federation not configured
- Service account missing permissions
- GitHub secrets not set correctly

**Solutions**:
```bash
# Re-run setup script
./scripts/setup-github-actions.sh

# Verify IAM bindings
gcloud iam service-accounts get-iam-policy github-actions-deployer@PROJECT_ID.iam.gserviceaccount.com

# Check GitHub secrets
gh secret list
```

### Docker Build Fails

**Symptom**: "docker: Error response from daemon: failed to build"

**Causes**:
- Missing dependencies in Dockerfile
- Go module download failure
- Insufficient disk space

**Solutions**:
- Check Dockerfile syntax
- Verify go.mod and go.sum are committed
- Test build locally: `docker build -t test .`

### Tests Fail in CI But Pass Locally

**Symptom**: Tests pass on local machine but fail in GitHub Actions

**Causes**:
- Environment differences (timezone, PATH, etc.)
- Missing dependencies in workflow
- Race conditions (use `-race` flag)

**Solutions**:
- Add debug logging to tests
- Check workflow Go version matches local
- Run tests with `-race` locally
- Use `act` to test workflows locally: https://github.com/nektos/act

### Deployment Succeeds But Health Check Fails

**Symptom**: Deployment completes but curl health check fails

**Causes**:
- Service not fully started yet
- Health endpoint returning non-200 status
- Network issues

**Solutions**:
```bash
# Check Cloud Run logs
gcloud run services logs read cloudcut-media-server --region=us-central1 --limit=50

# Test health endpoint manually
curl -v https://YOUR-SERVICE-URL.run.app/health

# Check service status
gcloud run services describe cloudcut-media-server --region=us-central1
```

## Cost

### GitHub Actions Free Tier

- **Public repositories**: Unlimited minutes
- **Private repositories**: 2,000 minutes/month (sufficient for MVP)

### Billable Minutes

Approximate workflow durations:
- Test workflow: ~2-3 minutes
- Deploy workflow: ~5-7 minutes

**Example monthly usage**:
- 20 PRs/month × 3 minutes = 60 minutes
- 20 merges/month × 7 minutes = 140 minutes
- **Total**: ~200 minutes/month

**Cost**: **$0/month** (within free tier)

## Security

### Workload Identity Federation

**Advantages over service account keys**:
- No long-lived credentials
- Automatic rotation
- Fine-grained access control
- Audit trail via Cloud Logging

**How it works**:
1. GitHub Actions generates OIDC token
2. Token exchanged for short-lived GCP credentials
3. Credentials used for deployment
4. Credentials expire after workflow completes

### Secrets

**Never commit**:
- Service account keys
- API keys
- Passwords
- OAuth tokens

**Use GitHub Secrets for**:
- GCP project ID
- Workload Identity Provider
- Service account email
- Third-party API tokens

## Related Documents

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [Cloud Run CI/CD](https://cloud.google.com/run/docs/continuous-deployment-with-cloud-build)
- [golangci-lint Configuration](https://golangci-lint.run/usage/configuration/)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
