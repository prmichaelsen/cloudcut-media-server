#!/bin/bash
set -e

PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
if [ -z "$PROJECT_ID" ]; then
  echo "❌ No GCP project set. Please run: gcloud config set project YOUR_PROJECT_ID"
  exit 1
fi

function create_secret() {
    local name=$1
    local value=$2

    echo "Creating secret: ${name}"
    echo -n "${value}" | gcloud secrets create ${name} \
        --replication-policy="automatic" \
        --data-file=- 2>/dev/null || \
    echo -n "${value}" | gcloud secrets versions add ${name} --data-file=-

    echo "✅ Secret ${name} created/updated"
}

function delete_secret() {
    local name=$1
    gcloud secrets delete ${name} --quiet
    echo "✅ Secret ${name} deleted"
}

function list_secrets() {
    echo "Secrets in project ${PROJECT_ID}:"
    echo ""
    gcloud secrets list --format="table(name,createTime,labels)"
}

function get_secret() {
    local name=$1
    echo "Secret ${name} value:"
    gcloud secrets versions access latest --secret=${name}
}

function grant_access() {
    local secret_name=$1
    local sa_email=${2:-"cloudcut-server@${PROJECT_ID}.iam.gserviceaccount.com"}

    echo "Granting ${sa_email} access to ${secret_name}..."
    gcloud secrets add-iam-policy-binding ${secret_name} \
      --member="serviceAccount:${sa_email}" \
      --role="roles/secretmanager.secretAccessor" \
      --quiet

    echo "✅ Access granted"
}

function create_jwt_secret() {
    echo "Generating JWT secret..."
    local jwt_value="jwt-secret-$(openssl rand -hex 32)"
    create_secret "jwt-secret" "${jwt_value}"
    grant_access "jwt-secret"
}

# Command dispatcher
case "${1}" in
    create)
        if [ -z "${2}" ] || [ -z "${3}" ]; then
            echo "Usage: $0 create <name> <value>"
            exit 1
        fi
        create_secret "${2}" "${3}"
        ;;
    delete)
        if [ -z "${2}" ]; then
            echo "Usage: $0 delete <name>"
            exit 1
        fi
        delete_secret "${2}"
        ;;
    list)
        list_secrets
        ;;
    get)
        if [ -z "${2}" ]; then
            echo "Usage: $0 get <name>"
            exit 1
        fi
        get_secret "${2}"
        ;;
    grant)
        if [ -z "${2}" ]; then
            echo "Usage: $0 grant <secret-name> [service-account-email]"
            exit 1
        fi
        grant_access "${2}" "${3}"
        ;;
    init)
        echo "Initializing secrets for cloudcut-media-server..."
        create_jwt_secret
        echo ""
        echo "✅ Secrets initialized"
        ;;
    *)
        echo "Secret Manager - cloudcut-media-server"
        echo ""
        echo "Usage: $0 {create|delete|list|get|grant|init}"
        echo ""
        echo "Commands:"
        echo "  create <name> <value>         - Create or update secret"
        echo "  delete <name>                 - Delete secret"
        echo "  list                          - List all secrets"
        echo "  get <name>                    - Get secret value"
        echo "  grant <name> [sa-email]       - Grant service account access to secret"
        echo "  init                          - Initialize default secrets (jwt-secret)"
        echo ""
        echo "Examples:"
        echo "  $0 create my-api-key \"sk-abc123\""
        echo "  $0 grant my-api-key cloudcut-server@PROJECT_ID.iam.gserviceaccount.com"
        echo "  $0 list"
        echo "  $0 get my-api-key"
        echo ""
        exit 1
        ;;
esac
