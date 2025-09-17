#!/bin/bash

# Deployment script for SpiceDB KubeAPI Proxy
# Usage: ./scripts/deploy.sh [IMAGE_TAG]

set -e

# Configuration
IMAGE_TAG=${1:-"latest"}
REGISTRY=${REGISTRY:-"quay.io/clyang82"}
IMAGE_NAME="spicedb-proxy-integration"
NAMESPACE="spicedb-proxy"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're logged into OpenShift/Kubernetes
if ! oc whoami &>/dev/null && ! kubectl cluster-info &>/dev/null; then
    log_error "Not logged into OpenShift/Kubernetes cluster"
    exit 1
fi

log_info "Deploying SpiceDB KubeAPI Proxy Integration..."
echo

# Step 1: Build and push image
log_info "=== Building Container Image ==="
FULL_IMAGE="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"

if command -v docker &> /dev/null; then
    CONTAINER_CLI="docker"
elif command -v podman &> /dev/null; then
    CONTAINER_CLI="podman"
else
    log_error "Neither docker nor podman found. Please install one of them."
    exit 1
fi

log_info "Building image with $CONTAINER_CLI..."
$CONTAINER_CLI build -f deployment/Dockerfile -t "$FULL_IMAGE" .

log_info "Pushing image to registry..."
$CONTAINER_CLI push "$FULL_IMAGE"

# Step 2: Update deployment with new image
log_info "=== Updating Deployment Configuration ==="
if [ -f deployment/deployment.yaml.bak ]; then
    cp deployment/deployment.yaml.bak deployment/deployment.yaml
else
    cp deployment/deployment.yaml deployment/deployment.yaml.bak
fi

# Replace image in deployment
sed -i.tmp "s|image: spicedb-proxy-integration:latest|image: $FULL_IMAGE|g" deployment/deployment.yaml
rm deployment/deployment.yaml.tmp

# Step 3: Deploy to cluster
log_info "=== Deploying to Cluster ==="

log_info "Creating namespace (if it doesn't exist)..."
oc create namespace $NAMESPACE --dry-run=client -o yaml | oc apply -f -

log_info "Applying RBAC configuration..."
oc apply -f deployment/rbac.yaml

log_info "Deploying application..."
oc apply -f deployment/deployment.yaml

log_info "Creating route (OpenShift only)..."
if oc api-resources | grep -q routes; then
    oc apply -f deployment/route.yaml
else
    log_warn "Routes not available (not OpenShift?). Creating LoadBalancer service instead..."
    oc patch svc spicedb-proxy-integration -n $NAMESPACE -p '{"spec":{"type":"LoadBalancer"}}'
fi

# Step 4: Wait for deployment to be ready
log_info "=== Waiting for Deployment ==="
log_info "Waiting for pods to be ready..."
oc wait --for=condition=available --timeout=300s deployment/spicedb-proxy-integration -n $NAMESPACE

# Step 5: Get access information
log_info "=== Deployment Complete ==="
echo

log_info "Deployment Status:"
oc get pods -n $NAMESPACE -l app=spicedb-proxy-integration
echo

log_info "Service Information:"
oc get svc -n $NAMESPACE spicedb-proxy-integration
echo

if oc get route spicedb-proxy-integration -n $NAMESPACE &>/dev/null; then
    ROUTE_URL=$(oc get route spicedb-proxy-integration -n $NAMESPACE -o jsonpath='{.spec.host}')
    log_info "Route URL: https://$ROUTE_URL"
    echo
    log_info "Test the deployment with:"
    echo "  ./scripts/test-deployment.sh https://$ROUTE_URL"
else
    log_info "External IP (LoadBalancer):"
    oc get svc spicedb-proxy-integration -n $NAMESPACE -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
    echo
    log_info "Test the deployment with port-forward:"
    echo "  oc port-forward svc/spicedb-proxy-integration 8080:8080 -n $NAMESPACE"
    echo "  ./scripts/test-deployment.sh http://localhost:8080"
fi

log_info "Check logs with:"
echo "  oc logs -f deployment/spicedb-proxy-integration -n $NAMESPACE"

echo
log_info "Deployment completed successfully!"

# Restore original deployment file
if [ -f deployment/deployment.yaml.bak ]; then
    mv deployment/deployment.yaml.bak deployment/deployment.yaml
fi