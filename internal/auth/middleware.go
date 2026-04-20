// Package auth 提供HTTP认证中间件。
//
// 实现了基于Bearer Token的认证机制，用于验证API请求。
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

// Middleware validates API token from Authorization header
type Middleware struct {
	apiToken string
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(apiToken string) *Middleware {
	return &Middleware{apiToken: apiToken}
}

// Middleware returns the HTTP middleware function
func (a *Middleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
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
func (a *Middleware) unauthorized(w http.ResponseWriter, message string) {
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
