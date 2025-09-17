# SpiceDB Kubeapi Proxy Sequence Diagram

```mermaid
sequenceDiagram
    participant C as Client (kubectl)
    participant P as SpiceDB Proxy
    participant S as SpiceDB
    participant K as Kubernetes API Server
    participant W as Workflow Engine

    Note over C,W: Request Authorization Flow

    C->>P: HTTP Request (e.g., GET /api/v1/pods)
    
    Note over P: Authentication
    P->>P: Extract user info from headers/certs
    
    Note over P: Rule Matching
    P->>P: Match request to configured rules
    P->>P: Extract RequestMeta (verb, group, version, resource)
    
    Note over P: CEL Condition Evaluation
    P->>P: Evaluate CEL if conditions
    
    Note over P: Authorization Checks
    P->>S: CheckPermission Request
    Note right of S: e.g., pod:default/nginx#view@user:alice
    S-->>P: Permission Result
    
    alt Permission Denied
        P-->>C: 403 Forbidden
    else Permission Granted
        
        alt Write Operation (create/update/delete)
            Note over P,W: Distributed Transaction
            P->>W: Start Workflow
            W->>S: WriteRelationships (preconditions)
            S-->>W: Precondition Result
            alt Preconditions Failed
                W-->>P: Transaction Failed
                P-->>C: 400 Bad Request
            else Preconditions Pass
                W->>K: Forward Request to Kubernetes
                K-->>W: Kubernetes Response
                alt K8s Success
                    W->>S: WriteRelationships (creates/deletes)
                    S-->>W: Write Result
                    W-->>P: Transaction Success
                else K8s Failure
                    W->>S: Cleanup/Rollback
                    W-->>P: Transaction Failed
                end
            end
            
        else Read Operation (get/list/watch)
            Note over P: Pre-filtering
            alt List/Watch with PreFilter
                P->>S: LookupResources
                Note right of S: Find resources user can access
                S-->>P: Allowed Resource IDs
                P->>P: Build name/namespace filters
            end
            
            P->>K: Forward Request (with filters)
            K-->>P: Kubernetes Response
            
            Note over P: Post-filtering
            alt List Operation with PostFilter
                loop For each item in response
                    P->>S: CheckPermission per item
                    S-->>P: Item Permission Result
                end
                P->>P: Filter out unauthorized items
            end
            
            alt Watch Operation
                Note over P: Streaming Response
                P->>P: Start response filterer
                loop For each watch event
                    P->>S: CheckPermission for event object
                    S-->>P: Event Permission Result
                    alt Authorized
                        P-->>C: Forward Watch Event
                    else Unauthorized
                        Note over P: Filter out event
                    end
                end
            end
        end
        
        P-->>C: Filtered Response
    end

    Note over C,W: Key Components:
    Note over P: • Authentication (headers/certs)
    Note over P: • Rule matching (RequestMeta)
    Note over P: • CEL condition evaluation
    Note over S: • Relationship-based authorization
    Note over W: • Distributed transactions
    Note over P: • Response filtering
```

## Flow Explanation

1. **Authentication**: Extract user identity from request headers or certificates
2. **Rule Matching**: Find matching authorization rules based on API verb, group, version, and resource
3. **CEL Evaluation**: Apply conditional logic using CEL expressions if defined
4. **Authorization**: Check permissions in SpiceDB using relationship queries
5. **Transaction Handling**: For write operations, use distributed transactions to ensure consistency
6. **Filtering**: Apply pre-filters and post-filters to limit response data based on permissions
7. **Response**: Return filtered results to the client

## Rule Types

- **Check**: Verify user has permission (e.g., `pod:nginx#view@user:alice`)
- **PreFilter**: Limit query scope using SpiceDB lookups (for list/watch)
- **PostFilter**: Filter individual items in responses (for list operations)
- **Update**: Manage SpiceDB relationships during resource lifecycle