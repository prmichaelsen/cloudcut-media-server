# Task 24: Set Up CI/CD Pipeline

**Status**: Not Started
**Milestone**: M6 - Production Deployment
**Estimated Hours**: 4-5
**Priority**: Medium

---

## Objective

Create automated CI/CD pipeline using GitHub Actions that runs tests on PRs and automatically deploys to Cloud Run on main branch merges.

---

## Context

Manual deployments are error-prone and slow. CI/CD automates:
- **Testing**: Run tests on every PR
- **Linting**: Enforce code quality
- **Building**: Build Docker images automatically
- **Deploying**: Deploy to Cloud Run on merge to main
- **Rollback**: Easy rollback to previous versions

---

## Steps

### 1. Create GitHub Actions Workflow Directory

**Action**: Create workflow directory structure

```bash
mkdir -p .github/workflows
```

### 2. Create Test Workflow (PR Checks)

**Action**: Create `.github/workflows/test.yml`

```yaml
name: Test

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache: true

      - name: Install dependencies
        run: go mod download

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: ./coverage.out
          token: ${{ secrets.CODECOV_TOKEN }}
        if: always()

  lint:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
```

### 3. Create Deploy Workflow (Main Branch)

**Action**: Create `.github/workflows/deploy.yml`

```yaml
name: Deploy to Cloud Run

on:
  push:
    branches: [main]
  workflow_dispatch:  # Allow manual trigger

env:
  PROJECT_ID: ${{ secrets.GCP_PROJECT_ID }}
  REGION: us-central1
  SERVICE_NAME: cloudcut-media-server

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      id-token: write  # For workload identity federation

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ secrets.WIF_PROVIDER }}
          service_account: ${{ secrets.WIF_SERVICE_ACCOUNT }}

      - name: Set up Cloud SDK
        uses: google-github-actions/setup-gcloud@v2

      - name: Configure Docker for Artifact Registry
        run: gcloud auth configure-docker ${{ env.REGION }}-docker.pkg.dev

      - name: Build Docker image
        run: |
          docker build -t ${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/cloudcut/${{ env.SERVICE_NAME }}:${{ github.sha }} .
          docker tag ${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/cloudcut/${{ env.SERVICE_NAME }}:${{ github.sha }} \
                     ${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/cloudcut/${{ env.SERVICE_NAME }}:latest

      - name: Push Docker image
        run: |
          docker push ${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/cloudcut/${{ env.SERVICE_NAME }}:${{ github.sha }}
          docker push ${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/cloudcut/${{ env.SERVICE_NAME }}:latest

      - name: Deploy to Cloud Run
        run: |
          gcloud run deploy ${{ env.SERVICE_NAME }} \
            --image=${{ env.REGION }}-docker.pkg.dev/${{ env.PROJECT_ID }}/cloudcut/${{ env.SERVICE_NAME }}:${{ github.sha }} \
            --region=${{ env.REGION }} \
            --platform=managed \
            --quiet

      - name: Get Service URL
        run: |
          SERVICE_URL=$(gcloud run services describe ${{ env.SERVICE_NAME }} --region=${{ env.REGION }} --format='value(status.url)')
          echo "Service deployed to: ${SERVICE_URL}"
          echo "SERVICE_URL=${SERVICE_URL}" >> $GITHUB_ENV

      - name: Test deployment
        run: |
          curl -f ${{ env.SERVICE_URL }}/health || exit 1
          echo "✅ Health check passed"
```

### 4. Set Up Workload Identity Federation

**Action**: Configure Workload Identity Federation for GitHub Actions

```bash
export PROJECT_ID=$(gcloud config get-value project)
export PROJECT_NUMBER=$(gcloud projects describe ${PROJECT_ID} --format='value(projectNumber)')
export POOL_NAME="github-actions-pool"
export PROVIDER_NAME="github-actions-provider"
export SA_NAME="github-actions-deployer"
export REPO="prmichaelsen/cloudcut-media-server"

# Create Workload Identity Pool
gcloud iam workload-identity-pools create ${POOL_NAME} \
  --location="global" \
  --display-name="GitHub Actions Pool"

# Create Workload Identity Provider
gcloud iam workload-identity-pools providers create-oidc ${PROVIDER_NAME} \
  --location="global" \
  --workload-identity-pool=${POOL_NAME} \
  --display-name="GitHub Actions Provider" \
  --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository" \
  --issuer-uri="https://token.actions.githubusercontent.com"

# Create service account for GitHub Actions
gcloud iam service-accounts create ${SA_NAME} \
  --display-name="GitHub Actions Deployer"

# Grant required roles
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountUser"

gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.writer"

# Allow GitHub Actions to impersonate service account
gcloud iam service-accounts add-iam-policy-binding ${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_NAME}/attribute.repository/${REPO}"

# Get Workload Identity Provider resource name
export WIF_PROVIDER="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_NAME}/providers/${PROVIDER_NAME}"

echo ""
echo "Add these secrets to GitHub repository settings:"
echo "GCP_PROJECT_ID=${PROJECT_ID}"
echo "WIF_PROVIDER=${WIF_PROVIDER}"
echo "WIF_SERVICE_ACCOUNT=${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
```

