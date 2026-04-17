// Package main 提供容器外命令执行客户端工具。
//
// ooc-client 是一个命令行工具，用于向 exec-server 发送命令执行请求。
// 支持命令发现模式和命令执行模式。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// 版本信息，在构建时通过 -ldflags 注入
var (
	Version   = "dev"
	BuildTime = ""
)

func init() {
	// 如果 BuildTime 未设置，使用当前时间
	if BuildTime == "" {
		BuildTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}
}

// WhitelistInfoResponse 白名单信息发现响应结构体
type WhitelistInfoResponse struct {
	LiteralCommands       []string `json:"literal_commands"`
	RegexCommands         []string `json:"regex_commands"`
	AllowedPaths          []string `json:"allowed_paths"`
	ReloadIntervalSeconds int      `json:"reload_interval_seconds"`
}

type ClientConfig struct {
	ServerURL     string `yaml:"server_url"`
	APIToken      string `yaml:"api_token"`
	TimeoutSecond int    `yaml:"timeout_seconds"`
}

type ExecRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Cwd     string   `json:"cwd"`
}

type ExecResponse struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	DurationMs int64  `json:"duration_ms"`
	Truncated  bool   `json:"truncated"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

var (
	serverURL     = flag.String("server", "", "Server URL")
	apiToken      = flag.String("token", "", "API token")
	command       = flag.String("command", "", "Command to execute")
	args          = flag.String("args", "", "Command arguments (comma-separated)")
	cwd           = flag.String("cwd", "", "Working directory")
	configPath    = flag.String("config", "", "Config file path")
	listCommands  = flag.Bool("list-commands", false, "List available commands from server")
	listPaths     = flag.Bool("list-paths", false, "List allowed paths from server")
	discoveryOnly = flag.Bool("discovery-only", false, "Only perform discovery, don't execute commands")
	version       = flag.Bool("version", false, "Show version information")
)

func main() {
	os.Exit(run())
}

// run 是主逻辑入口，返回退出码。将逻辑提取到独立函数以确保 defer 正常执行。
func run() int {
	flag.Parse()

	// 显示版本信息
	if *version {
		log.Printf("ooc-client version %s", Version)
		log.Printf("Build time: %s", BuildTime)
		return 0
	}

	// Load config
	configFile := *configPath
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Printf("Failed to get user home directory: %v", err)
			printHelp()
			return 1
		}
		configFile = filepath.Join(home, ".config/ooc-client/config.yaml")
	}

	var cfg ClientConfig
	if data, err := os.ReadFile(configFile); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.Printf("Failed to parse config file: %v", err)
		}
	}

	// Override with flags
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *apiToken != "" {
		cfg.APIToken = *apiToken
	}

	// Validate
	if cfg.ServerURL == "" {
		log.Println("Server URL required (set in config or use -server flag)")
		printHelp()
		return 1
	}
	if cfg.APIToken == "" {
		log.Println("API token required (set in config or use -token flag)")
		printHelp()
		return 1
	}

	// 发现模式处理
	if *listCommands || *listPaths || *discoveryOnly {
		handleDiscovery(cfg)
		return 0
	}

	// 命令执行模式验证
	if *command == "" {
		log.Println("Command required (use -command flag) or use discovery flags (-list-commands, -list-paths)")
		printHelp()
		return 1
	}
	if *cwd == "" {
		cwd = new(string)
		*cwd, _ = os.Getwd()
	}

	// Parse args
	var argsList []string
	if *args != "" {
		// Split comma-separated args into array
		for _, arg := range strings.Split(*args, ",") {
			if trimmed := strings.TrimSpace(arg); trimmed != "" {
				argsList = append(argsList, trimmed)
			}
		}
	}

	// Execute
	req := ExecRequest{
		Command: *command,
		Args:    argsList,
		Cwd:     *cwd,
	}

	body, err := json.Marshal(req)
	if err != nil {
		log.Printf("Failed to marshal request: %v", err)
		return 1
	}
	httpReq, err := http.NewRequest("POST", cfg.ServerURL+"/ooc-exec", bytes.NewReader(body))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		return 1
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return 1
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read response body: %v", err)
		return 1
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err != nil {
			log.Printf("Failed to parse error response: %v", err)
			return 1
		}
		log.Printf("Error: %s - %s", errResp.Error, errResp.Message)
		return 1
	}

	var execResp ExecResponse
	if err := json.Unmarshal(respBody, &execResp); err != nil {
		log.Printf("Failed to parse response: %v", err)
		return 1
	}

	fmt.Print(execResp.Stdout)
	if execResp.Stderr != "" {
		fmt.Fprint(os.Stderr, execResp.Stderr)
	}
	if execResp.Truncated {
		fmt.Fprintln(os.Stderr, "\n[Output truncated at 10MB]")
	}

	return execResp.ExitCode
}

// handleDiscovery 处理白名单信息发现请求
func handleDiscovery(cfg ClientConfig) {
	// 查询服务器白名单信息
	info, err := fetchWhitelistInfo(cfg)
	if err != nil {
		log.Printf("Discovery failed: %v", err)
		os.Exit(1)
	}

	// 根据用户请求展示相应信息
	if *listCommands {
		printCommands(info)
	}
	if *listPaths && *listCommands {
		fmt.Println() // 分隔行
	}
	if *listPaths {
		printPaths(info)
	}

	// 如果没有指定具体选项，显示全部信息
	if !*listCommands && !*listPaths {
		printAllInfo(info)
	}
}

// fetchWhitelistInfo 从服务器获取白名单信息
func fetchWhitelistInfo(cfg ClientConfig) (*WhitelistInfoResponse, error) {
	url := cfg.ServerURL + "/whitelist-info"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err != nil {
			return nil, fmt.Errorf("server returned status %d with unparseable error", resp.StatusCode)
		}

		// 处理认证错误
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("authentication failed: %s", errResp.Message)
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error, errResp.Message)
	}

	var info WhitelistInfoResponse
	if err := json.Unmarshal(respBody, &info); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &info, nil
}

// printCommands 以用户友好的方式显示可用命令
func printCommands(info *WhitelistInfoResponse) {
	fmt.Println("=== 可用命令 ===")
	if len(info.LiteralCommands) == 0 && len(info.RegexCommands) == 0 {
		fmt.Println("未配置任何允许的命令")
		return
	}

	if len(info.LiteralCommands) > 0 {
		fmt.Println("字面量命令:")
		for _, cmd := range info.LiteralCommands {
			fmt.Printf("  %s\n", cmd)
		}
	}

	if len(info.RegexCommands) > 0 {
		if len(info.LiteralCommands) > 0 {
			fmt.Println()
		}
		fmt.Println("正则表达式命令:")
		for _, pattern := range info.RegexCommands {
			fmt.Printf("  %s\n", pattern)
		}
	}
}

// printPaths 以用户友好的方式显示允许的路径
func printPaths(info *WhitelistInfoResponse) {
	fmt.Println("=== 允许的路径 ===")
	if len(info.AllowedPaths) == 0 {
		fmt.Println("未配置任何允许的路径")
		return
	}

	for _, path := range info.AllowedPaths {
		fmt.Printf("  %s\n", path)
	}
}

// printAllInfo 显示所有白名单信息
func printAllInfo(info *WhitelistInfoResponse) {
	fmt.Printf("白名单配置信息:\n")
	fmt.Printf("  重载间隔: %d 秒\n", info.ReloadIntervalSeconds)
	fmt.Println()

	printCommands(info)
	fmt.Println()
	printPaths(info)
}

// printHelp 显示帮助信息
func printHelp() {
	fmt.Println("ooc-client - 容器外命令执行客户端")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  ooc-client [选项]")
	fmt.Println()
	fmt.Println("发现模式:")
	fmt.Println("  -list-commands       列出服务器允许的命令")
	fmt.Println("  -list-paths          列出服务器允许的路径")
	fmt.Println("  -discovery-only      仅执行发现操作（不执行命令）")
	fmt.Println()
	fmt.Println("命令执行模式:")
	fmt.Println("  -server <url>       服务器地址")
	fmt.Println("  -token <token>      API令牌")
	fmt.Println("  -command <cmd>      要执行的命令")
	fmt.Println("  -args <args>        命令参数（逗号分隔）")
	fmt.Println("  -cwd <dir>          工作目录（默认为当前目录）")
	fmt.Println("  -config <path>      配置文件路径（默认为 ~/.config/ooc-client/config.yaml）")
	fmt.Println()
	fmt.Println("配置文件示例 (YAML):")
	fmt.Println("  server_url: \"http://localhost:8080\"")
	fmt.Println("  api_token: \"your-api-token\"")
	fmt.Println("  timeout_seconds: 30")
}
