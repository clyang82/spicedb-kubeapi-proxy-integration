#!/bin/bash

# Build and push script for SpiceDB KubeAPI Proxy
# Usage: ./scripts/build-and-push.sh [IMAGE_TAG]

set -e

# Configuration
IMAGE_TAG=${1:-"latest"}
REGISTRY="quay.io/clyang82"
IMAGE_NAME="spicedb-proxy-integration"

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

FULL_IMAGE="$REGISTRY/$IMAGE_NAME:$IMAGE_TAG"

log_info "Building and pushing $FULL_IMAGE"

# Check container CLI
if command -v docker &> /dev/null; then
    CONTAINER_CLI="docker"
elif command -v podman &> /dev/null; then
    CONTAINER_CLI="podman"
else
    log_error "Neither docker nor podman found. Please install one of them."
    exit 1
fi

# Build image for linux/amd64 platform
log_info "Building image with $CONTAINER_CLI for linux/amd64..."
$CONTAINER_CLI build --platform linux/amd64 -f Dockerfile -t "$FULL_IMAGE" .

# Login to registry (if needed)
log_info "Logging in to $REGISTRY..."
if ! $CONTAINER_CLI login $REGISTRY; then
    log_error "Failed to login to $REGISTRY"
    log_info "Please run: $CONTAINER_CLI login $REGISTRY"
    exit 1
fi

# Push image
log_info "Pushing image to registry..."
$CONTAINER_CLI push "$FULL_IMAGE"

log_info "✓ Successfully built and pushed $FULL_IMAGE"
echo
log_info "You can now deploy with:"
echo "  ./scripts/deploy.sh $IMAGE_TAG"
echo
log_info "Or update your deployment manually:"
echo "  oc set image deployment/spicedb-proxy-integration integration=$FULL_IMAGE -n spicedb-proxy"