### 5. Add GitHub Secrets

**Action**: Add secrets to GitHub repository

Go to: https://github.com/prmichaelsen/cloudcut-media-server/settings/secrets/actions

Add secrets:
- `GCP_PROJECT_ID`: Your GCP project ID
- `WIF_PROVIDER`: Workload Identity Provider (from step 4)
- `WIF_SERVICE_ACCOUNT`: Service account email (from step 4)
- `CODECOV_TOKEN`: (Optional) For code coverage reporting

### 6. Create Branch Protection Rules

**Action**: Protect main branch

Go to: https://github.com/prmichaelsen/cloudcut-media-server/settings/branches

Add rule for `main`:
- ✅ Require pull request before merging
- ✅ Require status checks to pass (test, lint)
- ✅ Require branches to be up to date
- ✅ Do not allow force pushes

### 7. Test CI/CD Pipeline

**Action**: Create test PR to verify workflows

```bash
# Create feature branch
git checkout -b test-cicd

# Make small change
echo "# CI/CD Test" >> README.md

# Commit and push
git add README.md
git commit -m "test: verify CI/CD pipeline"
git push origin test-cicd

# Create PR via GitHub UI or gh CLI
gh pr create --title "Test CI/CD" --body "Testing automated workflows"
```

**Verify**:
- Test workflow runs on PR
- All checks pass (tests, lint)
- Merge PR
- Deploy workflow triggers
- Deployment succeeds
- Health check passes

### 8. Add Status Badges to README

**Action**: Update README.md with badges

```markdown
# cloudcut-media-server

[![Test](https://github.com/prmichaelsen/cloudcut-media-server/workflows/Test/badge.svg)](https://github.com/prmichaelsen/cloudcut-media-server/actions/workflows/test.yml)
[![Deploy](https://github.com/prmichaelsen/cloudcut-media-server/workflows/Deploy%20to%20Cloud%20Run/badge.svg)](https://github.com/prmichaelsen/cloudcut-media-server/actions/workflows/deploy.yml)
[![codecov](https://codecov.io/gh/prmichaelsen/cloudcut-media-server/branch/main/graph/badge.svg)](https://codecov.io/gh/prmichaelsen/cloudcut-media-server)
```

### 9. Document CI/CD Process

**Action**: Create `docs/cicd.md`

```markdown
# CI/CD Pipeline

## Workflows

### Test (`test.yml`)
Runs on every PR and push to main:
- Run Go tests with race detector
- Run golangci-lint
- Upload coverage to Codecov

### Deploy (`deploy.yml`)
Runs on push to main:
- Build Docker image
- Push to Artifact Registry (tagged with commit SHA and latest)
- Deploy to Cloud Run
- Run health check

## GitHub Actions Secrets

Required secrets (set in repo settings):
- `GCP_PROJECT_ID`: GCP project ID
- `WIF_PROVIDER`: Workload Identity Provider
- `WIF_SERVICE_ACCOUNT`: Service account email
- `CODECOV_TOKEN`: (Optional) Codecov token

## Deployment Process

1. Create feature branch
2. Make changes and commit
3. Push branch and create PR
4. Tests run automatically
5. After review, merge PR
6. Automatic deployment to Cloud Run
7. Verify via health check

## Rollback

### Via GitHub
1. Go to Actions tab
2. Find last successful deploy
3. Click "Re-run jobs"

### Via gcloud
```bash
# List revisions
gcloud run revisions list --service=cloudcut-media-server --region=us-central1

# Rollback to specific revision
gcloud run services update-traffic cloudcut-media-server \
  --region=us-central1 \
  --to-revisions=REVISION_NAME=100
```

## Manual Deployment

Trigger manually via Actions tab or:
```bash
gh workflow run deploy.yml
```
```

---

## Verification

- [ ] GitHub Actions workflows created (test.yml, deploy.yml)
- [ ] Workload Identity Federation configured
- [ ] GitHub secrets added to repository
- [ ] Branch protection rules enabled
- [ ] Test workflow runs on PR creation
- [ ] Deploy workflow runs on main branch push
- [ ] Automated deployment succeeds
- [ ] Health check passes after deployment
- [ ] Status badges added to README
- [ ] CI/CD documentation created

---

## Definition of Done

- CI/CD pipeline configured and working
- Tests run automatically on PRs
- Deployments automated on main branch merges
- Branch protection enabled
- Documentation complete
- First successful automated deployment verified

---

## Dependencies

**Blocking**:
- Task 22 (Cloud Run service deployed manually first)
- GitHub repository exists and is accessible

**Required**:
- GCP Workload Identity Federation enabled
- GitHub Actions enabled on repository

---

## Notes

- Workload Identity Federation is more secure than service account keys
- Deploy on main branch only (not on tags or PRs)
- Each deployment tagged with commit SHA for rollback
- Zero-downtime deployments (Cloud Run handles traffic shifting)
- Manual trigger available for emergency deploys

**GitHub Actions free tier**: 2,000 minutes/month (sufficient for MVP)

---

## Related Documents

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [Cloud Run CI/CD](https://cloud.google.com/run/docs/continuous-deployment-with-cloud-build)

---

**Created**: 2026-03-17
**Last Updated**: 2026-03-17
