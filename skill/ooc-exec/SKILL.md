---
name: ooc-exec
preamble-tier: 1
version: 1.0.0
description: |
  Execute commands on the host machine from within a container. This skill bridges the gap between containerized development environments and host system command execution.
allowed-tools:
  - Bash
  - Read

---

# ooc-exec: Out-of-Container Command Execution

## Preamble (run first)

```bash
# Check if the ooc-client binary exists and is executable
if [ ! -x "./ooc-client" ]; then
  echo "ERROR: ooc-client binary not found or not executable"
  echo "Please ensure ooc-client is installed and in the PATH"
  exit 1
fi

# Check if the configuration file exists
CONFIG_FILE="$HOME/.config/ooc-client/config.yaml"
if [ ! -f "$CONFIG_FILE" ]; then
  echo "WARNING: Configuration file not found at $CONFIG_FILE"
  echo "You may need to create it first. See documentation for details."
fi
```

## Description

Execute commands on the host machine from within a container. This skill bridges the gap between containerized development environments and host system command execution.

The `ooc-exec` (Out-of-Container Execution) skill allows AI agents like Claude Code to run compilation, testing, and other system commands on the host machine while working within a containerized development environment.

## How it works

1. A server component (`ooc-server`) runs on the host machine
2. A client binary (`ooc-client`) is installed inside the container
3. When this skill is invoked, it uses the `ooc-client` to send commands to the `ooc-server`
4. The server executes the commands and returns the results

## Usage

```bash
/ooc-exec command="ls" cwd="/home/user"
/ooc-exec command="go" args="test,-v" cwd="/app"
/ooc-exec command="make" args="build" cwd="/project"
```

## Parameters

- `command` (required): The command to execute on the host
- `args` (optional): Comma-separated arguments for the command
- `cwd` (required): Working directory on the host where the command should run

## Examples

```bash
# List files in the home directory
/ooc-exec command="ls" cwd="/home/user"

# Run Go tests with verbose output
/ooc-exec command="go" args="test,-v" cwd="/app"

# Build a project with Make
/ooc-exec command="make" args="build" cwd="/project"

# Run Python tests
/ooc-exec command="python" args="-m,pytest,tests/" cwd="/app"
```

## Security

This skill implements multiple security measures:

1. **API Token Authentication**: All requests must include a valid API token
2. **Command Whitelisting**: Only explicitly allowed commands can be executed
3. **Path Restrictions**: Commands can only run in specified directories
4. **Shell Metacharacter Detection**: Prevents command injection attacks
5. **Audit Logging**: All command executions are logged for security review

## Configuration

The client reads its configuration from `~/.config/ooc-client/config.yaml`:

```yaml
server_url: "http://localhost:8080"
api_token: "your-api-token-here"
timeout_seconds: 35
```

## Installation

1. Install the `ooc-server` on your host machine
2. Install the `ooc-client` in your container
3. Configure the client with the server URL and API token
4. Ensure the server has the necessary command whitelists configured

For detailed installation instructions, see the project README.