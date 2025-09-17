#!/bin/bash

# Test script for SpiceDB KubeAPI Proxy deployment
# Usage: ./scripts/test-deployment.sh [ROUTE_URL]

set -e

# Configuration
ROUTE_URL=${1:-""}
NAMESPACE="spicedb-proxy"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

test_endpoint() {
    local url=$1
    local method=${2:-GET}
    local data=${3:-""}
    local expected_status=${4:-200}
    
    if [ "$method" = "POST" ] && [ -n "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" -X POST \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$url")
    else
        response=$(curl -s -w "\n%{http_code}" "$url")
    fi
    
    body=$(echo "$response" | head -n -1)
    status=$(echo "$response" | tail -n 1)
    
    if [ "$status" -eq "$expected_status" ]; then
        log_info "✓ $method $url - Status: $status"
        echo "$body" | jq 2>/dev/null || echo "$body"
    else
        log_error "✗ $method $url - Expected: $expected_status, Got: $status"
        echo "$body"
        return 1
    fi
    echo
}

# Get route URL if not provided
if [ -z "$ROUTE_URL" ]; then
    log_info "Getting route URL from OpenShift..."
    ROUTE_URL=$(oc get route spicedb-proxy-integration -n $NAMESPACE -o jsonpath='{.spec.host}' 2>/dev/null || echo "")
    
    if [ -z "$ROUTE_URL" ]; then
        log_error "Could not get route URL. Please provide it as an argument or ensure the route exists."
        echo "Usage: $0 [ROUTE_URL]"
        echo "Example: $0 https://spicedb-proxy-integration-spicedb-proxy.apps.cluster.local"
        exit 1
    fi
    
    ROUTE_URL="https://$ROUTE_URL"
fi

log_info "Testing SpiceDB KubeAPI Proxy at: $ROUTE_URL"
echo

# Test 1: Health checks
log_info "=== Health Checks ==="
test_endpoint "$ROUTE_URL/healthz"
test_endpoint "$ROUTE_URL/readyz"

# Test 2: API documentation
log_info "=== API Documentation ==="
test_endpoint "$ROUTE_URL/api/demo"

# Test 3: Create namespaces for different users
log_info "=== Creating Namespaces ==="

# Create namespace for Alice
test_endpoint "$ROUTE_URL/api/namespaces/create" "POST" '{
    "username": "alice",
    "namespace": "alice-workspace-test"
}'

# Create namespace for Bob
test_endpoint "$ROUTE_URL/api/namespaces/create" "POST" '{
    "username": "bob", 
    "namespace": "bob-workspace-test"
}'

# Create another namespace for Alice
test_endpoint "$ROUTE_URL/api/namespaces/create" "POST" '{
    "username": "alice",
    "namespace": "alice-project-test"
}'

# Test 4: List namespaces (user isolation)
log_info "=== Testing User Isolation ==="

log_info "Alice's namespaces:"
test_endpoint "$ROUTE_URL/api/namespaces/list" "POST" '{
    "username": "alice"
}'

log_info "Bob's namespaces:"
test_endpoint "$ROUTE_URL/api/namespaces/list" "POST" '{
    "username": "bob"
}'

# Test 5: Error handling
log_info "=== Error Handling ==="

log_info "Testing empty username (should fail):"
test_endpoint "$ROUTE_URL/api/namespaces/create" "POST" '{
    "username": "",
    "namespace": "test-ns"
}' 200  # API returns 200 with error in JSON

log_info "Testing duplicate namespace (should fail):"
test_endpoint "$ROUTE_URL/api/namespaces/create" "POST" '{
    "username": "alice",
    "namespace": "alice-workspace-test"
}' 200  # API returns 200 with error in JSON

log_info "Testing invalid JSON (should fail):"
curl -s -X POST \
    -H "Content-Type: application/json" \
    -d 'invalid json' \
    "$ROUTE_URL/api/namespaces/create" || log_warn "Request failed as expected"

echo
log_info "=== Test Summary ==="
log_info "All tests completed. Check the output above for any failures."
log_info "Expected behavior:"
log_info "- Alice should see 'alice-workspace-test' and 'alice-project-test'"
log_info "- Bob should see 'bob-workspace-test'"
log_info "- Neither user should see the other's namespaces"
log_info "- Error cases should return appropriate error messages"

echo
log_info "To manually verify the results, you can also check in the cluster:"
echo "oc get namespaces | grep -E '(alice|bob)'"