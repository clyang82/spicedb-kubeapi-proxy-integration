# Testing spicedb-kubeapi-proxy

This guide explains how to test the spicedb-kubeapi-proxy in different ways.

## Prerequisites

Install required tools:
```bash
# Install mage (build tool)
brew install mage  # macOS
# or
go install github.com/magefile/mage@latest

# Install kind (for local Kubernetes clusters)
brew install kind  # macOS
# or
go install sigs.k8s.io/kind@latest

# Install kustomizer (for applying manifests)
go install github.com/stefanprodan/kustomizer@latest

# Install kubectx (for switching contexts)
brew install kubectx  # macOS
```

## Testing Methods

### 1. Quick Unit Tests

Run unit tests only:
```bash
mage test:unit
```

This runs:
- All Go unit tests with coverage
- Outputs coverage to `coverageunit.txt`

### 2. End-to-End Tests

Run full integration tests against a real API server:
```bash
mage test:e2e
```

This runs:
- End-to-end tests using Ginkgo framework
- Tests against a real Kubernetes API server
- Outputs coverage to `coveragee2e.txt`

### 3. All Tests

Run both unit and e2e tests:
```bash
mage test:all
```

### 4. Development Environment Testing

#### Step 1: Set up development cluster
```bash
mage dev:up
```

This creates:
- A Kind cluster named `spicedb-kubeapi-proxy`
- Deploys the proxy with embedded SpiceDB
- Generates `dev.kubeconfig` with multiple contexts

#### Step 2: Test the proxy
```bash
# Set up environment
export KUBECONFIG=$(pwd)/dev.kubeconfig

# Switch to proxy context
kubectx proxy

# Test basic functionality
kubectl get namespaces

# Test with different contexts:
kubectl --context proxy get namespace     # Through proxy
kubectl --context admin get namespace     # Direct to cluster
```

#### Step 3: Clean up
```bash
mage dev:clean
```

### 5. Local Proxy Testing

Run the proxy locally for debugging:

#### Option 1: Using mage (requires dev environment)
```bash
# First set up dev cluster
mage dev:up

# Run proxy locally
mage dev:run
```

#### Option 2: Direct Go execution
```bash
go run ./cmd/spicedb-kubeapi-proxy/main.go \
  --bind-address=127.0.0.1 \
  --secure-port=8443 \
  --backend-kubeconfig $(pwd)/spicedb-kubeapi-proxy.kubeconfig \
  --client-ca-file $(pwd)/client-ca.crt \
  --requestheader-client-ca-file $(pwd)/client-ca.crt \
  --spicedb-endpoint embedded://
```

Then test with:
```bash
export KUBECONFIG=$(pwd)/dev.kubeconfig
kubectx local
kubectl --insecure-skip-tls-verify get namespace
```

## Test Configuration

### Authorization Rules

The proxy uses rules defined in `deploy/rules.yaml` which include:

1. **Namespace Rules**:
   - Create: Sets up creator relationship
   - Delete: Removes relationships
   - Get: Requires view permission
   - List/Watch: Pre-filters based on permissions

2. **Pod Rules**:
   - Create: Links to namespace, sets creator
   - Delete: Removes relationships
   - Get: Requires view permission
   - List/Watch: Pre-filters by namespace and permissions

### Example Test Scenarios

The e2e tests (`e2e/proxy_test.go`) cover scenarios like:

- **User Isolation**: Users can only see their own resources
- **Permission Checks**: Unauthorized access returns 403
- **Resource Filtering**: List operations only show allowed resources
- **Distributed Transactions**: Resource creation updates both K8s and SpiceDB
- **Watch Filtering**: Real-time filtering of watch events

### Custom Testing

To create custom tests:

1. **Define Rules**: Create YAML rules in `deploy/rules.yaml`
2. **Set up Users**: Generate client certificates for different users
3. **Test Scenarios**: Use different kubeconfig contexts to simulate users
4. **Verify SpiceDB**: Check relationships are created/deleted correctly

## Debugging

### Enable Verbose Logging
```bash
# Add to proxy command
--v=4  # Kubernetes logging level
```

### Check SpiceDB State
The embedded SpiceDB stores data in SQLite. You can inspect relationships through the proxy's permission client.

### Common Issues

1. **Certificate Errors**: Ensure client certificates are properly generated
2. **Rule Matching**: Verify rules match your test resources (GVR + verb)
3. **Permission Failures**: Check SpiceDB relationships exist
4. **Port Conflicts**: Ensure ports 8443 and 30443 are available

## Continuous Testing

For development, run tests continuously:
```bash
mage test:e2eUntilItFails  # Runs until failure for stress testing
```

This is useful for catching race conditions or intermittent failures.

## Test Structure

- `magefiles/test.go`: Test automation scripts
- `magefiles/dev.go`: Development environment setup
- `e2e/`: End-to-end test suite using Ginkgo
- `deploy/rules.yaml`: Authorization rules for testing
- `*_test.go`: Unit tests throughout the codebase