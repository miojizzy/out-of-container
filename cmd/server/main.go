package main

import (
	"log"
	"net/http"
	"os"

	"github.com/user/exec-server/internal/auth"
	"github.com/user/exec-server/internal/concurrency"
	"github.com/user/exec-server/internal/executor"
)

func main() {
	// Simple server startup
	// TODO: Load config from file
	apiToken := os.Getenv("API_TOKEN")
	if apiToken == "" {
		apiToken = "test-token-change-me"
	}

	authMiddleware := auth.NewAuthMiddleware(apiToken)
	limiter := concurrency.NewConcurrencyLimiter(5)
	exec := executor.NewExecutor(30, 10)

	// Simple handler
	handler := func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement full handler
		w.Write([]byte("OK"))
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/exec", authMiddleware.Middleware(limiter.Middleware(handler)))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
