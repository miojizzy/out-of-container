package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	validToken := "test-token-123"
	middleware := NewMiddleware(validToken)

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"valid token", validToken, http.StatusOK},
		{"invalid token", "wrong-token", http.StatusUnauthorized},
		{"empty token", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/ooc-exec", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			rr := httptest.NewRecorder()
			middleware.Middleware(handler).ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantStatus)
			}
		})
	}
}

func TestGetTokenPrefix(t *testing.T) {
	tests := []struct {
		token  string
		prefix string
	}{
		{"abcd1234efgh5678", "abcd1234"},
		{"short", "short"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			got := GetTokenPrefix(tt.token)
			if got != tt.prefix {
				t.Errorf("GetTokenPrefix(%q) = %q, want %q", tt.token, got, tt.prefix)
			}
		})
	}
}
