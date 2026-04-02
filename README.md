# exec-server

容器远程命令执行系统 - 让容器内的 AI agent 能够在宿主机上执行编译、测试等命令。

## 问题背景

在容器化开发环境中，AI agent 可以读写挂载的代码目录，但无法直接在宿主机上执行命令（编译、测试等）。当前需要手动退出容器执行命令，打断自动化流程。

本工具提供一个 server-client 系统：
- **Server** 运行在宿主机上，接收命令请求，执行并返回结果
- **Client** 在容器内运行，提供 CLI 接口供 agent 调用

## 特性

### 安全性
- ✅ API Token 认证（crypto/rand 生成，constant-time 比较）
- ✅ 白名单控制（字面匹配 + 正则匹配）
- ✅ 工作目录限制（防止路径跳转攻击）
- ✅ Shell 元字符检测（防止命令注入）
- ✅ 审计日志（JSONL 格式，自动轮转）

### 性能与可靠性
- ✅ 并发控制（最大 5 个并发执行）
- ✅ 超时控制（30 秒硬编码超时）
- ✅ 输出大小限制（10MB，防止内存耗尽）
- ✅ 进程组清理（超时后 kill 整个进程组）
- ✅ 配置热重载（5 秒检查间隔，RWMutex 保护）

### 易用性
- ✅ 单二进制文件部署（Go 静态编译）
- ✅ 配置文件外部化（支持 `exec-server init` 生成）
- ✅ 兼容老系统（支持 CentOS 7 / glibc 2.17+）

## 快速开始

### 1. 安装

