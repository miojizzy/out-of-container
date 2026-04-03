package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ClientConfig struct {
	ServerURL     string `yaml:"server_url"`
	ApiToken      string `yaml:"api_token"`
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
	serverURL  = flag.String("server", "", "Server URL")
	apiToken   = flag.String("token", "", "API token")
	command    = flag.String("command", "", "Command to execute")
	args       = flag.String("args", "", "Command arguments (comma-separated)")
	cwd        = flag.String("cwd", "", "Working directory")
	configPath = flag.String("config", "", "Config file path")
)

func main() {
	flag.Parse()

	// Load config
	configFile := *configPath
	if configFile == "" {
		home, _ := os.UserHomeDir()
		configFile = filepath.Join(home, ".config/ooc-client/config.yaml")
	}

	var cfg ClientConfig
	if data, err := os.ReadFile(configFile); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse config file: %v\n", err)
		}
	}

	// Override with flags
	if *serverURL != "" {
		cfg.ServerURL = *serverURL
	}
	if *apiToken != "" {
		cfg.ApiToken = *apiToken
	}

	// Validate
	if cfg.ServerURL == "" {
		fmt.Fprintln(os.Stderr, "Server URL required (set in config or use -server flag)")
		os.Exit(1)
	}
	if cfg.ApiToken == "" {
		fmt.Fprintln(os.Stderr, "API token required (set in config or use -token flag)")
		os.Exit(1)
	}
	if *command == "" {
		fmt.Fprintln(os.Stderr, "Command required (use -command flag)")
		os.Exit(1)
	}
	if *cwd == "" {
		cwd = new(string)
		*cwd, _ = os.Getwd()
	}

	// Parse args
	var argsList []string
	if *args != "" {
		if err := json.Unmarshal([]byte("["+*args+"]"), &argsList); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse args: %v\n", err)
			os.Exit(1)
		}
	}

	// Execute
	req := ExecRequest{
		Command: *command,
		Args:    argsList,
		Cwd:     *cwd,
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", cfg.ServerURL+"/ooc-exec", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+cfg.ApiToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse error response: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error: %s - %s\n", errResp.Error, errResp.Message)
		os.Exit(1)
	}

	var execResp ExecResponse
	if err := json.Unmarshal(respBody, &execResp); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(execResp.Stdout)
	if execResp.Stderr != "" {
		fmt.Fprint(os.Stderr, execResp.Stderr)
	}
	if execResp.Truncated {
		fmt.Fprintln(os.Stderr, "\n[Output truncated at 10MB]")
	}

	os.Exit(execResp.ExitCode)
}
