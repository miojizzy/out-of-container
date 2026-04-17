## API 参考

### 端点列表

#### 1. 健康检查

```
GET /health
```

**响应**：
```
Status: 200 OK
Content-Type: text/plain
```

**返回**：纯文本 `OK`

---

#### 2. 命令执行

```
POST /ooc-exec
```

**认证**：Bearer Token，通过 `Authorization: Bearer <token>` 头部提供

**请求体**：

```json
{
  "command": "ls",
  "args": ["-la"],
  "cwd": "/tmp"
}
```

| 字段 | 类型 | 必填 | 描述 |
|------|------|------|------|
| `command` | string | 是 | 要执行的命令（必须在白名单中） |
| `args` | array[string] | 否 | 命令参数 |
| `cwd` | string | 是 | 工作目录（必须在允许的路径列表中） |

**成功响应** (200 OK)：

```json
{
  "exit_code": 0,
  "stdout": "file1\nfile2\n",
  "stderr": "",
  "duration_ms": 150,
  "truncated": false
}
```

| 字段 | 类型 | 描述 |
|------|------|------|
| `exit_code` | int | 命令退出码 |
| `stdout` | string | 标准输出 |
| `stderr` | string | 标准错误 |
| `duration_ms` | int64 | 执行耗时（毫秒） |
| `truncated` | bool | 输出是否被截断 |

**错误响应**：

- `400 Bad Request`：请求体无效或缺少必填字段
- `403 Forbidden`：命令不在白名单 或工作目录不允许
- `408 Request Timeout`：命令执行超时
- `500 Internal Server Error`：执行失败

**响应头**：
- `X-Output-Truncated: true`（如果输出被截断）
- `X-Output-Size-Bytes: <size>`（输出的实际字节数）

---

#### 3. 白名单信息发现

```
GET /whitelist-info
```

**认证**：Bearer Token

**响应** (200 OK)：

```json
{
  "literal_commands": ["ls", "pwd", "echo"],
  "regex_commands": ["^git (clone|pull|fetch)", "^go (build|test|mod)"],
  "allowed_paths": ["/tmp", "/home"],
  "reload_interval_seconds": 5
}
```

| 字段 | 类型 | 描述 |
|------|------|------|
| `literal_commands` | array[string] | 字面量命令白名单 |
| `regex_commands` | array[string] | 正则表达式命令白名单 |
| `allowed_paths` | array[string] | 允许的工作目录路径 |
| `reload_interval_seconds` | int | 配置重载间隔（秒） |

---

#### 4. 异步任务提交（Phase 2）

```
POST /task
```

**认证**：Bearer Token

**请求体**：

```json
{
  "command": "go build",
  "args": ["-v"],
  "cwd": "/home/user/project"
}
```

**成功响应** (202 Accepted)：

```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "message": "task submitted successfully",
  "created_at": "2026-04-17T01:30:00Z"
}
```

---

#### 5. 任务状态查询（Phase 2）

```
GET /task/{task_id}
```

**响应** (200 OK)：

```json
{
  "task_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "created_at": "2026-04-17T01:30:00Z",
  "started_at": "2026-04-17T01:30:01Z",
  "completed_at": "2026-04-17T01:30:05Z",
  "duration_ms": 4000,
  "exit_code": 0,
  "stdout": "build successful",
  "stderr": "",
  "output_size": 4096,
  "truncated": false
}
```

**任务状态值**：
- `pending`：任务已提交，等待执行
- `running`：任务正在执行
- `completed`：任务已完成（无论退出码）
- `failed`：任务执行失败（内部错误）

---

### 错误码参考

| HTTP 状态码 | 错误码 | 说明 |
|-------------|--------|------|
| 400 | invalid_request | 请求参数无效 |
| 403 | forbidden | 命令或路径不在白名单 |
| 403 | authentication_failed | API Token 无效 |
| 404 | not_found | 任务不存在 |
| 408 | timeout | 执行超时 |
| 429 | too_many_requests | 并发数已达上限 |
| 500 | execution_failed | 命令执行失败 |
| 500 | internal_error | 服务器内部错误 |

---

### 客户端示例

使用 `ooc-client` 命令行工具：

```bash
# 执行命令
ooc-client -server http://localhost:8080 \
  -token your-api-token \
  -command "ls" \
  -args "-la" \
  -cwd "/tmp"

# 查看允许的命令
ooc-client -server http://localhost:8080 \
  -token your-api-token \
  -list-commands

# 查看允许的路径
ooc-client -server http://localhost:8080 \
  -token your-api-token \
  -list-paths
```

配置文件：`~/.config/ooc-client/config.yaml`

```yaml
server_url: "http://localhost:8080"
api_token: "your-api-token"
timeout_seconds: 30
```

---

### 服务器端配置

配置文件：`~/.config/ooc-server/config.yaml`

```yaml
server:
  listen: ":8080"
  timeout_seconds: 30         # 命令执行超时时间
  max_output_mb: 10           # 最大输出量（MB），超过截断
  max_concurrent: 5           # 最大并发执行数
  api_token: "your-secret-token"
  task_ttl_hours: 24          # 任务保留时间（Phase 2）

whitelist:
  literal_commands:
    - "ls"
    - "pwd"
  regex_commands:
    - "^git (clone|pull|fetch)"
  allowed_paths:
    - "/tmp"
    - "/home"
  reload_interval_seconds: 5  # 配置文件重载间隔

audit:
  enabled: true
  log_file: "audit.log"
  rotation_max_mb: 100
  rotation_count: 5
```

<!-- AUTO-GENERATED -->