---
name: exec
preamble-tier: 2
version: 1.0.0
description: |
  容器远程命令执行系统。在容器内执行命令，通过 HTTP 发送到宿主机
  的 exec-server 执行。用于让 AI agent 能够在容器环境中执行编译、测试、文件操作等任务。
allowed-tools:
  - Bash
  - Read
  - Write
  - Edit
  - Grep
  - Glob
---

## Preamble (run first)

```bash
# 技能初始化：检测安装状态和配置

_EXEC_SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_EXEC_BIN="${_EXEC_SKILL_DIR}/bin/exec-client"
_EXEC_CONFIG="${HOME}/.config/exec-client/config.yaml"

# 检查二进制文件
if [ ! -x "$_EXEC_BIN" ]; then
  echo "STATUS: exec client binary not found or not executable"
  echo "ACTION: Run installation: cp ${_EXEC_SKILL_DIR}/../exec-client ${_EXEC_BIN} 2>/dev/null || make install-exec-skill"
  echo "HELP: See README.md for setup instructions"
else
  echo "STATUS: exec client ready at $_EXEC_BIN"
fi

# 检查配置文件
if [ ! -f "$_EXEC_CONFIG" ]; then
  echo "CONFIG: not found at $_EXEC_CONFIG"
  echo "ACTION: Create config or run: make exec-skill-setup"
else
  echo "CONFIG: found at $_EXEC_CONFIG"
  # 尝试提取 server URL（如果已安装 yq）
  if command -v yq &>/dev/null; then
    _SERVER_URL=$(yq eval '.server_url // ""' "$_EXEC_CONFIG" 2>/dev/null)
    [ -n "$_SERVER_URL" ] && echo "SERVER: $_SERVER_URL"
  fi
fi

# 检查服务器可访问性 (不阻塞执行)
if [ -f "$_EXEC_CONFIG" ] && command -v curl &>/dev/null; then
  if command -v yq &>/dev/null; then
    _SERVER_URL=$(yq eval '.server_url // ""' "$_EXEC_CONFIG" 2>/dev/null)
    if [ -n "$_SERVER_URL" ]; then
      _HEALTH_URL="${_SERVER_URL%/}/health"
      if curl -s --max-time 2 "$_HEALTH_URL" &>/dev/null; then
        echo "SERVER-HEALTH: reachable"
      else
        echo "SERVER-HEALTH: unreachable (check exec-server is running)"
      fi
    fi
  fi
fi
```

## 容器命令执行 (/exec)

使用 `/exec` 技能在远程容器宿主机上执行命令。适用于构建、测试、文件操作、服务管理等场景。

### 基本用法

```bash
# 执行简单命令
/exec command="ls" cwd="/home/user"

# 带参数 (逗号分隔)
/exec command="go" args="test,-v" cwd="/app"

# 使用完整 JSON 数组格式的参数
/exec command="python" args='["-m","pytest","tests/"]' cwd="/app"

# 设置超时 (秒)
/exec command="make" cwd="/build" timeout=600

# 设置环境变量
/exec command="npm" args="test" env='{"NODE_ENV":"test","DEBUG":"*"}'
```

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| command | string | 是 | 要执行的命令名 (如 "ls", "go", "make") |
| args | string | 否 | 参数列表，支持逗号分隔的字符串或 JSON 数组 |
| cwd | string | 否 | 工作目录，默认：`/` (容器根目录) |
| env | string | 否 | 环境变量，JSON 格式对象，如 `{"KEY":"val"}` |
| timeout | int | 否 | 超时秒数，默认：300 (5分钟) |

### 返回结果

执行成功后返回以下字段：
- `stdout` - 标准输出内容
- `stderr` - 标准错误输出
- `exit_code` - 退出码（0表示成功）
- `duration_ms` - 执行时间（毫秒）
- `truncated` - 输出是否被截断（超过10MB）

错误时返回 `error` 和 `message`：
- `unauthorized` - Token认证失败
- `forbidden` - 命令不在白名单或路径不允许
- `timeout` - 执行超时
- `invalid_request` - 请求参数错误
- `internal_error` - 服务器内部错误

### 示例工作流

**Go 项目测试与构建：**
```bash
# 运行测试
/exec command="go" args="test,./...,-v,-race" cwd="/app" timeout=120
# 构建二进制
/exec command="go" args="build,-o,app" cwd="/app"
```

**Node.js 项目：**
```bash
# 安装依赖
/exec command="npm" args="ci" cwd="/app"
# 运行构建
/exec command="npm" args="run,build" cwd="/app" timeout=300
# 运行测试
/exec command="npm" args="test,-,-ci" cwd="/app"
```

**Python 项目：**
```bash
# 安装依赖
/exec command="pip" args="install,-r,requirements.txt" cwd="/app"
# 运行测试
/exec command="pytest" args="-v,tests/," cwd="/app"
```

