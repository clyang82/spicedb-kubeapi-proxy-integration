# SpiceDB Schema Comparison: Complex vs Simple Approach

This document compares two different SpiceDB schema approaches for Kubernetes authorization: the complex hierarchical schema with automatic inheritance versus the simple flat schema with explicit permissions.

## Overview

### [Complex Schema](spicedb-kubernetes-schema-example.md)
- Uses 4 object types: `user`, `cluster`, `namespace`, `resource`, `subresource`
- Implements automatic permission inheritance through relationships
- Models Kubernetes hierarchy explicitly in the schema

### [Simple Schema](spicedb-kubernetes-schema-simple.md)
- Uses 2 object types: `user`, `resource`
- Encodes hierarchy in resource IDs
- Requires explicit permission grants at each level

## Detailed Comparison

| Aspect | Complex Schema | Simple Schema |
|--------|----------------|---------------|
| **Object Types** | 4 types: user, cluster, namespace, resource, subresource | 2 types: user, resource |
| **Hierarchy Modeling** | Explicit object relations (`namespace->cluster`) | Encoded in resource ID (`cluster/prod/namespace/default`) |
| **Permission Inheritance** | Automatic (`namespace->view + cluster->view`) | None - explicit permissions only |
| **Schema Complexity** | High - complex permission calculations | Low - simple permission rules |
| **Relationship Count** | Many - each resource needs 2-3 relationships | Fewer - 1 relationship per permission level |
| **Permission Propagation** | Cluster admin → all namespaces → all resources | No propagation - explicit grants only |
| **Learning Curve** | Steep - requires understanding inheritance | Shallow - straightforward permission model |
| **Debugging** | Complex - permission chains can be hard to trace | Simple - direct permission relationships |

## Permission Inheritance Examples

### Complex Schema: Automatic Inheritance

**When alice is granted cluster admin:**
```yaml
cluster:prod#admin@user:alice
```

**Alice automatically gets:**
- ✅ `namespace:default#manage` (via `cluster->manage`)
- ✅ `namespace:kube-system#manage` (via `cluster->manage`)
- ✅ `resource:io/k8s/core/pods/default/nginx#read` (via `cluster->view`)
- ✅ `resource:io/k8s/core/pods/default/nginx#write` (via `cluster->manage`)
- ✅ `resource:io/k8s/core/pods/default/nginx#delete` (via `cluster->manage`)
- ✅ **ALL resources in ALL namespaces in cluster:prod**

**Permission Flow:**
```
alice → cluster:prod#admin → namespace:*#manage → resource:*#write/delete
```

### Simple Schema: Explicit Only

**Alice needs explicit grants:**
```yaml
resource:cluster/prod#owner@user:alice
resource:cluster/prod/namespace/default#owner@user:alice  
resource:cluster/prod/namespace/default/pod/nginx#owner@user:alice
```

**Alice only gets what's explicitly granted:**
- ✅ `resource:cluster/prod#read/write/delete`
- ✅ `resource:cluster/prod/namespace/default#read/write/delete`
- ✅ `resource:cluster/prod/namespace/default/pod/nginx#read/write/delete`
- ❌ `resource:cluster/prod/namespace/default/pod/web-server` (not granted)

## Relationship Management

### Complex Schema
```yaml
# Each resource needs multiple relationships
resource:io/k8s/core/pods/default/nginx#namespace@namespace:default
resource:io/k8s/core/pods/default/nginx#cluster@cluster:prod  
resource:io/k8s/core/pods/default/nginx#owner@user:frank

# Namespace must be linked to cluster
namespace:default#cluster@cluster:prod

# Benefits: Once linked, permissions flow automatically
# Drawbacks: More relationships to maintain per resource
```

### Simple Schema
```yaml
# Single relationship per access level
resource:cluster/prod/namespace/default/pod/nginx#owner@user:eve
resource:cluster/prod/namespace/default/pod/nginx#viewer@user:frank

# Benefits: Fewer relationships, explicit control
# Drawbacks: Must grant at each level separately
```

## Resource Creation Scenarios

### When Creating a New Pod

**Complex Schema:**
```yaml
# Only need to create these relationships:
resource:io/k8s/core/pods/default/new-app#namespace@namespace:default
resource:io/k8s/core/pods/default/new-app#cluster@cluster:prod

# Alice (cluster admin) can immediately access new-app
# David (namespace viewer) can immediately read new-app
# No additional relationships needed for inherited access
```

