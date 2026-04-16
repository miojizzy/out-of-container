// Package whitelist 提供命令白名单验证功能。
//
// 实现了基于字面量和正则表达式的命令和路径白名单检查机制。
package whitelist

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/user/exec-server/internal/models"
	"gopkg.in/yaml.v3"
)

var (
	ErrCommandNotInWhitelist = errors.New("command not in whitelist")
	ErrPathNotAllowed        = errors.New("path not in allowed_paths")
)

// Checker validates commands against whitelist rules
type Checker struct {
	configPath string
	config     *models.Config
	lastMod    time.Time
	mutex      sync.RWMutex
	rules      []models.WhitelistRule
	regexCache map[string]struct{} // Track compiled regexes
}

// NewChecker creates a new whitelist checker
func NewChecker(configPath string) (*Checker, error) {
	checker := &Checker{
		configPath: configPath,
		regexCache: make(map[string]struct{}),
	}

	// Initial load
	if err := checker.loadConfig(); err != nil {
		return nil, err
	}

	// Compile rules
	if err := checker.compileRules(); err != nil {
		return nil, err
	}

	return checker, nil
}

// IsAllowed checks if command is in whitelist and cwd is allowed
func (c *Checker) IsAllowed(command, cwd string) (bool, string, error) {
	// First, check if we need to trigger a config reload (no lock needed)
	needsReload := false
	c.mutex.RLock()
	if time.Since(c.lastMod) > time.Duration(c.config.Whitelist.ReloadIntervalSeconds)*time.Second {
		needsReload = true
	}
	c.mutex.RUnlock()

	// Trigger async reload outside of lock if needed
	if needsReload {
		go c.reloadConfig()
	}

	// Now check command and path under read lock with short duration
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Check command against rules
	for _, rule := range c.rules {
		matched, err := rule.Match(command)
		if err != nil {
			return false, "", err
		}
		if matched {
			// Check working directory
			if !c.isPathAllowed(cwd) {
				return false, "", ErrPathNotAllowed
			}
			return true, rule.Type(), nil
		}
	}

	return false, "", ErrCommandNotInWhitelist
}

// isPathAllowed checks if path is in allowed_paths (with symlink resolution)
func (c *Checker) isPathAllowed(path string) bool {
	// Resolve symlinks and normalize path
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If resolution fails, use original path
		realPath = path
	}

	// Get absolute path
	absPath, err := filepath.Abs(realPath)
	if err != nil {
		return false
	}

	// Check against allowed paths
	for _, allowedPath := range c.config.Whitelist.AllowedPaths {
		// Resolve allowed path as well
		realAllowed, err := filepath.EvalSymlinks(allowedPath)
		if err != nil {
			realAllowed = allowedPath
		}

		absAllowed, err := filepath.Abs(realAllowed)
		if err != nil {
			continue
		}

		// Check if path starts with allowed path
		if len(absPath) >= len(absAllowed) && absPath[:len(absAllowed)] == absAllowed {
			return true
		}
	}

	return false
}

// loadConfig loads configuration from file
func (c *Checker) loadConfig() error {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(c.configPath)
	if err != nil {
		return err
	}

	// Parse YAML config
	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.config = &config
	c.lastMod = info.ModTime()

	return nil
}

// reloadConfig reloads configuration if file has changed
func (c *Checker) reloadConfig() {
	info, err := os.Stat(c.configPath)
	if err != nil {
		return
	}

	c.mutex.RLock()
	lastMod := c.lastMod
	c.mutex.RUnlock()

	if info.ModTime().After(lastMod) {
		// File changed, reload
		// Get current config with proper locking
		c.mutex.RLock()
		oldConfig := c.config
		c.mutex.RUnlock()

		if err := c.loadConfig(); err != nil {
			// Keep old config on error
			c.mutex.Lock()
			c.config = oldConfig
			c.mutex.Unlock()
			return
		}

		// Recompile rules
		if err := c.compileRules(); err != nil {
			// Keep old config on compile error
			c.mutex.Lock()
			c.config = oldConfig
			c.mutex.Unlock()
			return
		}
	}
}

// compileRules compiles literal and regex rules into WhitelistRule slice
func (c *Checker) compileRules() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	rules := make([]models.WhitelistRule, 0, len(c.config.Whitelist.LiteralCommands)+len(c.config.Whitelist.RegexCommands))

	// Add literal rules
	for _, cmd := range c.config.Whitelist.LiteralCommands {
		rules = append(rules, models.NewLiteralRule(cmd))
	}

	// Add regex rules (already compiled at startup)
	for _, regex := range c.config.Whitelist.RegexCommands {
		rule, err := models.NewRegexRule(regex)
		if err != nil {
			return err
		}
		rules = append(rules, rule)
		c.regexCache[regex] = struct{}{} // Mark as compiled
	}

	c.rules = rules
	return nil
}

// GetConfig returns a copy of the current configuration
// This is safe to call concurrently as it uses the internal mutex
func (c *Checker) GetConfig() *models.Config {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.config == nil {
		return nil
	}

	// Return a copy to prevent external modification
	configCopy := *c.config
	return &configCopy
}
