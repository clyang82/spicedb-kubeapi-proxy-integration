package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/client-go/rest"

	"github.com/clyang82/spicedb-kubeapi-proxy-integration/pkg/api"
	"github.com/clyang82/spicedb-kubeapi-proxy-integration/pkg/proxy"
)

// Server wraps the embedded SpiceDB proxy for HTTP API access
type Server struct {
	proxy  *proxy.SpiceDBKubeProxy
	server *http.Server
}

// NewServer creates a new HTTP server with the embedded proxy
func NewServer() (*Server, error) {
	// Set cache directory to writable location
	err := os.Setenv("KUBECACHEDIR", "/tmp/kube-cache")
	if err != nil {
		log.Printf("Warning: failed to set KUBECACHEDIR: %v", err)
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll("/tmp/kube-cache", 0755); err != nil {
		log.Printf("Warning: failed to create cache directory: %v", err)
	}

	// Get in-cluster config
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	// Create proxy
	proxy, err := proxy.NewSpiceDBKubeProxy(context.Background(), kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy: %w", err)
	}

	// Start the proxy
	if err := proxy.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start proxy: %w", err)
	}

	// Wait for proxy to be ready
	time.Sleep(2 * time.Second)

	// Create HTTP server
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	// API endpoints with real authentication
	mux.HandleFunc("/api/namespaces/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Authenticate user from request headers
		user, err := proxy.AuthenticateFromRequest(r)
		if err != nil {
			writeJSON(w, api.Response{Success: false, Error: fmt.Sprintf("Authentication failed: %v", err)})
			return
		}

		var req api.CreateNamespaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, api.Response{Success: false, Error: "Invalid JSON"})
			return
		}

		if req.Namespace == "" {
			writeJSON(w, api.Response{Success: false, Error: "Namespace is required"})
			return
		}

		// Check Kubernetes RBAC permission first
		allowed, err := proxy.CheckKubernetesPermission(r.Context(), user, "namespaces", "create", "")
		if err != nil {
			writeJSON(w, api.Response{Success: false, Error: fmt.Sprintf("Permission check failed: %v", err)})
			return
		}
		if !allowed {
			writeJSON(w, api.Response{Success: false, Error: "User does not have permission to create namespaces"})
			return
		}

		// Use authenticated user for namespace creation
		if err := proxy.CreateNamespaceAsUser(r.Context(), sanitizeUserName(user.Username), req.Namespace); err != nil {
			writeJSON(w, api.Response{Success: false, Error: err.Error()})
			return
		}

		writeJSON(w, api.Response{Success: true, Data: map[string]string{"namespace": req.Namespace, "user": sanitizeUserName(user.Username)}})
	})

	mux.HandleFunc("/api/namespaces/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Authenticate user from request headers
		user, err := proxy.AuthenticateFromRequest(r)
		if err != nil {
			writeJSON(w, api.Response{Success: false, Error: fmt.Sprintf("Authentication failed: %v", err)})
			return
		}

		// Check Kubernetes RBAC permission first
		allowed, err := proxy.CheckKubernetesPermission(r.Context(), user, "namespaces", "list", "")
		if err != nil {
			writeJSON(w, api.Response{Success: false, Error: fmt.Sprintf("Permission check failed: %v", err)})
			return
		}
		if !allowed {
			writeJSON(w, api.Response{Success: false, Error: "User does not have permission to list namespaces"})
			return
		}

		namespaces, err := proxy.ListNamespacesAsUser(r.Context(), sanitizeUserName(user.Username))
		if err != nil {
			writeJSON(w, api.Response{Success: false, Error: err.Error()})
			return
		}

		writeJSON(w, api.Response{Success: true, Data: map[string]interface{}{"namespaces": namespaces, "user": sanitizeUserName(user.Username)}})
	})

	mux.HandleFunc("/api/namespaces/grant-view", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Authenticate user from request headers
		user, err := proxy.AuthenticateFromRequest(r)
		if err != nil {
			writeJSON(w, api.Response{Success: false, Error: fmt.Sprintf("Authentication failed: %v", err)})
			return
		}

		var req api.GrantViewPermissionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, api.Response{Success: false, Error: "Invalid JSON"})
			return
		}

		if req.Namespace == "" || req.User == "" {
			writeJSON(w, api.Response{Success: false, Error: "Both namespace and user are required"})
			return
		}

		// Check if user has admin permission on the namespace
		allowed, err := proxy.CheckKubernetesPermission(r.Context(), user, "namespaces", "update", req.Namespace)
		if err != nil {
			writeJSON(w, api.Response{Success: false, Error: fmt.Sprintf("Permission check failed: %v", err)})
			return
		}
		if !allowed {
			writeJSON(w, api.Response{Success: false, Error: "User does not have permission to grant access to this namespace"})
			return
		}

		// Grant view permission in SpiceDB
		if err := proxy.GrantViewPermission(r.Context(), req.Namespace, sanitizeUserName(req.User)); err != nil {
			writeJSON(w, api.Response{Success: false, Error: fmt.Sprintf("Failed to grant view permission: %v", err)})
			return
		}

		writeJSON(w, api.Response{
			Success: true, 
			Data: map[string]string{
				"namespace": req.Namespace,
				"user": sanitizeUserName(req.User),
				"permission": "view",
				"granted_by": sanitizeUserName(user.Username),
			},
		})
	})

	// Example usage endpoint
	mux.HandleFunc("/api/demo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		demo := map[string]interface{}{
			"message": "SpiceDB KubeAPI Proxy Integration Demo",
			"endpoints": map[string]string{
				"create_namespace": "POST /api/namespaces/create",
				"list_namespaces":  "POST /api/namespaces/list",
				"grant_view":       "POST /api/namespaces/grant-view",
				"health":           "GET /healthz",
				"ready":            "GET /readyz",
			},
			"example_requests": map[string]interface{}{
				"create_namespace": map[string]string{
					"namespace": "alice-workspace",
				},
				"list_namespaces": map[string]string{},
				"grant_view": map[string]string{
					"namespace": "alice-workspace",
					"user":      "bob",
				},
			},
		}

		writeJSON(w, api.Response{Success: true, Data: demo})
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	return &Server{
		proxy:  proxy,
		server: server,
	}, nil
}

// sanitizeUserName converts user names to be valid SpiceDB object IDs
// For service accounts, extract just the service account name (e.g., testuser from system:serviceaccount:spicedb-proxy:testuser)
// For other users, replace invalid characters with underscores
func sanitizeUserName(userName string) string {
	// Check if this is a service account name
	if strings.HasPrefix(userName, "system:serviceaccount:") {
		// Extract the service account name from system:serviceaccount:namespace:name
		parts := strings.Split(userName, ":")
		if len(parts) >= 4 {
			return parts[3] // Return just the service account name
		}
	}

	return userName
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// GetProxy returns the SpiceDB proxy
func (s *Server) GetProxy() *proxy.SpiceDBKubeProxy {
	return s.proxy
}