**Simple Schema:**
```yaml
# Must explicitly grant each desired access level:
resource:cluster/prod/namespace/default/pod/new-app#owner@user:pod-owner
resource:cluster/prod/namespace/default/pod/new-app#viewer@user:namespace-viewer

# Alice needs explicit grant to access new-app
# No automatic access based on cluster/namespace roles
```

## Use Case Scenarios

### Scenario 1: Organization with Clear Hierarchy

**Requirements:**
- Cluster admins should manage everything
- Namespace owners should control their namespace
- Developers should access only their pods

**Best Choice: Complex Schema**
- Natural mapping to organizational structure
- Automatic permission propagation
- Less relationship management overhead

### Scenario 2: Fine-Grained Control Required

**Requirements:**
- Explicit approval for each resource access
- No automatic inheritance desired
- Clear audit trail of what each user can access

**Best Choice: Simple Schema**
- Explicit permissions only
- No unexpected access through inheritance
- Simple permission model

### Scenario 3: Mixed Environment

**Requirements:**
- Some resources need inheritance (dev environments)
- Some resources need explicit control (production)
- Different teams have different needs

**Best Choice: Complex Schema with Careful Design**
- Use inheritance for dev environments
- Explicit resource-level grants for production
- Leverage both patterns as needed

## Implementation Complexity

### Complex Schema Implementation

**Benefits:**
- Less code to manage relationships
- Natural organizational mapping
- Automatic access for new resources

**Challenges:**
- Complex permission debugging
- Unexpected access through inheritance
- Schema design requires careful planning

**Example Implementation:**
```go
// Only need to link resource to namespace/cluster
func createPod(pod *v1.Pod) {
    resourceID := buildResourceID(pod)
    
    // Resource inherits permissions automatically
    relationships := []*v1.Relationship{
        {Resource: resourceID, Relation: "namespace", Subject: pod.Namespace},
        {Resource: resourceID, Relation: "cluster", Subject: clusterName},
    }
    
    spicedbClient.WriteRelationships(relationships)
}
```

### Simple Schema Implementation

**Benefits:**
- Predictable permissions
- Easy to debug
- Clear permission boundaries

**Challenges:**
- More relationships to manage
- Manual permission propagation
- Verbose relationship creation

**Example Implementation:**
```go
// Must explicitly grant desired access levels
func createPod(pod *v1.Pod, owner string) {
    resourceID := buildResourceID(pod) // "cluster/prod/namespace/default/pod/nginx"
    
    relationships := []*v1.Relationship{
        {Resource: resourceID, Relation: "owner", Subject: owner},
        // Must explicitly add namespace/cluster admins if desired
        {Resource: resourceID, Relation: "viewer", Subject: "namespace-viewer"},
    }
    
    spicedbClient.WriteRelationships(relationships)
}
```

## Performance Considerations

### Complex Schema
- **Query Performance**: More complex permission calculations
- **Relationship Storage**: Fewer relationships overall
- **Cache Efficiency**: Permission inheritance can improve cache hit rates

### Simple Schema  
- **Query Performance**: Direct permission lookups, faster
- **Relationship Storage**: More relationships to store
- **Cache Efficiency**: Simple permissions easier to cache

## Migration Considerations

### From Simple to Complex
- **Difficulty**: High
- **Process**: Need to restructure all relationships and implement inheritance logic
- **Downtime**: Likely required for schema migration

### From Complex to Simple
- **Difficulty**: Medium  
- **Process**: Flatten inherited permissions into explicit relationships
- **Downtime**: Can be done with careful planning

## Recommendations

### Choose Complex Schema When:
- ✅ Clear organizational hierarchy exists
- ✅ Want automatic permission propagation
- ✅ Managing large numbers of resources
- ✅ Trust hierarchical access patterns
- ✅ Team comfortable with complex SpiceDB schemas

### Choose Simple Schema When:
- ✅ Need explicit, granular control
- ✅ Want predictable, debuggable permissions
- ✅ Have compliance requirements for explicit access
- ✅ Prefer simpler implementation and maintenance
- ✅ Team new to SpiceDB

### Hybrid Approach
Consider using both patterns:
- Complex schema for development/staging environments
- Simple schema for production environments
- Different schemas for different resource types

## Conclusion

The choice between complex and simple schemas depends on your organization's needs:

- **Complex schema** provides powerful inheritance but requires careful design and deeper SpiceDB expertise
- **Simple schema** offers predictability and simplicity but requires more manual relationship management

Most organizations should start with the simple schema to understand SpiceDB concepts, then evaluate whether the complexity of inheritance is justified by their specific use cases.