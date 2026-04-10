package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/exec-server/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestLoader_Load tests the Load method
func TestLoader_Load(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Test case 1: Successful config loading
	t.Run("SuccessfulConfigLoading", func(t *testing.T) {
		// Create a valid config file
		config := &models.Config{
			Server: models.ServerConfig{
				Listen:         "127.0.0.1:8080",
				TimeoutSeconds: 30,
				MaxOutputMB:    10,
				MaxConcurrent:  5,
				ApiToken:       "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls", "pwd"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{"/tmp"},
			},
			Audit: models.AuditConfig{
				Enabled:       true,
				LogFile:       "/tmp/audit.log",
				RotationMaxMB: 10,
				RotationCount: 5,
			},
		}

		// Write config to file
		configPath := filepath.Join(tempDir, "config.yaml")
		data, err := yaml.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)

		// Load config
		loader := NewLoader(configPath)
		loadedConfig, err := loader.Load()
		require.NoError(t, err)
		assert.Equal(t, config, loadedConfig)
	})

	// Test case 2: Config file not found
	t.Run("ConfigFileNotFound", func(t *testing.T) {
		loader := NewLoader(filepath.Join(tempDir, "nonexistent.yaml"))
		_, err := loader.Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read config file")
	})

	// Test case 3: Invalid YAML format
	t.Run("InvalidYAMLFormat", func(t *testing.T) {
		// Create an invalid config file
		configPath := filepath.Join(tempDir, "invalid.yaml")
		invalidData := []byte("invalid: yaml: content:")
		err := os.WriteFile(configPath, invalidData, 0644)
		require.NoError(t, err)

		// Try to load config
		loader := NewLoader(configPath)
		_, err = loader.Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse config file")
	})

	// Test case 4: Validation failure - missing API token
	t.Run("ValidationFailureMissingAPIToken", func(t *testing.T) {
		// Create a config without API token
		config := &models.Config{
			Server: models.ServerConfig{
				Listen:         "127.0.0.1:8080",
				TimeoutSeconds: 30,
				MaxOutputMB:    10,
				MaxConcurrent:  5,
				ApiToken:       "", // Empty API token
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls", "pwd"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{"/tmp"},
			},
		}

		// Write config to file
		configPath := filepath.Join(tempDir, "config-no-token.yaml")
		data, err := yaml.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)

		// Try to load config
		loader := NewLoader(configPath)
		_, err = loader.Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api_token is required")
	})

	// Test case 5: Validation failure - no whitelist commands
	t.Run("ValidationFailureNoWhitelistCommands", func(t *testing.T) {
		// Create a config without whitelist commands
		config := &models.Config{
			Server: models.ServerConfig{
				Listen:         "127.0.0.1:8080",
				TimeoutSeconds: 30,
				MaxOutputMB:    10,
				MaxConcurrent:  5,
				ApiToken:       "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{}, // Empty literal commands
				RegexCommands:   []string{}, // Empty regex commands
				AllowedPaths:    []string{"/tmp"},
			},
		}

		// Write config to file
		configPath := filepath.Join(tempDir, "config-no-whitelist.yaml")
		data, err := yaml.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)

		// Try to load config
		loader := NewLoader(configPath)
		_, err = loader.Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one whitelist command is required")
	})

	// Test case 6: Validation failure - no allowed paths
	t.Run("ValidationFailureNoAllowedPaths", func(t *testing.T) {
		// Create a config without allowed paths
		config := &models.Config{
			Server: models.ServerConfig{
				Listen:         "127.0.0.1:8080",
				TimeoutSeconds: 30,
				MaxOutputMB:    10,
				MaxConcurrent:  5,
				ApiToken:       "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls", "pwd"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{}, // Empty allowed paths
			},
		}

		// Write config to file
		configPath := filepath.Join(tempDir, "config-no-paths.yaml")
		data, err := yaml.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)

		// Try to load config
		loader := NewLoader(configPath)
		_, err = loader.Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one allowed_path is required")
	})

	// Test case 7: Validation failure - non-existent allowed path
	t.Run("ValidationFailureNonExistentAllowedPath", func(t *testing.T) {
		// Create a config with non-existent allowed path
		config := &models.Config{
			Server: models.ServerConfig{
				Listen:         "127.0.0.1:8080",
				TimeoutSeconds: 30,
				MaxOutputMB:    10,
				MaxConcurrent:  5,
				ApiToken:       "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls", "pwd"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{"/non/existent/path"}, // Non-existent path
			},
		}

		// Write config to file
		configPath := filepath.Join(tempDir, "config-invalid-path.yaml")
		data, err := yaml.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)

		// Try to load config
		loader := NewLoader(configPath)
		_, err = loader.Load()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "allowed_path does not exist")
	})
}

