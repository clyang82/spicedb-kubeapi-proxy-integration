package api

// API Request types
type CreateNamespaceRequest struct {
	Username  string `json:"username"`
	Namespace string `json:"namespace"`
}

type ListNamespacesRequest struct {
	Username string `json:"username"`
}

// API Response type
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}