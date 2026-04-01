package models

import "time"

// Config represents the server configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Whitelist WhitelistConfig `yaml:"whitelist"`
	Audit     AuditConfig     `yaml:"audit"`
}

// ServerConfig represents server settings
type ServerConfig struct {
	Listen        string `yaml:"listen"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	MaxOutputMB   int64  `yaml:"max_output_mb"`
	MaxConcurrent int    `yaml:"max_concurrent"`
	ApiToken      string `yaml:"api_token"`
}

// WhitelistConfig represents whitelist rules
type WhitelistConfig struct {
	LiteralCommands      []string `yaml:"literal_commands"`
	RegexCommands        []string `yaml:"regex_commands"`
	AllowedPaths         []string `yaml:"allowed_paths"`
	ReloadIntervalSeconds int     `yaml:"reload_interval_seconds"`
}

// AuditConfig represents audit settings
type AuditConfig struct {
	Enabled        bool   `yaml:"enabled"`
	LogFile        string `yaml:"log_file"`
	RotationMaxMB  int64  `yaml:"rotation_max_mb"`
	RotationCount  int    `yaml:"rotation_count"`
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	Timestamp       string   `json:"timestamp"`
	Command         string   `json:"command"`
	Args            []string `json:"args,omitempty"`
	Cwd             string   `json:"cwd"`
	TokenPrefix     string   `json:"token_prefix"`
	ExitCode        int      `json:"exit_code"`
	DurationMs      int64    `json:"duration_ms"`
	OutputSizeBytes int64    `json:"output_size_bytes"`
	Truncated       bool     `json:"truncated"`
	AllowedBy       string   `json:"allowed_by"`
}

// ConfigWithMetadata wraps config with reload metadata
type ConfigWithMetadata struct {
	Config     *Config
	LastMod    time.Time
	ConfigPath string
}
