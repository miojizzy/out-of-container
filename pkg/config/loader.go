package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/user/exec-server/internal/models"
	"gopkg.in/yaml.v3"
)

// Loader handles config file operations
type Loader struct {
	configPath string
}

// NewLoader creates a new config loader
func NewLoader(configPath string) *Loader {
	return &Loader{configPath: configPath}
}

// Load reads and parses config file
func (l *Loader) Load() (*models.Config, error) {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config models.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate config
	if err := l.validate(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validate checks config validity
func (l *Loader) validate(config *models.Config) error {
	if config.Server.APIToken == "" {
		return fmt.Errorf("api_token is required")
	}

	if len(config.Whitelist.LiteralCommands) == 0 && len(config.Whitelist.RegexCommands) == 0 {
		return fmt.Errorf("at least one whitelist command is required")
	}

	if len(config.Whitelist.AllowedPaths) == 0 {
		return fmt.Errorf("at least one allowed_path is required")
	}

	// Validate allowed paths exist
	for _, path := range config.Whitelist.AllowedPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("allowed_path does not exist: %s", path)
		}
	}

	return nil
}

// InitConfig generates a new config file with random token
func (l *Loader) InitConfig() error {
	// Check if file already exists
	if _, err := os.Stat(l.configPath); err == nil {
		return fmt.Errorf("config file already exists: %s", l.configPath)
	}

	// Generate random token
	token, err := generateToken(32) // 32 bytes = 64 hex chars
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	// Create default config
	config := &models.Config{
		Server: models.ServerConfig{
			Listen:         "0.0.0.0:8080",
			TimeoutSeconds: 30,
			MaxOutputMB:    10,
			MaxConcurrent:  5,
			APIToken:       token,
			TaskTTLHours:   24,
		},
		Whitelist: models.WhitelistConfig{
			LiteralCommands: []string{
				"make",
				"cmake",
				"g++",
				"gcc",
				"python3",
				"pytest",
			},
			AllowedPaths: []string{
				"/home/user/projects",
				"/tmp/build",
			},
			ReloadIntervalSeconds: 5,
		},
		Audit: models.AuditConfig{
			Enabled:       true,
			LogFile:       "~/.local/share/ooc-server/audit.log",
			RotationMaxMB: 10,
			RotationCount: 10,
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(l.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write file with restricted permissions
	if err := os.WriteFile(l.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Create log directory
	logDir := expandPath(config.Audit.LogFile)
	logDir = filepath.Dir(logDir)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	fmt.Printf("Config file created: %s\n", l.configPath)
	fmt.Printf("API Token: %s\n", token)
	fmt.Println("\nPlease edit the config file to customize:")
	fmt.Println("  - literal_commands: Add your allowed commands")
	fmt.Println("  - allowed_paths: Set your project directories")

	return nil
}

// generateToken generates a random hex token
func generateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
