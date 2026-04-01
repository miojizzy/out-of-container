package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/user/exec-server/internal/auditor"
	"github.com/user/exec-server/internal/auth"
	"github.com/user/exec-server/internal/concurrency"
	"github.com/user/exec-server/internal/executor"
	"github.com/user/exec-server/internal/models"
	"github.com/user/exec-server/internal/whitelist"
)

// ExecHandler handles /exec requests
type ExecHandler struct {
	executor  *executor.Executor
	whitelist *whitelist.Checker
	auditor   *auditor.Auditor
	auth      *auth.AuthMiddleware
	limiter   *concurrency.ConcurrencyLimiter
}

// NewExecHandler creates a new exec handler
func NewExecHandler(
	exec *executor.Executor,
	whitelistChecker *whitelist.Checker,
	aud *auditor.Auditor,
	apiToken string,
	maxConcurrent int,
) *ExecHandler {
	return &ExecHandler{
		executor:  exec,
		whitelist: whitelistChecker,
		auditor:   aud,
		auth:      auth.NewAuthMiddleware(apiToken),
		limiter:   concurrency.NewConcurrencyLimiter(maxConcurrent),
	}
}

// ServeHTTP handles the /exec endpoint
func (h *ExecHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply middleware chain: auth -> limiter -> handler
	handler := h.auth.Middleware(h.limiter.Middleware(h.handle))
	handler(w, r)
}

// handle is the actual request handler
func (h *ExecHandler) handle(w http.ResponseWriter, r *http.Request) {
	// Only allow POST
	if r.Method != http.MethodPost {
		executor.ErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST is allowed")
		return
	}

	// Parse request
	var cmd models.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		executor.ErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to parse JSON body")
		return
	}

	// Validate request
	if cmd.Command == "" {
		executor.ErrorResponse(w, http.StatusBadRequest, "invalid_request", "command is required")
		return
	}
	if cmd.Cwd == "" {
		executor.ErrorResponse(w, http.StatusBadRequest, "invalid_request", "cwd is required")
		return
	}

	// Check whitelist
	_, ruleType, err := h.whitelist.IsAllowed(cmd.Command, cmd.Cwd)
	if err != nil {
		if err == whitelist.ErrCommandNotInWhitelist {
			executor.ErrorResponse(w, http.StatusForbidden, "forbidden", "command not in whitelist")
			return
		}
		if err == whitelist.ErrPathNotAllowed {
			executor.ErrorResponse(w, http.StatusForbidden, "forbidden", "cwd not in allowed_paths")
			return
		}
		executor.ErrorResponse(w, http.StatusInternalServerError, "internal_error", "whitelist check failed")
		return
	}

	// Execute command
	result := h.executor.Execute(&cmd)

	// Handle execution errors
	if result.Error != nil {
		if result.HTTPError == http.StatusRequestTimeout {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(result.HTTPError)
			json.NewEncoder(w).Encode(models.ErrorResponse{
				Error:   "timeout",
				Message: result.Error.Error(),
			})
		} else {
			executor.ErrorResponse(w, result.HTTPError, "execution_failed", result.Error.Error())
		}
		return
	}

	// Log audit entry
	if h.auditor != nil {
		token := r.Header.Get("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		tokenPrefix := auth.GetTokenPrefix(token)

		h.auditor.Log(&models.AuditEntry{
			Timestamp:       r.Header.Get("X-Request-Time"),
			Command:         cmd.Command,
			Args:            cmd.Args,
			Cwd:             cmd.Cwd,
			TokenPrefix:     tokenPrefix,
			ExitCode:        result.Result.ExitCode,
			DurationMs:      result.Result.DurationMs,
			OutputSizeBytes: result.Result.OutputSize,
			Truncated:       result.Result.Truncated,
			AllowedBy:       ruleType,
		})
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	if result.Result.Truncated {
		w.Header().Set("X-Output-Truncated", "true")
		w.Header().Set("X-Output-Size-Bytes", strconv.FormatInt(result.Result.OutputSize, 10))
	}

	// Send response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result.Result); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode response: %v\n", err)
	}
}

// HealthHandler handles /health requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write health response: %v\n", err)
	}
}

// getHostname returns hostname for audit logs
// func getHostname() string {
// 	hostname, _ := os.Hostname()
// 	if hostname == "" {
// 		hostname = "unknown"
// 	}
// 	return hostname
// }
