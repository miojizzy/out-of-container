package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/exec-server/internal/auditor"
	"github.com/user/exec-server/internal/executor"
	"github.com/user/exec-server/internal/models"
	"github.com/user/exec-server/internal/whitelist"
)

func TestExecHandler_AuditLog(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit.log")

	// Create auditor
	aud := auditor.NewAuditor(logFile, 10, 3)
	defer func() {
		if err := aud.Close(); err != nil {
			t.Logf("Warning: failed to close auditor: %v", err)
		}
	}()

	// Create a temporary config file for whitelist
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
whitelist:
  literal_commands:
    - "echo"
  allowed_paths:
    - "/tmp"
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create whitelist checker
	whitelistChecker, err := whitelist.NewChecker(configFile)
	if err != nil {
		t.Fatalf("Failed to create whitelist checker: %v", err)
	}

	// Create executor
	exec := executor.NewExecutor(30, 10)

	// Create handler
	handler := NewExecHandler(exec, whitelistChecker, aud, "test-token", 5)

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Create a command request
	cmd := &models.Command{
		Command: "echo",
		Args:    []string{"hello world"},
		Cwd:     "/tmp",
	}
	cmdJSON, _ := json.Marshal(cmd)

	// Send request
	req, _ := http.NewRequest("POST", server.URL+"/ooc-exec", bytes.NewBuffer(cmdJSON))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Warning: failed to close response body: %v", err)
		}
	}()

	// Check response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK; got %v", resp.Status)
	}

	// Read response body
	body, _ := io.ReadAll(resp.Body)
	t.Logf("Response body: %s", string(body))

	// Close auditor to ensure logs are written
	if err := aud.Close(); err != nil {
		t.Fatalf("Failed to close auditor: %v", err)
	}

	// Wait a bit for file to be written
	time.Sleep(100 * time.Millisecond)

	// Read the log file
	logData, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Check that log entry exists and has timestamp
	logLines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(logLines) == 0 {
		t.Fatal("No log entries found")
	}

	// Parse the first log entry
	var auditEntry models.AuditEntry
	if err := json.Unmarshal([]byte(logLines[0]), &auditEntry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Verify that timestamp is not empty
	if auditEntry.Timestamp == "" {
		t.Error("Audit log timestamp should not be empty")
	}

	// Verify that timestamp is in RFC3339 format
	_, err = time.Parse(time.RFC3339, auditEntry.Timestamp)
	if err != nil {
		t.Errorf("Audit log timestamp should be in RFC3339 format: %v", err)
	}

	// Verify other fields
	if auditEntry.Command != "echo" {
		t.Errorf("Expected command 'echo'; got %v", auditEntry.Command)
	}
	if auditEntry.Cwd != "/tmp" {
		t.Errorf("Expected cwd '/tmp'; got %v", auditEntry.Cwd)
	}
}
