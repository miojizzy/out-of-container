package auditor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/exec-server/internal/models"
)

func TestAuditor_Log(t *testing.T) {
	// Create a temporary log file
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit.log")

	// Create auditor
	auditor := NewAuditor(logFile, 10, 3)
	defer auditor.Close()

	// Create a test audit entry
	entry := &models.AuditEntry{
		Timestamp:       time.Now().Format(time.RFC3339),
		Command:         "echo",
		Args:            []string{"hello"},
		Cwd:             "/tmp",
		TokenPrefix:     "test1234",
		ExitCode:        0,
		DurationMs:      100,
		OutputSizeBytes: 6,
		Truncated:       false,
		AllowedBy:       "literal",
	}

	// Log the entry
	auditor.Log(entry)

	// Close the auditor to ensure all entries are written
	if err := auditor.Close(); err != nil {
		t.Fatalf("Failed to close auditor: %v", err)
	}

	// Read the log file
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	// Parse the log entry
	var loggedEntry models.AuditEntry
	if err := json.Unmarshal(data, &loggedEntry); err != nil {
		t.Fatalf("Failed to parse log entry: %v", err)
	}

	// Verify the entry
	if loggedEntry.Command != entry.Command {
		t.Errorf("Command = %v, want %v", loggedEntry.Command, entry.Command)
	}
	if loggedEntry.Cwd != entry.Cwd {
		t.Errorf("Cwd = %v, want %v", loggedEntry.Cwd, entry.Cwd)
	}
	if loggedEntry.TokenPrefix != entry.TokenPrefix {
		t.Errorf("TokenPrefix = %v, want %v", loggedEntry.TokenPrefix, entry.TokenPrefix)
	}
	if loggedEntry.ExitCode != entry.ExitCode {
		t.Errorf("ExitCode = %v, want %v", loggedEntry.ExitCode, entry.ExitCode)
	}
	// Check that timestamp is not empty
	if loggedEntry.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

func TestAuditor_RotateLog(t *testing.T) {
	// Create a temporary log file
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "audit.log")

	// Create auditor with small rotation size
	auditor := NewAuditor(logFile, 1, 3) // 1MB for testing
	defer auditor.Close()

	// Create a test audit entry with large output to trigger rotation
	entry := &models.AuditEntry{
		Timestamp:       time.Now().Format(time.RFC3339),
		Command:         "echo",
		Args:            []string{"hello"},
		Cwd:             "/tmp",
		TokenPrefix:     "test1234",
		ExitCode:        0,
		DurationMs:      100,
		OutputSizeBytes: 2048 * 1024, // 2MB
		Truncated:       false,
		AllowedBy:       "literal",
	}

	// Log the entry
	auditor.Log(entry)

	// Close the auditor
	if err := auditor.Close(); err != nil {
		t.Fatalf("Failed to close auditor: %v", err)
	}

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file should exist")
	}
}
