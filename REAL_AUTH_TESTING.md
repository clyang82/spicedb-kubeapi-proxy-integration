# Real Kubernetes Authentication Testing

This updated implementation now validates users against actual Kubernetes RBAC permissions before allowing SpiceDB operations.

## Authentication Methods Supported

1. **Bearer Token Authentication** - Uses `Authorization: Bearer <token>` header
2. **Client Certificate Authentication** - Uses mTLS client certificates  
3. **Header-based Authentication** - For development/testing with `X-Remote-User` header

## Testing Examples

### 1. Bearer Token Authentication

```bash
# Get a service account token
kubectl create serviceaccount testuser
kubectl create token testuser

# Use the token in API calls
curl -k -X POST \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -d '{"namespace": "test-ns"}' \
  https://your-proxy/api/namespaces/create
```

### 2. Client Certificate Authentication

```bash  
# Create client certificate for user
openssl genrsa -out user.key 2048
openssl req -new -key user.key -out user.csr -subj "/CN=testuser/O=developers"

# Sign the certificate with cluster CA
kubectl apply -f - <<EOF
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: testuser
spec:
  request: $(cat user.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF

kubectl certificate approve testuser
kubectl get csr testuser -o jsonpath='{.status.certificate}' | base64 -d > user.crt

# Use client certificate
curl -k -X POST \
  --cert user.crt --key user.key \
  -H "Content-Type: application/json" \
  -d '{"namespace": "test-ns"}' \
  https://your-proxy/api/namespaces/create
```

### 3. Header-based Authentication (Development)

```bash
curl -k -X POST \
  -H "X-Remote-User: testuser" \
  -H "X-Remote-Groups: developers,users" \
  -H "Content-Type: application/json" \
  -d '{"namespace": "test-ns"}' \
  https://your-proxy/api/namespaces/create
```

## RBAC Permission Checks

The proxy now checks Kubernetes RBAC permissions before allowing operations:

1. **For namespace creation**: Checks if user has `create` permission on `namespaces` resource
2. **For namespace listing**: Checks if user has `list` permission on `namespaces` resource

Example RBAC setup:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: namespace-manager
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["create", "list", "get"]

---
apiVersion: rbac.authorization.k8s.io/v1  
kind: ClusterRoleBinding
metadata:
  name: testuser-namespace-manager
subjects:
- kind: ServiceAccount
  name: testuser
  namespace: spicedb-proxy
roleRef:
  kind: ClusterRole
  name: namespace-manager
  apiGroup: rbac.authorization.k8s.io
```

## Flow Summary

1. **Authentication**: Extract user identity from request (token/cert/header)
2. **RBAC Check**: Validate user has Kubernetes permission for the operation
3. **SpiceDB Authorization**: Apply SpiceDB rules for fine-grained permissions
4. **Action**: Perform the requested operation if all checks pass

## API Changes

The API no longer requires `username` in request body since it's extracted from authentication:

**Before:**
```json
{
  "username": "testuser",
  "namespace": "test-ns"
}
```

**After:**
```json
{
  "namespace": "test-ns"
}
```

The response now includes the authenticated username:
```json
{
  "success": true,
  "data": {
    "namespace": "test-ns",
    "user": "testuser"
  }
}
```

## Error Responses

- `Authentication failed: <reason>` - Invalid or missing authentication
- `User does not have permission to <action> <resource>` - RBAC check failed
- `<specific SpiceDB error>` - SpiceDB authorization or operation error