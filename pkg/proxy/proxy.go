package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/spicedb-kubeapi-proxy/pkg/config/proxyrule"
	"github.com/authzed/spicedb-kubeapi-proxy/pkg/proxy"
	"github.com/authzed/spicedb-kubeapi-proxy/pkg/rules"
)

// SpiceDBKubeProxy integrates SpiceDB authorization with Kubernetes API access
type SpiceDBKubeProxy struct {
	proxySrv     *proxy.Server
	kubeClient   *kubernetes.Clientset
	embeddedHTTP *http.Client
}

// NewSpiceDBKubeProxy creates a new proxy component with embedded spicedb-kubeapi-proxy
func NewSpiceDBKubeProxy(ctx context.Context, kubeConfig *rest.Config) (*SpiceDBKubeProxy, error) {
	// Bootstrap content for SpiceDB schema - includes required workflow definitions
	bootstrapContent := map[string][]byte{
		"bootstrap.yaml": []byte(`schema: |-
  use expiration

  definition cluster {}
  definition user {}
  definition namespace {
    relation cluster: cluster
    relation creator: user
    relation viewer: user

    permission admin = creator
    permission edit = creator
    permission view = viewer + creator
    permission no_one_at_all = nil
  }
  definition pod {
    relation namespace: namespace
    relation creator: user
    relation viewer: user
    permission edit = creator
    permission view = viewer + creator
  }
  definition testresource {
    relation namespace: namespace
    relation creator: user
    relation viewer: user
    permission edit = creator
    permission view = viewer + creator
  }
  definition lock {
    relation workflow: workflow
  }
  definition workflow {
    relation idempotency_key: activity with expiration
  }
  definition activity{}
relationships: |
`),
	}

	// Create embedded proxy options
	opts := proxy.NewOptions(proxy.WithEmbeddedProxy, proxy.WithEmbeddedSpiceDBBootstrap(bootstrapContent))
	
	// Set workflow database to a unique path to avoid conflicts
	opts.WorkflowDatabasePath = fmt.Sprintf("/tmp/proxy-workflow-%d.sqlite", time.Now().UnixNano())

	// Configure backend Kubernetes cluster
	opts.RestConfigFunc = func() (*rest.Config, http.RoundTripper, error) {
		transport, err := rest.TransportFor(kubeConfig)
		if err != nil {
			return nil, nil, err
		}
		configCopy := rest.CopyConfig(kubeConfig)
		return configCopy, transport, nil
	}

	// Define authorization rules
	ruleConfigs := []proxyrule.Config{
		{
			Spec: proxyrule.Spec{
				Matches: []proxyrule.Match{{
					GroupVersion: "v1",
					Resource:     "namespaces",
					Verbs:        []string{"create"},
				}},
				Update: proxyrule.Update{
					CreateRelationships: []proxyrule.StringOrTemplate{{
						Template: "namespace:{{name}}#creator@user:{{user.name}}",
					}},
				},
			},
		},
		{
			Spec: proxyrule.Spec{
				Matches: []proxyrule.Match{{
					GroupVersion: "v1",
					Resource:     "namespaces",
					Verbs:        []string{"get"},
				}},
				Checks: []proxyrule.StringOrTemplate{{
					Template: "namespace:{{name}}#view@user:{{user.name}}",
				}},
			},
		},
		{
			Spec: proxyrule.Spec{
				Matches: []proxyrule.Match{{
					GroupVersion: "v1",
					Resource:     "namespaces",
					Verbs:        []string{"list"},
				}},
				PreFilters: []proxyrule.PreFilter{{
					FromObjectIDNameExpr:    "{{resourceId}}",
					LookupMatchingResources: &proxyrule.StringOrTemplate{Template: "namespace:$#view@user:{{user.name}}"},
				}},
			},
		},
		{
			Spec: proxyrule.Spec{
				Matches: []proxyrule.Match{{
					GroupVersion: "v1",
					Resource:     "pods",
					Verbs:        []string{"create"},
				}},
				Update: proxyrule.Update{
					CreateRelationships: []proxyrule.StringOrTemplate{{
						Template: "pod:{{name}}#creator@user:{{user.name}}",
					}, {
						Template: "pod:{{name}}#namespace@namespace:{{namespace}}",
					}},
				},
			},
		},
		{
			Spec: proxyrule.Spec{
				Matches: []proxyrule.Match{{
					GroupVersion: "v1",
					Resource:     "pods",
					Verbs:        []string{"get", "list", "delete"},
				}},
				Checks: []proxyrule.StringOrTemplate{{
					Template: "pod:{{name}}#edit@user:{{user.name}}",
				}},
			},
		},
	}

	matcher, err := rules.NewMapMatcher(ruleConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to create rule matcher: %w", err)
	}
	opts.Matcher = matcher

	// Complete configuration
	completedConfig, err := opts.Complete(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to complete proxy configuration: %w", err)
	}

	// Create proxy server
	proxySrv, err := proxy.NewServer(ctx, completedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy server: %w", err)
	}

	return &SpiceDBKubeProxy{
		proxySrv: proxySrv,
	}, nil
}