**文件操作与检查：**
```bash
# 列出文件
/exec command="ls" args="-la" cwd="/app"
# 查看文件
/exec command="cat" args="README.md" cwd="/app"
# 搜索文件
/exec command="find" args=".,-name,'*.go'" cwd="/app"
# 检查磁盘使用
/exec command="du" args="-sh,*" cwd="/app"
```

### 安全说明

- ✅ 所有命令 exec-server 会进行白名单检查
- ✅ 工作目录必须位于 `allowed_paths` 配置中
- ✅ 支持命令注入防护（禁止 shell 元字符）
- ✅ 所有执行记录审计日志
- ✅ 输出限制 10MB（可配置）
- ✅ 默认超时 300 秒（可调整）

### 常见问题

**Q: /exec 命令返回 "exec client binary not found"**
A: 需要先安装 skill。运行 `make install-exec-skill` 或 `make exec-skill-setup`。

**Q: 命令返回 "forbidden - command not in whitelist"**
A: exec-server 的白名单需要包含该命令。编辑 server 配置文件并重载（5秒自动重载）。

**Q: 连接服务器失败或不可达**
A: 检查配置文件 `~/.config/exec-client/config.yaml` 中的 `server_url` 是否正确，以及 exec-server 是否在宿主机上运行。

**Q: 输出被截断**
A: 命令输出超过 10MB 会被截断。考虑：
- 增加 server 的 `max_output_mb` 配置
- 使用过滤：`| head -1000` 或重定向到文件
- 分阶段执行

### 依赖

- 宿主机运行 `exec-server` (见 README.md)
- 容器与宿主机网络互通
- 配置文件 `~/.config/exec-client/config.yaml`

### 安装与设置

#### 自动安装（推荐）

```bash
# 从项目根目录运行
make install-exec-skill
```

这会：
1. 将项目中的 exec-client 二进制复制到 skill 目录
2. 检查配置文件是否存在
3. 如果不存在，提示使用 `make exec-skill-setup` 创建配置

#### 手动安装

```bash
# 1. 复制二进制（如果未自动完成）
cp exec-client .claude/skills/exec/bin/

# 2. 创建配置文件
mkdir -p ~/.config/exec-client
cat > ~/.config/exec-client/config.yaml <<EOF
server_url: "http://localhost:8080"  # 改为宿主机实际IP
api_token: "your-token-from-server-config"
timeout_seconds: 35
EOF
```

#### 设置向导

```bash
# 运行交互式设置脚本
make exec-skill-setup
# 或直接运行脚本
./scripts/setup-exec-skill.sh
```

脚本会引导你完成配置。

#### 验证安装

```bash
# 测试连接
/exec command="echo" args="Hello from container!" cwd="/"
```

### 技能路由（自动触发）

要让 Claude 自动使用此技能，在项目 `CLAUDE.md` 中添加：

```markdown
## Skill routing

当用户请求匹配可用技能时，始终使用 Skill 工具调用它作为第一个动作。

关键路由规则：
- 需要运行命令、查看文件、执行构建等操作 → 调用 /exec
```

---

## 技术细节

### 二进制位置
- **Skill 包内**: `.claude/skills/exec/bin/exec-client`
- **用户配置**: `~/.config/exec-client/config.yaml`
- **全局安装**: 可选，用户可将二进制放到 `~/.local/bin/exec-client`

### 配置文件格式

```yaml
server_url: "http://<宿主机IP>:8080"  # 必填，宿主机 exec-server 地址
api_token: "a1b2c3d4..."              # 必填，从 server 配置文件获取
timeout_seconds: 35                   # 可选，建议比 server 的 timeout 大 5
```

### 与 server 的通信

技能通过 HTTP POST `/exec` 与 exec-server 通信：
- Content-Type: application/json
- Authorization: Bearer {api_token}
- Body: `{"command":"...", "args":[...], "cwd":"..."}`

---

## 开发与调试

### 本地测试

```bash
# 在宿主机启动 server
./exec-server --config ~/.config/exec-server/config.yaml

# 在容器内测试 client（模拟）
./.claude/skills/exec/bin/exec-client -server http://localhost:8080 -token YOUR_TOKEN -command "ls" -cwd "/app"
```

### 更新技能

当项目更新时：
```bash
# 重新复制二进制
make install-exec-skill

# 检查新版本的 preamble
/exec --dry-run  # (如果支持)
```

---

## 完整示例会话

```bash
# 用户说："在容器里运行 go test ./..."
# Claude 自动使用 /exec 技能：

/exec command="go" args="test,./...,-v" cwd="/app"

# 输出：
# stdout: ok  	github.com/user/myapp	0.123s
# stderr:
# exit_code: 0
# duration_ms: 456
# truncated: false
```

---

## Version History

- **1.0.0** (2026-04-02): Initial release, project-embedded skill with auto-detect preamble

---

## License

MIT License - Same as the parent project.
