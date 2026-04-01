package auditor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/user/exec-server/internal/models"
)

// Auditor handles audit logging
type Auditor struct {
	logFile       string
	rotationMaxMB int64
	rotationCount int
	channel       chan *models.AuditEntry
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewAuditor creates a new auditor with buffered channel
func NewAuditor(logFile string, rotationMaxMB int64, rotationCount int) *Auditor {
	ctx, cancel := context.WithCancel(context.Background())

	a := &Auditor{
		logFile:       expandPath(logFile),
		rotationMaxMB: rotationMaxMB,
		rotationCount: rotationCount,
		channel:       make(chan *models.AuditEntry, 1000), // Bounded buffer
		ctx:           ctx,
		cancel:        cancel,
	}

	// Start background writer goroutine
	a.wg.Add(1)
	go a.writerLoop()

	return a
}

// Log queues an audit entry for writing
func (a *Auditor) Log(entry *models.AuditEntry) {
	select {
	case a.channel <- entry:
		// Entry queued successfully
	default:
		// Channel full, entry lost (should be rare with buffer=1000)
		// Could log to stderr here if needed
	}
}

// Close gracefully shuts down the auditor
func (a *Auditor) Close() error {
	a.cancel()

	// Wait for writer goroutine with timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("auditor shutdown timeout, some entries may be lost")
	}
}

// writerLoop runs in background goroutine to serialize writes
func (a *Auditor) writerLoop() {
	defer a.wg.Done()

	for {
		select {
		case entry := <-a.channel:
			if err := a.writeEntry(entry); err != nil {
				// Log error to stderr (don't block on file write errors)
				fmt.Fprintf(os.Stderr, "audit log error: %v\n", err)
			}
		case <-a.ctx.Done():
			// Drain remaining entries
			for len(a.channel) > 0 {
				entry := <-a.channel
				if err := a.writeEntry(entry); err != nil {
					fmt.Fprintf(os.Stderr, "audit log error: %v\n", err)
				}
			}
			return
		}
	}
}

// writeEntry writes a single audit entry to file
func (a *Auditor) writeEntry(entry *models.AuditEntry) error {
	// Ensure log directory exists
	dir := filepath.Dir(a.logFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Open file in append mode
	f, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Check if rotation needed
	if info, err := f.Stat(); err == nil {
		if info.Size() >= a.rotationMaxMB*1024*1024 {
			f.Close()
			if err := a.rotateLog(); err != nil {
				return err
			}
			// Reopen file
			f, err = os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			defer f.Close()
		}
	}

	// Write JSONL (single line JSON)
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

// rotateLog performs log rotation
func (a *Auditor) rotateLog() error {
	// Move existing files: log.1 -> log.2, log -> log.1
	for i := a.rotationCount - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", a.logFile, i)
		newPath := fmt.Sprintf("%s.%d", a.logFile, i+1)
		os.Rename(oldPath, newPath)
	}

	// Move current log to .1
	if err := os.Rename(a.logFile, a.logFile+".1"); err != nil {
		return err
	}
	return nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
