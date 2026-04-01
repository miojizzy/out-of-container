package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"syscall"
	"time"

	"github.com/user/exec-server/internal/models"
	"github.com/user/exec-server/internal/validation"
)

// Executor handles command execution
type Executor struct {
	timeout       time.Duration
	maxOutputBytes int64
}

// NewExecutor creates a new executor
func NewExecutor(timeoutSeconds int, maxOutputMB int64) *Executor {
	return &Executor{
		timeout:        time.Duration(timeoutSeconds) * time.Second,
		maxOutputBytes: maxOutputMB * 1024 * 1024,
	}
}

// ExecuteResult contains execution results
type ExecuteResult struct {
	Result    *models.Result
	Error     error
	HTTPError int // HTTP status code if error
}

// Execute runs the command and returns result
func (e *Executor) Execute(cmd *models.Command) *ExecuteResult {
	startTime := time.Now()

	// Validate command safety
	if err := validation.CheckCommandSafety(cmd.Command); err != nil {
		return &ExecuteResult{
			Error:     err,
			HTTPError: http.StatusBadRequest,
		}
	}

	// Validate args safety
	if err := validation.CheckArgsSafety(cmd.Args); err != nil {
		return &ExecuteResult{
			Error:     err,
			HTTPError: http.StatusBadRequest,
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	// Create command with context
	execCmd := exec.CommandContext(ctx, cmd.Command, cmd.Args...)
	execCmd.Dir = cmd.Cwd

	// Set process group for cleanup
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Setup output capture with size limiting
	stdoutBuf := &LimitedBuffer{maxBytes: e.maxOutputBytes}
	stderrBuf := &LimitedBuffer{maxBytes: e.maxOutputBytes}
	sharedCounter := &SharedCounter{maxBytes: e.maxOutputBytes}

	stdoutBuf.SetSharedCounter(sharedCounter)
	stderrBuf.SetSharedCounter(sharedCounter)

	execCmd.Stdout = stdoutBuf
	execCmd.Stderr = stderrBuf

	// Start command
	err := execCmd.Start()
	if err != nil {
		return &ExecuteResult{
			Result: &models.Result{
				ExitCode:   -1,
				Stdout:     "",
				Stderr:     err.Error(),
				DurationMs: time.Since(startTime).Milliseconds(),
			},
			Error:     fmt.Errorf("failed to start command: %w", err),
			HTTPError: http.StatusInternalServerError,
		}
	}

	// Wait for command completion
	err = execCmd.Wait()

	duration := time.Since(startTime).Milliseconds()

	// Check for timeout
	if ctx.Err() == context.DeadlineExceeded {
		// Kill process group
		syscall.Kill(-execCmd.Process.Pid, syscall.SIGKILL)
		// Wait again to reap zombie
		execCmd.Wait()

		return &ExecuteResult{
			Result: &models.Result{
				ExitCode:   -1,
				Stdout:     stdoutBuf.String(),
				Stderr:     stderrBuf.String(),
				DurationMs: duration,
				Truncated:  stdoutBuf.IsTruncated() || stderrBuf.IsTruncated(),
			},
			Error:     fmt.Errorf("command exceeded %v timeout", e.timeout),
			HTTPError: http.StatusRequestTimeout,
		}
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	truncated := stdoutBuf.IsTruncated() || stderrBuf.IsTruncated()
	outputSize := stdoutBuf.BytesWritten() + stderrBuf.BytesWritten()

	return &ExecuteResult{
		Result: &models.Result{
			ExitCode:    exitCode,
			Stdout:      stdoutBuf.String(),
			Stderr:      stderrBuf.String(),
			DurationMs:  duration,
			Truncated:   truncated,
			OutputSize:  outputSize,
		},
	}
}

// LimitedBuffer is a size-limited buffer with shared counter
type LimitedBuffer struct {
	bytes.Buffer
	maxBytes      int64
	bytesWritten  int64
	truncated     bool
	sharedCounter *SharedCounter
}

// SetSharedCounter sets the shared counter for combined output limiting
func (b *LimitedBuffer) SetSharedCounter(counter *SharedCounter) {
	b.sharedCounter = counter
}

// Write implements io.Writer with size limiting
func (b *LimitedBuffer) Write(p []byte) (n int, err error) {
	if b.truncated {
		return len(p), nil // Discard if already truncated
	}

	// Check shared counter first
	if b.sharedCounter != nil {
		if !b.sharedCounter.CanWrite(int64(len(p))) {
			b.truncated = true
			return len(p), nil // Discard
		}
	}

	// Check individual buffer limit
	if b.bytesWritten+int64(len(p)) > b.maxBytes {
		// Write only up to limit
		remaining := b.maxBytes - b.bytesWritten
		if remaining > 0 {
			n, err = b.Buffer.Write(p[:remaining])
			b.bytesWritten += int64(n)
		}
		b.truncated = true
		return len(p), nil // Report full write to satisfy caller
	}

	n, err = b.Buffer.Write(p)
	b.bytesWritten += int64(n)
	return
}

// IsTruncated returns whether output was truncated
func (b *LimitedBuffer) IsTruncated() bool {
	return b.truncated
}

// BytesWritten returns total bytes written
func (b *LimitedBuffer) BytesWritten() int64 {
	return b.bytesWritten
}

// SharedCounter tracks total bytes across multiple writers
type SharedCounter struct {
	maxBytes int64
	total    int64
}

// CanWrite checks if we can write n more bytes
func (c *SharedCounter) CanWrite(n int64) bool {
	if c.total+n > c.maxBytes {
		return false
	}
	c.total += n
	return true
}

// ErrorResponse creates a JSON error response
func ErrorResponse(w http.ResponseWriter, statusCode int, errMsg, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   errMsg,
		Message: message,
	})
}

// Ensure Executor implements io.Writer interface check
var _ io.Writer = (*LimitedBuffer)(nil)
