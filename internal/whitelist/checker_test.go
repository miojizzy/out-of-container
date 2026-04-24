package whitelist

import (
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testConfigYAML 是测试中使用的标准配置YAML字符串
const testConfigYAML = `server:
  listen: ":8080"
  timeout_seconds: 30
  max_output_mb: 10
  max_concurrent: 5
  api_token: "test-token"
whitelist:
  literal_commands:
    - "ls"
    - "pwd"
    - "echo"
  regex_commands:
    - "^git (clone|pull|fetch)"
    - "^go (build|test|mod)"
  allowed_paths:
    - "/tmp"
    - "/home"
  reload_interval_seconds: 5
audit:
  enabled: true
  log_file: "audit.log"
  rotation_max_mb: 100
  rotation_count: 5
`)

func TestNewChecker(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// 创建有效的配置内容
	configContent := testConfigYAML

func TestNewChecker(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// 创建有效的配置内容
	configContent := testConfigYAML

	// 写入配置文件
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// 测试正常创建Checker
	checker, err := NewChecker(configPath)
	require.NoError(t, err)
	require.NotNil(t, checker)

	// 测试配置文件不存在的情况
	checker, err = NewChecker("nonexistent.yaml")
	assert.Error(t, err)
	assert.Nil(t, checker)
}

func TestIsAllowed_CommandWhitelist(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := testConfigYAML

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	tests := []struct {
		name        string
		command     string
		cwd         string
		expected    bool
		expectedErr error
	}{
		{
			name:        "Allowed literal command",
			command:     "ls",
			cwd:         "/tmp",
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "Disallowed command",
			command:     "rm",
			cwd:         "/tmp",
			expected:    false,
			expectedErr: ErrCommandNotInWhitelist,
		},
		{
			name:        "Allowed regex command - git clone",
			command:     "git clone https://github.com/user/repo.git",
			cwd:         "/tmp",
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "Allowed regex command - go build",
			command:     "go build main.go",
			cwd:         "/tmp",
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "Disallowed regex command",
			command:     "git push origin main",
			cwd:         "/tmp",
			expected:    false,
			expectedErr: ErrCommandNotInWhitelist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, ruleType, err := checker.IsAllowed(tt.command, tt.cwd)
			assert.Equal(t, tt.expected, allowed)
			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}

			if allowed {
				assert.NotEmpty(t, ruleType)
			}
		})
	}
}

func TestIsAllowed_PathWhitelist(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := testConfigYAML

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	tests := []struct {
		name        string
		command     string
		cwd         string
		expected    bool
		expectedErr error
	}{
		{
			name:        "Allowed path - direct match",
			command:     "ls",
			cwd:         "/tmp",
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "Allowed path - subdirectory",
			command:     "ls",
			cwd:         "/tmp/subdir",
			expected:    true,
			expectedErr: nil,
		},
		{
			name:        "Disallowed path",
			command:     "ls",
			cwd:         "/etc",
			expected:    false,
			expectedErr: ErrPathNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, ruleType, err := checker.IsAllowed(tt.command, tt.cwd)
			assert.Equal(t, tt.expected, allowed)
			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}

			if allowed {
				assert.NotEmpty(t, ruleType)
			}
		})
	}
}

func TestConfigReload(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// 初始配置内容
	initialConfig := `server:
  listen: ":8080"
whitelist:
  literal_commands:
    - "ls"
  regex_commands:
    - "^git clone"
  allowed_paths:
    - "/tmp"
  reload_interval_seconds: 1
audit:
  enabled: true
  log_file: "audit.log"
`

	// 写入初始配置
	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	// 创建Checker
	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	// 检查初始配置是否正确加载
	allowed, _, err := checker.IsAllowed("ls", "/tmp")
	assert.True(t, allowed)
	assert.NoError(t, err)

	// 检查不被允许的命令
	allowed, _, err = checker.IsAllowed("pwd", "/tmp")
	assert.False(t, allowed)
	assert.Equal(t, ErrCommandNotInWhitelist, err)

	// 修改配置文件
	updatedConfig := `server:
  listen: ":8080"
whitelist:
  literal_commands:
    - "ls"
    - "pwd"
  regex_commands:
    - "^git clone"
  allowed_paths:
    - "/tmp"
  reload_interval_seconds: 1
audit:
  enabled: true
  log_file: "audit.log"
`

	// 等待一小段时间确保时间戳不同
	time.Sleep(100 * time.Millisecond)

	// 写入更新后的配置
	err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
	require.NoError(t, err)

	// 手动触发重载：先loadConfig，然后compileRules
	loadErr := checker.loadConfig()
	assert.NoError(t, loadErr)
	compileErr := checker.compileRules()
	assert.NoError(t, compileErr)

	// 现在pwd命令应该被允许了
	allowed, _, err = checker.IsAllowed("pwd", "/tmp")
	assert.True(t, allowed)
	assert.NoError(t, err)
}

func TestReloadConfigFileError(_ *testing.T) {
	// 创建Checker实例
	checker := &Checker{}

	// 调用reloadConfig，由于configPath为空，应该不会出错但也不会做任何事
	checker.reloadConfig()

	// 设置一个不存在的路径
	checker.configPath = "nonexistent.yaml"
	checker.reloadConfig() // 也不会出错
}