// TestLoader_validate tests the validate method
func TestLoader_validate(t *testing.T) {
	loader := NewLoader("test-config.yaml")

	// Test case 1: Valid config
	t.Run("ValidConfig", func(t *testing.T) {
		config := &models.Config{
			Server: models.ServerConfig{
				ApiToken: "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{"/tmp"},
			},
		}
		err := loader.validate(config)
		assert.NoError(t, err)
	})

	// Test case 2: Missing API token
	t.Run("MissingAPIToken", func(t *testing.T) {
		config := &models.Config{
			Server: models.ServerConfig{
				ApiToken: "", // Empty API token
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{"/tmp"},
			},
		}
		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "api_token is required")
	})

	// Test case 3: No whitelist commands
	t.Run("NoWhitelistCommands", func(t *testing.T) {
		config := &models.Config{
			Server: models.ServerConfig{
				ApiToken: "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{}, // Empty literal commands
				RegexCommands:   []string{}, // Empty regex commands
				AllowedPaths:    []string{"/tmp"},
			},
		}
		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one whitelist command is required")
	})

	// Test case 4: No allowed paths
	t.Run("NoAllowedPaths", func(t *testing.T) {
		config := &models.Config{
			Server: models.ServerConfig{
				ApiToken: "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{}, // Empty allowed paths
			},
		}
		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one allowed_path is required")
	})

	// Test case 5: Non-existent allowed path
	t.Run("NonExistentAllowedPath", func(t *testing.T) {
		config := &models.Config{
			Server: models.ServerConfig{
				ApiToken: "test-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{"/non/existent/path"}, // Non-existent path
			},
		}
		err := loader.validate(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "allowed_path does not exist")
	})
}

// TestLoader_InitConfig tests the InitConfig method
func TestLoader_InitConfig(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Test case 1: Successful config initialization
	t.Run("SuccessfulConfigInitialization", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "new-config.yaml")
		loader := NewLoader(configPath)

		err := loader.InitConfig()
		assert.NoError(t, err)

		// Check that the config file was created
		_, err = os.Stat(configPath)
		assert.NoError(t, err)

		// Modify the generated config to use test paths that exist
		// Read the config file
		configData, err := os.ReadFile(configPath)
		assert.NoError(t, err)

		// Parse the config
		var config models.Config
		err = yaml.Unmarshal(configData, &config)
		assert.NoError(t, err)

		// Change allowed paths to use temp directory paths that exist
		config.Whitelist.AllowedPaths = []string{tempDir, "/tmp"}

		// Write the modified config back
		modifiedData, err := yaml.Marshal(&config)
		assert.NoError(t, err)
		err = os.WriteFile(configPath, modifiedData, 0644)
		assert.NoError(t, err)

		// Load and verify the config
		loadedConfig, err := loader.Load()
		assert.NoError(t, err)
		assert.NotEmpty(t, loadedConfig.Server.ApiToken)
		assert.Equal(t, "0.0.0.0:8080", loadedConfig.Server.Listen)
		assert.NotEmpty(t, loadedConfig.Whitelist.LiteralCommands)
	})

	// Test case 2: Config file already exists
	t.Run("ConfigFileAlreadyExists", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "existing-config.yaml")

		// Create an existing config file
		existingConfig := &models.Config{
			Server: models.ServerConfig{
				Listen:   "127.0.0.1:8080",
				ApiToken: "existing-token",
			},
			Whitelist: models.WhitelistConfig{
				LiteralCommands: []string{"ls"},
				RegexCommands:   []string{},
				AllowedPaths:    []string{"/tmp"},
			},
		}
		data, err := yaml.Marshal(existingConfig)
		require.NoError(t, err)
		err = os.WriteFile(configPath, data, 0644)
		require.NoError(t, err)

		// Try to initialize config
		loader := NewLoader(configPath)
		err = loader.InitConfig()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config file already exists")
	})
}

// TestExpandPath tests the expandPath function
func TestExpandPath(t *testing.T) {
	// Test case 1: Path without tilde
	t.Run("PathWithoutTilde", func(t *testing.T) {
		path := "/home/user/test"
		result := expandPath(path)
		assert.Equal(t, path, result)
	})

	// Test case 2: Path with tilde at the beginning
	t.Run("PathWithTilde", func(t *testing.T) {
		path := "~/test"
		result := expandPath(path)
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		expected := filepath.Join(home, "test")
		assert.Equal(t, expected, result)
	})

	// Test case 3: Empty path
	t.Run("EmptyPath", func(t *testing.T) {
		path := ""
		result := expandPath(path)
		assert.Equal(t, path, result)
	})

	// Test case 4: Path with tilde in the middle (should not be expanded)
	t.Run("PathWithTildeInMiddle", func(t *testing.T) {
		path := "/path/~/test"
		result := expandPath(path)
		assert.Equal(t, path, result)
	})
}