从 [GitHub Releases](https://github.com/user/exec-server/releases) 下载对应平台的二进制文件：

```bash
# Linux x86_64
curl -L -o exec-server https://github.com/user/exec-server/releases/latest/download/exec-server-linux-amd64
chmod +x exec-server

curl -L -o exec-client https://github.com/user/exec-server/releases/latest/download/exec-client-linux-amd64
chmod +x exec-client
```

### 2. 初始化配置

```bash
# 在宿主机上生成配置文件
./exec-server --init

# 输出示例：
# Config file created: /home/user/.config/exec-server/config.yaml
# API Token: a1b2c3d4e5f6...
#
# Please edit the config file to customize:
#   - literal_commands: Add your allowed commands
#   - allowed_paths: Set your project directories
```

### 3. 配置白名单

编辑 `~/.config/exec-server/config.yaml`：

```yaml
whitelist:
  literal_commands:
    - "make"
    - "cmake"
    - "g++"
    - "gcc"
    - "python3"
    - "pytest"

  allowed_paths:
    - "/home/user/projects"
    - "/tmp/build"
```

### 4. 启动 Server

```bash
./exec-server --config ~/.config/exec-server/config.yaml

# 输出：
# Server starting on 0.0.0.0:8080
```

### 5. 配置 Client

在容器内创建配置文件 `~/.config/exec-client/config.yaml`：

```yaml
server_url: "http://<宿主机IP>:8080"
api_token: "a1b2c3d4e5f6..."  # 从 server 配置复制
timeout_seconds: 35
```

### 6. 执行命令

```bash
# 基本用法
./exec-client -command make -cwd /home/user/projects

# 带参数
./exec-client -command g++ -args '"-std=c++17","main.cpp"' -cwd /home/user/projects

# 使用配置文件中的 server URL 和 token
./exec-client -command pytest -cwd /home/user/projects

# 覆盖配置
./exec-client -server http://localhost:8080 -token your-token -command echo -cwd /tmp
```

## 配置说明

### Server 配置

配置文件路径：`~/.config/exec-server/config.yaml`

```yaml
server:
  listen: "0.0.0.0:8080"        # 监听地址
  timeout_seconds: 30             # 命令执行超时（秒）
  max_output_mb: 10               # 输出大小限制（MB）
  max_concurrent: 5               # 最大并发执行数
  api_token: "..."                # API Token（由 init 命令生成）

whitelist:
  literal_commands:               # 字面匹配（完全匹配命令名）
    - "make"
    - "g++"

  regex_commands:                 # 正则匹配（注意转义）
    - "^pip\\s+install"

  allowed_paths:                  # 允许的工作目录
    - "/home/user/projects"

  reload_interval_seconds: 5      # 配置热重载间隔

audit:
  enabled: true                   # 启用审计日志
  log_file: "~/.local/share/exec-server/audit.log"
  rotation_max_mb: 10             # 单个日志文件最大大小
  rotation_count: 10              # 保留的日志文件数量
```

### Client 配置

配置文件路径：`~/.config/exec-client/config.yaml`

```yaml
server_url: "http://localhost:8080"
api_token: "your-api-token-here"
timeout_seconds: 35               # 比 server 略大，避免网络延迟误判
```

## 安全性说明

### 威胁模型

**假设前提：**
- 宿主机已受保护（物理访问、操作系统安全）
- 配置文件权限正确（600，仅 owner 可读写）
- Token 仅在可信环境内传递

**防御措施：**

1. **认证**
   - Token 使用 `crypto/rand` 生成（不可预测）
   - 使用 `crypto/subtle.ConstantTimeCompare` 验证（防止时序攻击）
   - Token 每次启动不会自动轮换（需手动更换）

2. **授权**
   - 白名单机制：只有明确允许的命令可以执行
   - 路径限制：只能指定目录下的工作目录
   - 符号链接解析：防止 `..` 跳出允许范围

3. **命令注入防护**
   - 禁止 shell 元字符：`|`, `&`, `;`, `$`, `` ` ``, `<`, `>`, `(`, `)`
   - 使用 `exec.Command(command, args...)` 形式（非 `bash -c` 形式）

4. **资源限制**
   - 并发控制：防止资源耗尽
   - 超时控制：防止长时间占用
   - 输出限制：防止内存耗尽

5. **审计**
   - 所有执行记录审计日志
   - Token 前缀记录（前 8 位，用于追踪）
   - 日志文件权限 600

### 安全建议

1. **定期轮换 Token**
   ```bash
   # 生成新 token
   openssl rand -hex 32

   # 更新配置文件
   vim ~/.config/exec-server/config.yaml

   # 重启 server
   pkill exec-server
   ./exec-server --config ~/.config/exec-server/config.yaml
   ```

2. **限制网络访问**
   - 使用防火墙规则限制 8080 端口访问
   - 或修改 `listen` 为 `127.0.0.1:8080`（仅本地访问）

3. **最小权限原则**
   - 白名单只添加必需的命令
   - `allowed_paths` 只包含必要的目录

4. **监控审计日志**
   ```bash
   # 查看最近的执行记录
   tail -f ~/.local/share/exec-server/audit.log | jq .

   # 统计命令执行频率
   cat ~/.local/share/exec-server/audit.log | jq -r '.command' | sort | uniq -c
   ```

## 架构设计

详见 `design/zhaozeyu-main-design-20260401-050215.md`。

### 核心组件

```
cmd/server/main.go         # Server 入口
├── pkg/config/loader.go    # 配置加载器
├── internal/
│   ├── auth/               # Token 认证中间件
│   ├── validation/         # Shell 元字符检测
│   ├── concurrency/        # 并发控制（Semaphore）
│   ├── executor/           # 命令执行器
│   ├── whitelist/          # 白名单检查器
│   ├── auditor/            # 审计日志
│   ├── handlers/           # HTTP handlers
│   └── models/             # 数据结构

cmd/client/main.go         # Client 入口
```

### 数据流

```
Client Request
    ↓
TokenAuth Middleware (验证 token)
    ↓
ConcurrencyLimiter Middleware (检查并发数)
    ↓
WhitelistChecker (检查命令和白名单)
    ↓
Executor (执行命令，超时控制，输出限制)
    ↓
Auditor (记录审计日志)
    ↓
Response (返回结果)
```

## 开发

### 构建

```bash
# 构建 server 和 client
make build

# 或手动构建
go build -o exec-server ./cmd/server
go build -o exec-client ./cmd/client
```

### 测试

```bash
# 运行所有测试
make test

# 或手动运行
go test -v ./...
```

### 代码结构

遵循 Go 标准项目布局：
- `cmd/` - 可执行文件入口
- `internal/` - 私有库代码
- `pkg/` - 公共库代码（可被外部引用）
- `config/` - 配置文件示例

## Claude Code 技能集成

本项目提供了一个 [Claude Code](https://claude.ai/code) 技能，让 AI agent 能够直接在容器中执行命令。

### 安装技能

#### 自动安装（推荐）

```bash
# 从项目根目录运行
make install-exec-skill
```

这会将 `exec-client` 二进制文件安装到 `.claude/skills/exec/bin/` 目录。

#### 交互式配置向导

```bash
# 运行交互式设置脚本
make exec-skill-setup
# 或直接运行脚本
./scripts/setup-exec-skill.sh
```

脚本会引导你完成：
1. 配置服务器 URL 和 API Token
2. 验证连接
3. 测试技能

### 使用技能

在 Claude Code 中，你可以直接使用 `/exec` 命令：

```bash
# 执行简单命令
/exec command="ls" cwd="/home/user"

# 带参数的命令
/exec command="go" args="test,-v" cwd="/app"

# 复杂参数（JSON 数组格式）
/exec command="python" args='["-m","pytest","tests/"]' cwd="/app"
```

### 技能路由

为了让 Claude 自动使用此技能，在项目 `CLAUDE.md` 中添加：

```markdown
## Skill routing

当用户请求匹配可用技能时，始终使用 Skill 工具调用它作为第一个动作。

关键路由规则：
- 需要运行命令、查看文件、执行构建等操作 → 调用 /exec
```

## 故障排查

### Server 无法启动

**错误：** `listen tcp :8080: bind: address already in use`

**解决：**
```bash
# 查找占用端口的进程
lsof -i :8080

# 修改配置文件中的端口
vim ~/.config/exec-server/config.yaml
# server.listen: "0.0.0.0:8081"
```

### Client 认证失败

**错误：** `Error: unauthorized - invalid API token`

**解决：**
1. 检查 client 配置文件中的 `api_token` 是否正确
2. 确保 token 没有 trailing whitespace
3. 重启 server 后 token 未更新

### 命令被拒绝

**错误：** `Error: forbidden - command not in whitelist`

**解决：**
```bash
# 检查当前白名单配置
cat ~/.config/exec-server/config.yaml | grep -A 10 whitelist

# 添加需要的命令
vim ~/.config/exec-server/config.yaml

# 等待 5 秒让配置自动重载，或重启 server
```

### 命令超时

**错误：** `Error: timeout - command exceeded 30s timeout`

**解决：**
- 短命令：检查命令是否卡住
- 长命令：使用 Phase 2 的异步模式（开发中）

## 后续计划

详见 `TODOS.md`。

**Phase 2（计划中）：**
- 异步执行模式（`/exec/async` + `/exec/status`）
- 任务队列（LRU 淘汰）
- Client 轮询支持

**Phase 3（计划中）：**
- SSE 流式输出（`/exec/stream`）
- 实时查看命令执行过程

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License

---

**Built with ❤️ for AI-assisted development**
