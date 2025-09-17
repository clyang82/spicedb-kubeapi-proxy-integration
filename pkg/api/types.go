package api

// API Request types
type CreateNamespaceRequest struct {
	Namespace string `json:"namespace"`
}

type GrantViewPermissionRequest struct {
	Namespace string `json:"namespace"`
	User      string `json:"user"`
}

// API Response type
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}