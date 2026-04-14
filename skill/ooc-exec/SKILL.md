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
# Basic command execution
./ooc-client -command "ls" -cwd "/home/user"

# Command with arguments (JSON array format)
./ooc-client -command "go" -args '["test","-v"]' -cwd "/app"
./ooc-client -command "make" -args '["build"]' -cwd "/project"

# Query available commands
./ooc-client -server http://localhost:8080 -token YOUR_TOKEN -list-commands

# Query allowed paths
./ooc-client -server http://localhost:8080 -token YOUR_TOKEN -list-paths
```

## Parameters

- `command` (required): The command to execute on the host
- `args` (optional): Command arguments in JSON array format, e.g., `'["-v","-race"]'`
- `cwd` (required): Working directory on the host where the command should run
- `server` (optional): Override server URL from config file
- `token` (optional): Override API token from config file
- `list-commands` (optional): List all whitelisted commands on the server
- `list-paths` (optional): List all allowed paths on the server

## Examples

```bash
# List files in the home directory
./ooc-client -command "ls" -cwd "/home/user"

# Run Go tests with verbose output and race detector
./ooc-client -command "go" -args '["test","-v","-race"]' -cwd "/app"

# Build a project with Make
./ooc-client -command "make" -args '["build"]' -cwd "/project"

# Run Python tests
./ooc-client -command "python" -args '["-m","pytest","tests/"]' -cwd "/app"

# Compile C++ code with specific standard
./ooc-client -command "g++" -args '["-std=c++17","main.cpp","-o","main"]' -cwd "/project"

# Query server capabilities
./ooc-client -list-commands
./ooc-client -list-paths
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

## Update

Automatically update `ooc-client` binary and skill documentation to the latest release from GitHub.

### Update Command

```bash
# Update to the latest version
update-ooc-exec
```

### Update Process

The update process performs the following steps:

1. **Check current version**
   ```bash
   ./ooc-client -version 2>/dev/null || echo "Not installed or version unknown"
   ```

2. **Fetch latest release information**
   ```bash
   # Get latest release metadata from GitHub API
   LATEST_RELEASE=$(curl -s https://api.github.com/repos/miojizzy/out-of-container/releases/latest)
   LATEST_VERSION=$(echo "$LATEST_RELEASE" | jq -r '.tag_name')
   DOWNLOAD_URL=$(echo "$LATEST_RELEASE" | jq -r '.assets[] | select(.name | test("ooc-client-linux-")) | .browser_download_url')
   SKILL_URL=$(echo "$LATEST_RELEASE" | jq -r '.assets[] | select(.name == "SKILL.md") | .browser_download_url')
   ```

3. **Download latest binaries**
   ```bash
   # Determine architecture
   ARCH=$(uname -m)
   case $ARCH in
     x86_64) BINARY="ooc-client-linux-amd64" ;;
     aarch64) BINARY="ooc-client-linux-arm64" ;;
     *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
   esac

   # Download binary
   curl -L -o ooc-client "https://github.com/miojizzy/out-of-container/releases/download/${LATEST_VERSION}/${BINARY}"
   chmod +x ooc-client
   ```

4. **Download latest skill documentation**
   ```bash
   # Download SKILL.md
   curl -L -o SKILL.md "https://github.com/miojizzy/out-of-container/releases/download/${LATEST_VERSION}/SKILL.md"
   ```

5. **Verify installation**
   ```bash
   # Verify binary is executable
   ./ooc-client -version

   # Verify checksum (optional but recommended)
   curl -L -o checksums.txt "https://github.com/miojizzy/out-of-container/releases/download/${LATEST_VERSION}/checksums.txt"
   sha256sum -c checksums.txt --ignore-missing
   ```

6. **Replace local files**
   ```bash
   # Backup current version (optional)
   [ -f "./ooc-client" ] && mv ./ooc-client ./ooc-client.backup
   [ -f "./SKILL.md" ] && mv ./SKILL.md ./SKILL.md.backup

   # Replace with new version
   mv ooc-client ./ooc-client
   mv SKILL.md ./SKILL.md

   echo "Updated to version: $LATEST_VERSION"
   ```

### Manual Update

If automatic update fails, perform manual update:

```bash
# 1. Get latest version from GitHub
open https://github.com/miojizzy/out-of-container/releases/latest

# 2. Download files manually
# - ooc-client-linux-amd64 (or arm64)
# - SKILL.md
# - checksums.txt

# 3. Verify checksum
sha256sum -c checksums.txt --ignore-missing

# 4. Install
chmod +x ooc-client-linux-*
mv ooc-client-linux-* ooc-client
```

### Version Check

Check current installed version:

```bash
./ooc-client -version
```

Compare with latest available version:

```bash
# Latest version from GitHub
curl -s https://api.github.com/repos/miojizzy/out-of-container/releases/latest | jq -r '.tag_name'

# Current version
./ooc-client -version 2>/dev/null || echo "Not installed"
```
