#!/bin/bash

# Configuration
FILE_MASTER=k8s-manifests.yaml
FILE_DEPLOYMENT=deployment-${COMPANY_ID}.yaml
# TAG="$(git describe --tags --abbrev=0 2>/dev/null || echo "latest-dev")"
TAG="latest-dev"
IMAGE_REGISTRY="registry.gitlab.com/timkado/api"  # Update with your registry

# Required environment variables
COMPANY_ID=${COMPANY_ID:-""}
AGENT_ID=${AGENT_ID:-""}
BEARER_TOKEN=${BEARER_TOKEN:-""}
GMAIL_CLIENT_ID=${GMAIL_CLIENT_ID:-""}
GMAIL_CLIENT_SECRET=${GMAIL_CLIENT_SECRET:-""}
GMAIL_REFRESH_TOKEN=${GMAIL_REFRESH_TOKEN:-""}
MONGODB_DSN=${MONGODB_DSN:-""}
POSTGRES_DSN=${POSTGRES_DSN:-""}

# Validate required variables
if [ -z "$COMPANY_ID" ]; then
    echo "Error: COMPANY_ID is required"
    exit 1
fi

if [ -z "$AGENT_ID" ] || [ -z "$BEARER_TOKEN" ]; then
    echo "Error: AGENT_ID and BEARER_TOKEN are required"
    exit 1
fi

if [ -z "$MONGODB_DSN" ] || [ -z "$POSTGRES_DSN" ]; then
    echo "Error: MONGODB_DSN and POSTGRES_DSN are required"
    exit 1
fi

if [ -z "$GMAIL_CLIENT_ID" ] || [ -z "$GMAIL_CLIENT_SECRET" ] || [ -z "$GMAIL_REFRESH_TOKEN" ]; then
    echo "Error: Gmail OAuth credentials are required (GMAIL_CLIENT_ID, GMAIL_CLIENT_SECRET, GMAIL_REFRESH_TOKEN)"
    exit 1
fi

echo "Deploying mailcast-gmail-service for company: ${COMPANY_ID}"
echo "Using image tag: ${TAG}"

# Copy master file
cp ${FILE_MASTER} ${FILE_DEPLOYMENT}

# Replace placeholders
sed -i -e "s/<company_id>/${COMPANY_ID}/g" \
       -e "s/<agent_id>/${AGENT_ID}/g" \
       -e "s/<bearer_token>/${BEARER_TOKEN}/g" \
       -e "s/<mongodb_dsn>/${MONGODB_DSN}/g" \
       -e "s/<postgres_dsn>/${POSTGRES_DSN}/g" \
       -e "s|<image_registry>|${IMAGE_REGISTRY}|g" \
       -e "s|<version>|${TAG}|g" \
       -e "s|<gmail_client_id>|${GMAIL_CLIENT_ID}|g" \
       -e "s|<gmail_client_secret>|${GMAIL_CLIENT_SECRET}|g" \
       -e "s|<gmail_refresh_token>|${GMAIL_REFRESH_TOKEN}|g" ${FILE_DEPLOYMENT}

# Deploy to Kubernetes
echo "Applying Kubernetes manifests..."
kubectl apply -f ${FILE_DEPLOYMENT} -n tenant-dev-doks

# Check deployment status
echo "Checking deployment status..."
kubectl rollout status deployment/mailcast-gmail-service-${COMPANY_ID} -n tenant-dev-doks

# Show pods
echo "Pods for company ${COMPANY_ID}:"
kubectl get pods -n tenant-dev-doks -l app=mailcast-gmail-service-${COMPANY_ID}

# Cleanup
rm ${FILE_DEPLOYMENT}

echo "Deployment complete!"