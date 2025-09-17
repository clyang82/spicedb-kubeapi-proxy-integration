# Quick Test Reference - SpiceDB KubeAPI Proxy

## Quick Start Commands

```bash
# 1. Deploy to OpenShift
./scripts/deploy.sh

# 2. Test the deployment
./scripts/test-deployment.sh

# 3. Clean up when done
./scripts/cleanup.sh
```

## Manual API Testing

### Get Route URL
```bash
ROUTE_URL=$(oc get route spicedb-proxy-integration -n spicedb-proxy -o jsonpath='{.spec.host}')
export BASE_URL="https://$ROUTE_URL"
```

### Health Checks
```bash
curl $BASE_URL/healthz
curl $BASE_URL/readyz
curl $BASE_URL/api/demo | jq
```

### Create Namespaces
```bash
# Alice creates her workspace
curl -X POST $BASE_URL/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "namespace": "alice-workspace"}' | jq

# Bob creates his workspace
curl -X POST $BASE_URL/api/namespaces/create \
  -H "Content-Type: application/json" \
  -d '{"username": "bob", "namespace": "bob-workspace"}' | jq
```

### Test User Isolation
```bash
# Alice lists her namespaces
curl -X POST $BASE_URL/api/namespaces/list \
  -H "Content-Type: application/json" \
  -d '{"username": "alice"}' | jq

# Bob lists his namespaces  
curl -X POST $BASE_URL/api/namespaces/list \
  -H "Content-Type: application/json" \
  -d '{"username": "bob"}' | jq
```

## Expected Results

### ✅ Success: Alice sees only her namespaces
```json
{
  "success": true,
  "data": {
    "namespaces": ["alice-workspace"]
  }
}
```

### ✅ Success: Bob sees only his namespaces
```json
{
  "success": true,
  "data": {
    "namespaces": ["bob-workspace"]
  }
}
```

### ✅ Success: Error handling works
```json
{
  "success": false,
  "error": "Username and namespace are required"
}
```

## Verification Commands

```bash
# Check actual namespaces in cluster
oc get namespaces | grep -E "(alice|bob)"

# Check logs for authorization decisions
oc logs -f deployment/spicedb-proxy-integration -n spicedb-proxy | grep authz

# Check SpiceDB relationships
oc logs -f deployment/spicedb-proxy-integration -n spicedb-proxy | grep relationship
```

## Troubleshooting

```bash
# Check pod status
oc get pods -n spicedb-proxy

# View logs
oc logs deployment/spicedb-proxy-integration -n spicedb-proxy

# Debug with port-forward
oc port-forward svc/spicedb-proxy-integration 8080:8080 -n spicedb-proxy
curl http://localhost:8080/healthz
```

## Key Features Demonstrated

1. **User Isolation**: Each user only sees namespaces they created
2. **Authorization**: SpiceDB checks permissions for all operations  
3. **Embedded Mode**: No external SpiceDB required
4. **REST API**: Simple HTTP API for testing
5. **OpenShift Integration**: Deployed as standard Kubernetes workload