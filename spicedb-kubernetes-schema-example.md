```
schema: |-
    definition user {}

    definition cluster {
        relation admin: user
        relation viewer: user
        permission manage = admin
        permission view = viewer + admin
    }

    definition namespace {
        relation cluster: cluster
        relation admin: user
        relation viewer: user
        relation editor: user

        // Inherit cluster permissions and add namespace-specific ones
        permission manage = admin + cluster->manage
        permission edit = editor + admin + cluster->manage
        permission view = viewer + editor + admin + cluster->view
    }

    definition resource {
        relation namespace: namespace
        relation cluster: cluster
        relation owner: user
        relation viewer: user
        relation editor: user

        // Resources inherit from both namespace and cluster
        permission read = viewer + owner + editor + namespace->view + cluster->view
        permission write = owner + editor + namespace->edit + cluster->manage
        permission delete = owner + namespace->manage + cluster->manage
    }

    definition subresource {
        relation parent: resource
        relation namespace: namespace
        relation cluster: cluster
        relation allowed: user

        // Subresources inherit from parent resource and namespace/cluster
        permission access = allowed + parent->read + namespace->view + cluster->view
        permission execute = allowed + parent->write + namespace->edit + cluster->manage
    }

  relationships: |
    // Cluster level
    cluster:prod#admin@user:alice
    cluster:prod#viewer@user:bob

    // Namespace level
    namespace:default#cluster@cluster:prod
    namespace:default#admin@user:charlie
    namespace:default#viewer@user:david
    namespace:default#editor@user:eve

    namespace:kube-system#cluster@cluster:prod
    namespace:kube-system#admin@user:alice

    // Resource level - pods
    resource:io/k8s/core/pods/default/nginx#namespace@namespace:default
    resource:io/k8s/core/pods/default/nginx#cluster@cluster:prod
    resource:io/k8s/core/pods/default/nginx#owner@user:frank

    resource:io/k8s/core/pods/default/web-server#namespace@namespace:default
    resource:io/k8s/core/pods/default/web-server#cluster@cluster:prod
    resource:io/k8s/core/pods/default/web-server#editor@user:eve

    // Resource level - services
    resource:io/k8s/core/services/default/nginx-svc#namespace@namespace:default
    resource:io/k8s/core/services/default/nginx-svc#cluster@cluster:prod
    resource:io/k8s/core/services/default/nginx-svc#owner@user:frank

    // Resource level - secrets (sensitive)
    resource:io/k8s/core/secrets/default/app-secret#namespace@namespace:default
    resource:io/k8s/core/secrets/default/app-secret#cluster@cluster:prod
    resource:io/k8s/core/secrets/default/app-secret#viewer@user:charlie

    // Subresource level - pod/exec
    subresource:io/k8s/core/pods/exec/default/nginx#parent@resource:io/k8s/core/pods/default/nginx
    subresource:io/k8s/core/pods/exec/default/nginx#namespace@namespace:default
    subresource:io/k8s/core/pods/exec/default/nginx#cluster@cluster:prod
    subresource:io/k8s/core/pods/exec/default/nginx#allowed@user:david

    // Subresource level - pod/logs
    subresource:io/k8s/core/pods/logs/default/nginx#parent@resource:io/k8s/core/pods/default/nginx
    subresource:io/k8s/core/pods/logs/default/nginx#namespace@namespace:default
    subresource:io/k8s/core/pods/logs/default/nginx#cluster@cluster:prod

    // Subresource level - service/proxy
    subresource:io/k8s/core/services/proxy/default/nginx-svc#parent@resource:io/k8s/core/services/default/nginx-svc
    subresource:io/k8s/core/services/proxy/default/nginx-svc#namespace@namespace:default
    subresource:io/k8s/core/services/proxy/default/nginx-svc#cluster@cluster:prod
    subresource:io/k8s/core/services/proxy/default/nginx-svc#allowed@user:eve

  assertions:
    assertTrue:
      // Cluster admin can do everything
      - cluster:prod#manage@user:alice
      - namespace:default#manage@user:alice
      - resource:io/k8s/core/pods/default/nginx#read@user:alice
      - resource:io/k8s/core/pods/default/nginx#write@user:alice
      - subresource:io/k8s/core/pods/exec/default/nginx#execute@user:alice

      // Cluster viewer can view everything
      - cluster:prod#view@user:bob
      - namespace:default#view@user:bob
      - resource:io/k8s/core/pods/default/nginx#read@user:bob
      - subresource:io/k8s/core/pods/logs/default/nginx#access@user:bob

      // Namespace admin can manage namespace resources
      - namespace:default#manage@user:charlie
      - resource:io/k8s/core/pods/default/nginx#read@user:charlie
      - resource:io/k8s/core/pods/default/nginx#write@user:charlie
      - resource:io/k8s/core/pods/default/nginx#delete@user:charlie

      // Namespace viewer can view namespace resources
      - namespace:default#view@user:david
      - resource:io/k8s/core/pods/default/nginx#read@user:david
      - subresource:io/k8s/core/pods/exec/default/nginx#access@user:david

      // Namespace editor can edit namespace resources
      - namespace:default#edit@user:eve
      - resource:io/k8s/core/pods/default/web-server#write@user:eve
      - subresource:io/k8s/core/services/proxy/default/nginx-svc#access@user:eve

      // Resource owner has full control over their resources
      - resource:io/k8s/core/pods/default/nginx#read@user:frank
      - resource:io/k8s/core/pods/default/nginx#write@user:frank
      - resource:io/k8s/core/services/default/nginx-svc#read@user:frank

    assertFalse:
      // Cluster viewer cannot write
      - resource:io/k8s/core/pods/default/nginx#write@user:bob
      - resource:io/k8s/core/pods/default/nginx#delete@user:bob

      // Namespace viewer cannot write
      - resource:io/k8s/core/pods/default/nginx#write@user:david
      - resource:io/k8s/core/pods/default/nginx#delete@user:david

      // Users cannot access resources outside their permissions
      - resource:io/k8s/core/secrets/default/app-secret#read@user:david
      - resource:io/k8s/core/secrets/default/app-secret#read@user:eve

  validation: null
```