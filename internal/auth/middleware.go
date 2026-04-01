package auth

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/user/exec-server/internal/models"
)

// AuthMiddleware validates API token from Authorization header
type AuthMiddleware struct {
	apiToken string
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(apiToken string) *AuthMiddleware {
	return &AuthMiddleware{apiToken: apiToken}
}

// Middleware returns the HTTP middleware function
func (a *AuthMiddleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			a.unauthorized(w, "missing Authorization header")
			return
		}

		// Expected format: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			a.unauthorized(w, "invalid Authorization header format")
			return
		}

		token := parts[1]

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(a.apiToken)) != 1 {
			a.unauthorized(w, "invalid API token")
			return
		}

		// Token is valid, proceed to next handler
		next(w, r)
	}
}

// unauthorized sends a 401 error response
func (a *AuthMiddleware) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	if err := json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   "unauthorized",
		Message: message,
	}); err != nil {
		// Log error but don't fail the request
		fmt.Fprintf(os.Stderr, "Failed to encode unauthorized error response: %v\n", err)
	}
}

// GetTokenPrefix returns the first 8 characters of token for logging
func GetTokenPrefix(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8]
}
