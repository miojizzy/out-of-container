package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/user/exec-server/internal/models"
)

// ErrorResponse creates a JSON error response
func ErrorResponse(w http.ResponseWriter, statusCode int, errMsg, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   errMsg,
		Message: message,
	}); err != nil {
		// Log error but don't fail the request
		fmt.Fprintf(os.Stderr, "Failed to encode error response: %v\n", err)
	}
}