// Start starts the embedded proxy server
func (c *SpiceDBKubeProxy) Start(ctx context.Context) error {
	// Start proxy server in background
	go func() {
		if err := c.proxySrv.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("Proxy server error: %v", err)
		}
	}()

	return nil
}

// GetKubernetesClientForUser returns a Kubernetes client for a specific user
func (c *SpiceDBKubeProxy) GetKubernetesClientForUser(username string, groups ...string) (*kubernetes.Clientset, error) {
	embeddedHTTP := c.proxySrv.GetEmbeddedClient(
		proxy.WithUser(username),
		proxy.WithGroups(groups...),
	)

	kubeClient, err := kubernetes.NewForConfigAndClient(proxy.EmbeddedRestConfig, embeddedHTTP)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return kubeClient, nil
}

// CreateNamespaceAsUser creates a namespace as a specific user
func (c *SpiceDBKubeProxy) CreateNamespaceAsUser(ctx context.Context, username, namespace string) error {
	client, err := c.GetKubernetesClientForUser(username, "users")
	if err != nil {
		return err
	}

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	_, err = client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	return err
}

// ListNamespacesAsUser lists namespaces that a user has access to
func (c *SpiceDBKubeProxy) ListNamespacesAsUser(ctx context.Context, username string) ([]string, error) {
	client, err := c.GetKubernetesClientForUser(username, "users")
	if err != nil {
		return nil, err
	}

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var names []string
	for _, ns := range namespaces.Items {
		names = append(names, ns.Name)
	}
	return names, nil
}

// GetSpiceDBClient returns the SpiceDB permissions client from the embedded proxy
func (c *SpiceDBKubeProxy) GetSpiceDBClient() v1.PermissionsServiceClient {
	return c.proxySrv.PermissionClient()
}

// StartSpiceDBDataPrinter starts a goroutine that periodically prints SpiceDB data
func (c *SpiceDBKubeProxy) StartSpiceDBDataPrinter(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // Print every 30 seconds
		defer ticker.Stop()

		log.Println("Starting SpiceDB data printer goroutine...")

		for {
			select {
			case <-ctx.Done():
				log.Println("SpiceDB data printer stopping...")
				return
			case <-ticker.C:
				c.printSpiceDBData(ctx)
			}
		}
	}()
}

// printSpiceDBData queries and prints current SpiceDB relationships
func (c *SpiceDBKubeProxy) printSpiceDBData(ctx context.Context) {
	client := c.GetSpiceDBClient()
	if client == nil {
		log.Println("SpiceDB client not available")
		return
	}

	log.Println("=== SpiceDB Data Snapshot ===")

	// Read relationships - we'll read a sample to see what's in the system
	relResp, err := client.ReadRelationships(ctx, &v1.ReadRelationshipsRequest{
		OptionalLimit: 100, // Limit to avoid too much output
	})
	if err != nil {
		log.Printf("Error reading relationships: %v", err)
		return
	}

	log.Println("Current Relationships:")
	relationshipCount := 0
	for {
		msg, err := relResp.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error receiving relationship: %v", err)
			break
		}
		
		rel := msg.Relationship
		log.Printf("  %s:%s#%s@%s:%s", 
			rel.Resource.ObjectType, 
			rel.Resource.ObjectId,
			rel.Relation,
			rel.Subject.Object.ObjectType,
			rel.Subject.Object.ObjectId)
		relationshipCount++
	}
	
	log.Printf("Total relationships found: %d", relationshipCount)
	log.Println("=== End SpiceDB Data Snapshot ===")
}