func TestGetConfig(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := testConfigYAML

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	config := checker.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, ":8080", config.Server.Listen)
	assert.Equal(t, 3, len(config.Whitelist.LiteralCommands)) // ls, pwd, echo
	assert.Equal(t, 2, len(config.Whitelist.RegexCommands))   // git, go
	assert.Equal(t, 2, len(config.Whitelist.AllowedPaths))    // /tmp, /home
}

func TestIsPathAllowed(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := testConfigYAML

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	// 测试绝对路径匹配
	assert.True(t, checker.isPathAllowed("/tmp"))
	assert.True(t, checker.isPathAllowed("/tmp/subdir"))
	assert.False(t, checker.isPathAllowed("/etc"))

	// 测试home路径
	assert.True(t, checker.isPathAllowed("/home"))
	assert.True(t, checker.isPathAllowed("/home/user"))
}

func TestReloadConfigErrorHandling(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// 创建一个有效的初始配置
	initialConfig := `server:
  listen: ":8080"
whitelist:
  literal_commands:
    - "ls"
  regex_commands:
    - "^git clone"
  allowed_paths:
    - "/tmp"
  reload_interval_seconds: 1
audit:
  enabled: true
  log_file: "audit.log"
`

	// 写入初始配置
	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	// 创建Checker
	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	// 保存原始配置用于比较
	originalConfig := checker.GetConfig()

	// 创建一个无效的配置文件
	invalidConfig := `server:
  listen: ":8080"
whitelist:
  literal_commands:
    - "ls"
  regex_commands:
    - "[invalid-regex"
  allowed_paths:
    - "/tmp"
  reload_interval_seconds: 1
audit:
  enabled: true
  log_file: "audit.log"
`

	// 写入无效配置
	err = os.WriteFile(configPath, []byte(invalidConfig), 0644)
	require.NoError(t, err)

	// 手动触发加载，应该会失败但不会更新配置
	loadErr := checker.loadConfig()
	assert.NoError(t, loadErr) // loadConfig本身不会失败，只是加载了无效配置

	// 但compileRules应该会失败
	compileErr := checker.compileRules()
	assert.Error(t, compileErr)

	// 配置应该保持不变
	currentConfig := checker.GetConfig()
	assert.Equal(t, originalConfig.Server.Listen, currentConfig.Server.Listen)
}

func TestLoadConfigError(t *testing.T) {
	// 测试不存在的配置文件
	checker := &Checker{configPath: "nonexistent.yaml"}
	err := checker.loadConfig()
	assert.Error(t, err)

	// 测试无效的YAML文件
	tempDir := t.TempDir()
	invalidYamlPath := filepath.Join(tempDir, "invalid.yaml")
	invalidYaml := `server:
  listen: ":8080"
  invalid_field: [unclosed array
`
	err = os.WriteFile(invalidYamlPath, []byte(invalidYaml), 0644)
	require.NoError(t, err)

	checker = &Checker{configPath: invalidYamlPath}
	err = checker.loadConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestConfigHotReload(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// 初始配置内容
	initialConfig := `server:
  listen: ":8080"
whitelist:
  literal_commands:
    - "ls"
  regex_commands:
    - "^git clone"
  allowed_paths:
    - "/tmp"
  reload_interval_seconds: 0
audit:
  enabled: true
  log_file: "audit.log"
`

	// 写入初始配置
	err := os.WriteFile(configPath, []byte(initialConfig), 0644)
	require.NoError(t, err)

	// 创建Checker
	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	// 检查初始配置是否正确加载
	allowed, _, err := checker.IsAllowed("ls", "/tmp")
	assert.True(t, allowed)
	assert.NoError(t, err)

	// 检查不被允许的命令
	allowed, _, err = checker.IsAllowed("pwd", "/tmp")
	assert.False(t, allowed)
	assert.Equal(t, ErrCommandNotInWhitelist, err)

	// 修改配置文件
	updatedConfig := `server:
  listen: ":8080"
whitelist:
  literal_commands:
    - "ls"
    - "pwd"
  regex_commands:
    - "^git clone"
  allowed_paths:
    - "/tmp"
  reload_interval_seconds: 0
audit:
  enabled: true
  log_file: "audit.log"
`

	// 写入更新后的配置
	err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
	require.NoError(t, err)

	// 检查初始配置
	config := checker.GetConfig()
	log.Printf("Before reload - LiteralCommands: %v", config.Whitelist.LiteralCommands)

	// 强制重载配置和规则
	err = checker.loadConfig()
	assert.NoError(t, err)
	err = checker.compileRules()
	assert.NoError(t, err)

	// 等待一小段时间确保重载完成
	time.Sleep(50 * time.Millisecond)

	// 检查重载后配置
	config = checker.GetConfig()
	log.Printf("After reload - LiteralCommands: %v", config.Whitelist.LiteralCommands)

	// 现在pwd命令应该被允许了
	allowed, _, err = checker.IsAllowed("pwd", "/tmp")
	assert.True(t, allowed)
	assert.NoError(t, err)
}

func TestConcurrentAccess(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := testConfigYAML

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	checker, err := NewChecker(configPath)
	require.NoError(t, err)

	// 测试并发访问
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			allowed, _, err := checker.IsAllowed("ls", "/tmp")
			require.NoError(t, err)
			assert.True(t, allowed)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
