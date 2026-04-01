package models

// Command represents an execution request
type Command struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Cwd     string   `json:"cwd"`
}

// Result represents command execution result
type Result struct {
	ExitCode     int    `json:"exit_code"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	DurationMs   int64  `json:"duration_ms"`
	Truncated    bool   `json:"truncated"`
	OutputSize   int64  `json:"output_size_bytes,omitempty"`
	TruncatedAt  int64  `json:"truncated_at_mb,omitempty"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
