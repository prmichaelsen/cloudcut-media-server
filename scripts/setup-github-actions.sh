#!/bin/bash
set -e

echo "🚀 Setting up Workload Identity Federation for GitHub Actions"
echo ""

# Get project details
export PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
if [ -z "$PROJECT_ID" ]; then
  echo "❌ No GCP project set. Please run: gcloud config set project YOUR_PROJECT_ID"
  exit 1
fi

export PROJECT_NUMBER=$(gcloud projects describe ${PROJECT_ID} --format='value(projectNumber)')
export POOL_NAME="github-actions-pool"
export PROVIDER_NAME="github-actions-provider"
export SA_NAME="github-actions-deployer"
export REGION="us-central1"

# Get GitHub repository from git remote
REPO_URL=$(git config --get remote.origin.url 2>/dev/null || echo "")
if [ -z "$REPO_URL" ]; then
  echo "❌ No git remote found. Please add GitHub remote first:"
  echo "   git remote add origin https://github.com/USERNAME/REPO.git"
  exit 1
fi

# Extract owner/repo from URL
if [[ $REPO_URL =~ github\.com[:/](.+/.+)(\.git)?$ ]]; then
  export REPO="${BASH_REMATCH[1]%.git}"
else
  echo "❌ Could not parse GitHub repository from remote URL: $REPO_URL"
  exit 1
fi

echo "Project ID: ${PROJECT_ID}"
echo "Project Number: ${PROJECT_NUMBER}"
echo "GitHub Repository: ${REPO}"
echo ""

# Create Workload Identity Pool
echo "🔐 Creating Workload Identity Pool..."
if gcloud iam workload-identity-pools describe ${POOL_NAME} --location="global" >/dev/null 2>&1; then
  echo "✓ Pool already exists"
else
  gcloud iam workload-identity-pools create ${POOL_NAME} \
    --location="global" \
    --display-name="GitHub Actions Pool" \
    --quiet
  echo "✅ Pool created"
fi
echo ""

# Create Workload Identity Provider
echo "🔗 Creating Workload Identity Provider..."
if gcloud iam workload-identity-pools providers describe ${PROVIDER_NAME} \
    --workload-identity-pool=${POOL_NAME} --location="global" >/dev/null 2>&1; then
  echo "✓ Provider already exists"
else
  gcloud iam workload-identity-pools providers create-oidc ${PROVIDER_NAME} \
    --location="global" \
    --workload-identity-pool=${POOL_NAME} \
    --display-name="GitHub Actions Provider" \
    --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository" \
    --issuer-uri="https://token.actions.githubusercontent.com" \
    --quiet
  echo "✅ Provider created"
fi
echo ""

# Create service account for GitHub Actions
echo "👤 Creating service account for GitHub Actions..."
export SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

if gcloud iam service-accounts describe ${SA_EMAIL} >/dev/null 2>&1; then
  echo "✓ Service account already exists"
else
  gcloud iam service-accounts create ${SA_NAME} \
    --display-name="GitHub Actions Deployer" \
    --quiet
  echo "✅ Service account created"
fi
echo ""

# Grant required roles
echo "🔐 Granting IAM roles..."

echo "  → run.admin"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/run.admin" \
  --quiet || true

echo "  → iam.serviceAccountUser"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/iam.serviceAccountUser" \
  --quiet || true

echo "  → artifactregistry.writer"
gcloud projects add-iam-policy-binding ${PROJECT_ID} \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/artifactregistry.writer" \
  --quiet || true

echo "✅ IAM roles granted"
echo ""

# Allow GitHub Actions to impersonate service account
echo "🔗 Allowing GitHub Actions to impersonate service account..."
gcloud iam service-accounts add-iam-policy-binding ${SA_EMAIL} \
  --role="roles/iam.workloadIdentityUser" \
  --member="principalSet://iam.googleapis.com/projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_NAME}/attribute.repository/${REPO}" \
  --quiet || true

echo "✅ Impersonation configured"
echo ""

# Get Workload Identity Provider resource name
export WIF_PROVIDER="projects/${PROJECT_NUMBER}/locations/global/workloadIdentityPools/${POOL_NAME}/providers/${PROVIDER_NAME}"

echo "═══════════════════════════════════════════════════════"
echo "  ✅ Workload Identity Federation Setup Complete"
echo "═══════════════════════════════════════════════════════"
echo ""
echo "Add these secrets to GitHub repository settings:"
echo "  https://github.com/${REPO}/settings/secrets/actions"
echo ""
echo "Secret values:"
echo "  GCP_PROJECT_ID=${PROJECT_ID}"
echo "  WIF_PROVIDER=${WIF_PROVIDER}"
echo "  WIF_SERVICE_ACCOUNT=${SA_EMAIL}"
echo ""
echo "Optional (for code coverage):"
echo "  CODECOV_TOKEN=<your-codecov-token>"
echo ""
echo "Next steps:"
echo "  1. Add the secrets above to GitHub"
echo "  2. Push .github/workflows to trigger CI/CD"
echo "  3. Create a PR to test the workflows"
echo ""
