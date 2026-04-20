// Package concurrency 提供并发控制功能。
//
// 实现了基于信号量的并发限制器，用于控制同时执行的请求数量。
package concurrency

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/user/exec-server/internal/models"
)

// Limiter controls maximum concurrent executions
type Limiter struct {
	sem chan struct{}
	max int
}

// NewLimiter creates a new limiter with max concurrent executions
func NewLimiter(maxConcurrent int) *Limiter {
	return &Limiter{
		sem: make(chan struct{}, maxConcurrent),
		max: maxConcurrent,
	}
}

// Middleware returns HTTP middleware that limits concurrent requests
func (l *Limiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
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
func (l *Limiter) serviceUnavailable(w http.ResponseWriter) {
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
func (l *Limiter) Current() int {
	return len(l.sem)
}

// Max returns maximum allowed concurrent executions
func (l *Limiter) Max() int {
	return l.max
}
