package whitelist

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/user/exec-server/internal/models"
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
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Check if config needs reload
	if time.Since(c.lastMod) > time.Duration(c.config.Whitelist.ReloadIntervalSeconds)*time.Second {
		go c.reloadConfig() // Async reload
	}

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
	_, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}

	info, err := os.Stat(c.configPath)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.lastMod = info.ModTime()

	// TODO: Parse YAML config
	// For now, return nil
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
		oldConfig := c.config
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
