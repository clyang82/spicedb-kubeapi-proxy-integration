#!/bin/bash

# Cleanup script for SpiceDB KubeAPI Proxy deployment
# Usage: ./scripts/cleanup.sh

set -e

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

log_info "Cleaning up SpiceDB KubeAPI Proxy deployment..."

# Delete route (OpenShift)
if oc get route spicedb-proxy-integration -n $NAMESPACE &>/dev/null; then
    log_info "Deleting route..."
    oc delete -f deployment/route.yaml --ignore-not-found=true
fi

# Delete deployment
log_info "Deleting deployment..."
oc delete -f deployment/deployment.yaml --ignore-not-found=true

# Delete RBAC
log_info "Deleting RBAC configuration..."
oc delete -f deployment/rbac.yaml --ignore-not-found=true

# Clean up test namespaces
log_info "Cleaning up test namespaces..."
oc delete namespace alice-workspace-test --ignore-not-found=true
oc delete namespace alice-project-test --ignore-not-found=true
oc delete namespace bob-workspace-test --ignore-not-found=true

# Optionally delete the spicedb-proxy namespace
read -p "Delete the spicedb-proxy namespace entirely? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Deleting namespace $NAMESPACE..."
    oc delete namespace $NAMESPACE --ignore-not-found=true
else
    log_info "Keeping namespace $NAMESPACE"
fi

log_info "Cleanup completed!"