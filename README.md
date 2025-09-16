# SpiceDB KubeAPI Proxy - OpenShift Deployment Guide

This guide shows how to deploy the embedded SpiceDB KubeAPI Proxy integration to OpenShift/Kubernetes and test it manually.

## Prerequisites

- OpenShift/Kubernetes cluster access
- `oc` or `kubectl` CLI tools
- Docker/Podman for building images
- `curl` or similar HTTP client for testing

## Deployment Steps

### 1. Build and Push Container Image

```bash
# Build the container image
docker build -f deployment/Dockerfile -t spicedb-proxy-integration:latest .

# Tag for your registry
docker tag spicedb-proxy-integration:latest quay.io/clyang82/spicedb-proxy-integration:latest

# Push to registry
docker push quay.io/clyang82/spicedb-proxy-integration:latest
```

For OpenShift internal registry:
```bash
# Log in to OpenShift registry
oc registry login

# Tag for internal registry
docker tag spicedb-proxy-integration:latest default-route-openshift-image-registry.apps.cluster.local/spicedb-proxy/spicedb-proxy-integration:latest

# Push to internal registry
docker push default-route-openshift-image-registry.apps.cluster.local/spicedb-proxy/spicedb-proxy-integration:latest
```

### 2. Deploy to OpenShift

```bash
# Apply RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)
oc apply -f deployment/rbac.yaml

# Deploy the application
oc apply -f deployment/deployment.yaml

# Create OpenShift route (for external access)
oc apply -f deployment/route.yaml
```

### 3. Verify Deployment

```bash
# Check if pods are running
oc get pods -n spicedb-proxy

# Check logs
oc logs -f deployment/spicedb-proxy-integration -n spicedb-proxy

# Check service
oc get svc -n spicedb-proxy

# Get route URL (OpenShift)
oc get route spicedb-proxy-integration -n spicedb-proxy -o jsonpath='{.spec.host}'
```

## Manual Testing

### Health Checks

```bash
# Get the route URL
ROUTE_URL=$(oc get route spicedb-proxy-integration -n spicedb-proxy -o jsonpath='{.spec.host}')

# Test health endpoint
curl https://$ROUTE_URL/healthz

# Test readiness
curl https://$ROUTE_URL/readyz

# Get API documentation
curl https://$ROUTE_URL/api/demo | jq
```

### API Testing

#### 1. Create Namespaces for Different Users

```bash
# Create namespace for Alice
curl -X POST https://$ROUTE_URL/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "namespace": "alice-workspace"
  }' | jq

# Create namespace for Bob
curl -X POST https://$ROUTE_URL/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{
    "username": "bob", 
    "namespace": "bob-workspace"
  }' | jq

# Try to create another namespace for Alice
curl -X POST https://$ROUTE_URL/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "namespace": "alice-project-1"
  }' | jq
```

#### 2. Test User Isolation

```bash
# List namespaces Alice can see
curl -X POST https://$ROUTE_URL/api/namespaces/list \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice"
  }' | jq

# List namespaces Bob can see
curl -X POST https://$ROUTE_URL/api/namespaces/list \
  -H "Content-Type: application/json" \
  -d '{
    "username": "bob"
  }' | jq

# Verify isolation: Alice should only see her namespaces, Bob should only see his
```

#### 3. Test Error Handling

```bash
# Try to create namespace with empty username
curl -X POST https://$ROUTE_URL/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{
    "username": "",
    "namespace": "test-ns"
  }' | jq

# Try to create duplicate namespace
curl -X POST https://$ROUTE_URL/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "namespace": "alice-workspace"
  }' | jq
```

### Direct Kubernetes Testing

You can also test by accessing the service directly from within the cluster:

```bash
# Port forward to test locally
oc port-forward svc/spicedb-proxy-integration 8080:8080 -n spicedb-proxy

# In another terminal, test locally
curl http://localhost:8080/healthz
curl http://localhost:8080/api/demo | jq

# Create namespace via port-forward
curl -X POST http://localhost:8080/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "namespace": "test-local"}' | jq
```

## Expected Test Results

### Successful Namespace Creation
```json
{
  "success": true,
  "data": {
    "namespace": "alice-workspace"
  }
}
```

### User Isolation Working
Alice's namespace list:
```json
{
  "success": true,
  "data": {
    "namespaces": [
      "alice-workspace",
      "alice-project-1"
    ]
  }
}
```

Bob's namespace list:
```json
{
  "success": true,
  "data": {
    "namespaces": [
      "bob-workspace"
    ]
  }
}
```

### Error Response
```json
{
  "success": false,
  "error": "Username and namespace are required"
}
```

## Advanced Testing with kubectl

You can also verify the authorization by using kubectl with the embedded proxy:

```bash
# Get a shell in the pod
oc exec -it deployment/spicedb-proxy-integration -n spicedb-proxy -- /bin/sh

# Inside the pod, you can test the embedded client directly
# (This would require adding kubectl to the container image)
```

## Troubleshooting

### Common Issues

1. **Pod not starting**
   ```bash
   oc describe pod -l app=spicedb-proxy-integration -n spicedb-proxy
   oc logs -l app=spicedb-proxy-integration -n spicedb-proxy
   ```

2. **RBAC issues**
   ```bash
   # Check ServiceAccount
   oc get sa spicedb-proxy-integration -n spicedb-proxy
   
   # Check ClusterRoleBinding
   oc get clusterrolebinding spicedb-proxy-integration
   ```

3. **Image pull issues**
   ```bash
   # Check image pull policy and registry access
   oc describe pod -l app=spicedb-proxy-integration -n spicedb-proxy
   ```

4. **Route not accessible**
   ```bash
   # Check route status
   oc get route spicedb-proxy-integration -n spicedb-proxy
   
   # Check service
   oc get svc spicedb-proxy-integration -n spicedb-proxy
   ```

### Debug Logs

Enable debug logging by adding environment variable to deployment:
```yaml
env:
- name: LOG_LEVEL
  value: "debug"
```

### Monitoring

Check proxy metrics and SpiceDB operations:
```bash
# Watch logs for authorization decisions
oc logs -f deployment/spicedb-proxy-integration -n spicedb-proxy | grep authz

# Watch for SpiceDB relationship operations
oc logs -f deployment/spicedb-proxy-integration -n spicedb-proxy | grep "relationship"
```

## Production Considerations

### Security
- Use proper TLS certificates
- Configure network policies
- Use secrets for sensitive configuration
- Enable OpenShift Security Context Constraints (SCC)

### Scaling
- The embedded SpiceDB uses in-memory storage by default
- For production, consider external SpiceDB with persistent storage
- Scale horizontally by running multiple replicas

### Monitoring
- Add Prometheus metrics
- Configure health checks
- Set up alerting for proxy failures

### Configuration
- Use ConfigMaps for rules configuration
- Use Secrets for sensitive data
- Configure resource limits appropriately

## Next Steps

1. **Custom Resource Definitions**: Add support for your custom resources
2. **RBAC Integration**: Integrate with OpenShift RBAC
3. **Audit Logging**: Enable comprehensive audit logs
4. **Performance Testing**: Load test the integration
5. **CI/CD Integration**: Automate deployment pipeline
