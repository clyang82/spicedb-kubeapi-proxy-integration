package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	authv1 "k8s.io/api/authorization/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// UserInfo represents authenticated user information
type UserInfo struct {
	Username string
	Groups   []string
	UID      string
}

// AuthenticationResult contains auth result and user info
type AuthenticationResult struct {
	Authenticated bool
	User          *UserInfo
	Error         error
}

// Authenticator handles different authentication methods
type Authenticator struct {
	kubeClient kubernetes.Interface
}

// NewAuthenticator creates a new authenticator with Kubernetes client
func NewAuthenticator(kubeConfig *rest.Config) (*Authenticator, error) {
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Authenticator{
		kubeClient: kubeClient,
	}, nil
}

// AuthenticateRequest extracts and validates user from HTTP request
func (a *Authenticator) AuthenticateRequest(r *http.Request) *AuthenticationResult {
	// Try different authentication methods in order of preference
	
	// 1. Try Bearer token authentication
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			return a.authenticateToken(r.Context(), token)
		}
	}
	
	// 2. Try client certificate authentication
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		return a.authenticateCertificate(r)
	}
	
	// 3. Try custom headers (for testing/development)
	if username := r.Header.Get("X-Remote-User"); username != "" {
		groups := strings.Split(r.Header.Get("X-Remote-Groups"), ",")
		return &AuthenticationResult{
			Authenticated: true,
			User: &UserInfo{
				Username: username,
				Groups:   groups,
				UID:      username, // Use username as UID for header auth
			},
		}
	}
	
	return &AuthenticationResult{
		Authenticated: false,
		Error:         fmt.Errorf("no valid authentication method found"),
	}
}

// authenticateToken validates a bearer token using TokenReview
func (a *Authenticator) authenticateToken(ctx context.Context, token string) *AuthenticationResult {
	// Use Kubernetes TokenReview to validate the token
	tokenReview := &authenticationv1.TokenReview{
		Spec: authenticationv1.TokenReviewSpec{
			Token: token,
		},
	}
	
	result, err := a.kubeClient.AuthenticationV1().TokenReviews().Create(ctx, tokenReview, metav1.CreateOptions{})
	if err != nil {
		return &AuthenticationResult{
			Authenticated: false,
			Error:         fmt.Errorf("token review failed: %w", err),
		}
	}
	
	if !result.Status.Authenticated {
		return &AuthenticationResult{
			Authenticated: false,
			Error:         fmt.Errorf("token authentication failed: %s", result.Status.Error),
		}
	}
	
	return &AuthenticationResult{
		Authenticated: true,
		User: &UserInfo{
			Username: result.Status.User.Username,
			Groups:   result.Status.User.Groups,
			UID:      result.Status.User.UID,
		},
	}
}

// authenticateCertificate extracts user info from client certificate
func (a *Authenticator) authenticateCertificate(r *http.Request) *AuthenticationResult {
	cert := r.TLS.PeerCertificates[0]
	
	// Extract username from certificate Common Name
	username := cert.Subject.CommonName
	if username == "" {
		return &AuthenticationResult{
			Authenticated: false,
			Error:         fmt.Errorf("no common name in client certificate"),
		}
	}
	
	// Extract groups from certificate Organization fields
	groups := cert.Subject.Organization
	
	return &AuthenticationResult{
		Authenticated: true,
		User: &UserInfo{
			Username: username,
			Groups:   groups,
			UID:      username, // Use CN as UID for cert auth
		},
	}
}

// CheckKubernetesPermission checks if user has permission for a specific Kubernetes action
func (a *Authenticator) CheckKubernetesPermission(ctx context.Context, user *UserInfo, resource, verb, namespace string) (bool, error) {
	// Use SubjectAccessReview to check permissions
	sar := &authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User:   user.Username,
			Groups: user.Groups,
			UID:    user.UID,
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:      verb,
				Resource:  resource,
				Namespace: namespace,
			},
		},
	}
	
	result, err := a.kubeClient.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, fmt.Errorf("subject access review failed: %w", err)
	}
	
	return result.Status.Allowed, nil
}

// AuthMiddleware is HTTP middleware that adds authentication to requests
func (a *Authenticator) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authResult := a.AuthenticateRequest(r)
		
		if !authResult.Authenticated {
			http.Error(w, fmt.Sprintf("Authentication failed: %v", authResult.Error), http.StatusUnauthorized)
			return
		}
		
		// Add user info to request context
		ctx := context.WithValue(r.Context(), "user", authResult.User)
		r = r.WithContext(ctx)
		
		next(w, r)
	}
}

// GetUserFromContext extracts UserInfo from request context
func GetUserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value("user").(*UserInfo)
	return user, ok
}