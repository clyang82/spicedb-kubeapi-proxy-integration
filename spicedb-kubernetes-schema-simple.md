```
schema: |-
    definition user {}

    definition resource {
        relation owner: user
        relation viewer: user
        relation editor: user

        permission read = viewer + owner + editor
        permission write = owner + editor
        permission delete = owner
    }

  relationships: |
    // Cluster level access
    resource:cluster/prod#owner@user:alice
    resource:cluster/staging#viewer@user:bob

    // Namespace level access
    resource:cluster/prod/namespace/default#owner@user:charlie
    resource:cluster/prod/namespace/kube-system#editor@user:david

    // Pod level access
    resource:cluster/prod/namespace/default/pod/nginx#owner@user:eve
    resource:cluster/prod/namespace/default/pod/web-server#viewer@user:frank
    resource:cluster/staging/namespace/test/pod/app#editor@user:george

  assertions:
    assertTrue:
      // Cluster owners can access their cluster
      - resource:cluster/prod#read@user:alice
      - resource:cluster/prod#write@user:alice

      // Namespace owners can access their namespace
      - resource:cluster/prod/namespace/default#read@user:charlie
      - resource:cluster/prod/namespace/default#write@user:charlie

      // Pod access
      - resource:cluster/prod/namespace/default/pod/nginx#read@user:eve
      - resource:cluster/prod/namespace/default/pod/nginx#write@user:eve
      - resource:cluster/prod/namespace/default/pod/web-server#read@user:frank

    assertFalse:
      // Users cannot access resources they don't have permissions for
      - resource:cluster/prod/namespace/default/pod/nginx#write@user:frank
      - resource:cluster/staging/namespace/test/pod/app#delete@user:frank

  validation: null
```