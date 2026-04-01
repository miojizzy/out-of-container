package concurrency

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/user/exec-server/internal/models"
)

// ConcurrencyLimiter controls maximum concurrent executions
type ConcurrencyLimiter struct {
	sem chan struct{}
	max int
}

// NewConcurrencyLimiter creates a new limiter with max concurrent executions
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		sem: make(chan struct{}, max),
		max: max,
	}
}

// Middleware returns HTTP middleware that limits concurrent requests
func (l *ConcurrencyLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to acquire semaphore
		select {
		case l.sem <- struct{}{}:
			// Acquired, execute and release
			defer func() { <-l.sem }()
			next(w, r)
		default:
			// Semaphore full, reject request
			l.serviceUnavailable(w)
		}
	}
}

// serviceUnavailable sends a 503 error response
func (l *ConcurrencyLimiter) serviceUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	if err := json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   "service_unavailable",
		Message: "maximum concurrent executions reached",
	}); err != nil {
		// Log error but don't fail the request
		fmt.Fprintf(os.Stderr, "Failed to encode error response: %v\n", err)
	}
}

// Current returns current number of active executions
func (l *ConcurrencyLimiter) Current() int {
	return len(l.sem)
}

// Max returns maximum allowed concurrent executions
func (l *ConcurrencyLimiter) Max() int {
	return l.max
}
