## 运维手册

本文档提供服务器部署、监控、故障处理和维护操作的详细说明。

### 1. 初始部署

#### 1.1 生成默认配置

```bash
# 构建并初始化配置
make build
make init
```

这将在 `~/.config/ooc-server/` 目录下生成 `config.yaml`。

#### 1.2 编辑配置文件

修改 `~/.config/ooc-server/config.yaml`：

```yaml
server:
  listen: ":8080"                    # 监听地址
  timeout_seconds: 30                # 命令超时时间
  max_output_mb: 10                  # 最大输出量
  max_concurrent: 5                  # 并发限制
  api_token: "CHANGE_THIS"           # API 认证密钥
  task_ttl_hours: 24                 # 任务保留时间

whitelist:
  literal_commands:
    - "ls"
    - "pwd"
    - "echo"
  regex_commands:
    - "^git (clone|pull|fetch)"
  allowed_paths:
    - "/tmp"
    - "/some/allowed/path"
  reload_interval_seconds: 5

audit:
  enabled: true
  log_file: "/var/log/ooc-server/audit.log"
  rotation_max_mb: 100
  rotation_count: 5
```

#### 1.3 安装服务

**使用 systemd**：

```bash
sudo cp deployment/ooc-server.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable ooc-server
sudo systemctl start ooc-server
sudo systemctl status ooc-server
```

**手动运行**：

```bash
make run-server
```

---

### 2. 健康检查

#### 2.1 快速检查

```bash
curl http://localhost:8080/health
# 应返回 "OK"
```

#### 2.2 检查服务状态

```bash
# systemd
sudo systemctl status ooc-server

# 查看日志
sudo journalctl -u ooc-server -f
```

#### 2.3 检查配置文件

```bash
# 验证配置文件语法
make build
./ooc-server --config ~/.config/ooc-server/config.yaml
# 如果无错误输出，配置有效
```

---

### 3. 白名单管理

#### 3.1 查看当前白名单

```bash
# 使用客户端
ooc-client -server http://localhost:8080 \
  -token your-api-token \
  -list-commands

ooc-client -server http://localhost:8080 \
  -token your-api-token \
  -list-paths
```

#### 3.2 动态重载配置

配置文件会自动每 `reload_interval_seconds` 秒检查并重载。如果想立即生效：

```bash
# 修改配置文件后发送 SIGHUP 信号
sudo systemctl reload ooc-server
```

#### 3.3 添加新命令

编辑 `~/.config/ooc-server/config.yaml`：

```yaml
whitelist:
  literal_commands:
    - "ls"
    - "pwd"
    - "echo"
    - "docker"           # 添加新命令
  regex_commands:
    - "^git (clone|pull|fetch)"
  allowed_paths:
    - "/tmp"
    - "/var/lib/docker"  # 添加新路径
```

保存后，系统会在 5 秒内自动重载。

---

### 4. 监控指标

#### 4.1 审计日志

所有执行记录都会记录到审计日志。查看最近记录：

```bash
tail -f /var/log/ooc-server/audit.log
```

日志格式（JSON）：
```json
{
  "timestamp": "2026-04-17T01:30:00Z",
  "command": "ls",
  "args": ["-la"],
  "cwd": "/tmp",
  "token_prefix": "abc123...",
  "exit_code": 0,
  "duration_ms": 150,
  "output_size_bytes": 1024,
  "truncated": false,
  "allowed_by": "literal"
}
```

#### 4.2 日志轮转

日志会自动轮转，默认配置：
- 最大文件：100 MB
- 保留文件数：5
- 自动压缩

#### 4.3 检查并发数

```bash
# 查看当前并发执行的任务数
# 服务器端暂无内置指标，但可观察：
# 1. 系统进程数
ps aux | grep ooc-server

# 2. 审计日志的间隔时间快速判断负载
```

**Phase 2 将提供 Prometheus 指标端点**。

---

### 5. 常见故障处理

#### 5.1 服务器启动失败

**症状**：`systemctl status ooc-server` 显示 failed

**排查**：

```bash
# 1. 查看详细日志
sudo journalctl -u ooc-server -n 50

# 2. 检查配置文件是否存在
ls -la ~/.config/ooc-server/config.yaml

# 3. 手动运行查看错误
./ooc-server --config ~/.config/ooc-server/config.yaml
```

**常见原因**：
- 配置文件路径错误
- 配置文件语法错误（YAML 格式）
- 端口被占用：`netstat -tulpn | grep :8080`
- 权限不足：验证日志目录写入权限

---

#### 5.2 命令执行被拒绝（403）

**症状**：客户端收到 `"command not in whitelist"` 或 `"cwd not allowed"`

**处理**：

```bash
# 1. 验证白名单配置
ooc-client -server http://localhost:8080 \
  -token your-token \
  -list-commands

# 2. 检查请求的 cwd 是否在允许列表中
ooc-client -server http://localhost:8080 \
  -token your-token \
  -list-paths

# 3. 修改配置文件，添加需要的命令和路径
vi ~/.config/ooc-server/config.yaml

# 4. 重载配置
sudo systemctl reload ooc-server
```

---

#### 5.3 执行超时（408）

**症状**：客户端收到超时错误

**原因**：命令执行时间超过 `server.timeout_seconds`

**处理**：

1. **临时增加超时**：
   ```bash
   vi ~/.config/ooc-server/config.yaml
   # server.timeout_seconds: 300  # 改为 5 分钟
   sudo systemctl reload ooc-server
   ```

2. **优化命令**：将长时间任务转为异步（Phase 2）

---

#### 5.4 输出被截断

**症状**：`stdout` 或 `stderr` 不完整，响应头有 `X-Output-Truncated: true`

**原因**：输出超过 `server.max_output_mb` 限制（默认 10MB）

**处理**：

1. **增加限制**：
   ```yaml
   server:
     max_output_mb: 100  # 增加到 100MB
   ```

2. **优化命令**：减少输出量，或使用文件代替标准输出

---

#### 5.5 并发限制报错

**症状**：多个请求同时失败，日志显示并发限制

**原因**：同时执行的任务数超过 `server.max_concurrent`

**处理**：

1. **增加并发数**：
   ```yaml
   server:
     max_concurrent: 20
   ```

2. **使用异步任务（Phase 2）**：
   提交任务后查询结果，避免同步等待

---

#### 5.6 配置文件未自动重载

**症状**：修改了配置但没生效

**排查**：

```bash
# 1. 检查配置文件修改时间
ls -la ~/.config/ooc-server/config.yaml

# 2. 检查进程是否读取新配置
# 发送 SIGHUP 强制重载
sudo systemctl reload ooc-server

# 3. 查看服务器日志确认重载
sudo journalctl -u ooc-server | grep "reload"
```

---

### 6. 性能调优

#### 6.1 调整并发数

```yaml
server:
  max_concurrent: 10  # 根据 CPU 核心数调整，通常 = CPU 核心数 * 1.5
```

监控：
```bash
top -p $(pgrep ooc-server)
```

#### 6.2 调整栈空间

默认栈空间可能不够，可调整：

```bash
ulimit -s 8192  # 8MB stack
```

---

#### 6.3 日志性能

如果审计日志 I/O 成为瓶颈：

1. 使用 SSD 盘
2. 将日志文件放到独立磁盘
3. 考虑关闭审计（仅测试环境）：
   ```yaml
   audit:
     enabled: false
   ```

---

### 7. 安全建议

#### 7.1 API Token 管理

- 务必更改默认 `api_token`
- 使用强随机字符串：`openssl rand -hex 32`
- 定期轮换 Token
- 不要将 Token 提交到代码仓库

#### 7.2 白名单原则

- 默认拒绝所有：白名单为空意味着所有命令拒绝
- 最小权限：仅添加确实需要的命令和路径
- 定期审查：每个季度检查白名单有效性
- 使用正则：`^go test` 更安全，避免 `go`（可能被滥用）

#### 7.3 路径限制

- 尽量限制到特定项目目录
- 避免使用 `/` 根目录
- 使用符号链接真实路径：Checker 会自动解析 symlink

#### 7.4 网络隔离

- 将服务监听在 `127.0.0.1:8080`（仅本地）
- 如需外部访问，使用反向代理 + TLS：
  ```nginx
  location /ooc-exec {
      proxy_pass http://127.0.0.1:8080;
      proxy_set_header Authorization "Bearer $http_authorization";
  }
  ```

---

### 8. 备份与恢复

#### 8.1 备份内容

```bash
# 1. 配置文件
cp ~/.config/ooc-server/config.yaml /backup/ooc-server/

# 2. 审计日志（可选）
cp /var/log/ooc-server/audit.log /backup/ooc-server/
```

#### 8.2 恢复

```bash
cp /backup/ooc-server/config.yaml ~/.config/ooc-server/
sudo systemctl reload ooc-server
```

---

### 9. 升级流程

1. **准备**：备份配置文件和审计日志
2. **编译**：`make build-linux` 生成新二进制
3. **停止服务**：`sudo systemctl stop ooc-server`
4. **替换二进制**：`sudo cp ooc-server /usr/local/bin/`
5. **启动服务**：`sudo systemctl start ooc-server`
6. **验证**：`curl http://localhost:8080/health`
7. **回滚**（如失败）：恢复旧版本二进制并重启

---

### 10. 调试技巧

#### 10.1 启用详细日志

在代码中添加 `log.SetFlags(log.Ltime | log.Lmicroseconds)` 和 `log.SetLevel(log.DebugLevel)`（使用 zap 或 logrus 后）。

#### 10.2 检查 Unix 时间戳

```bash
date -d @$(stat -c %Y ~/.config/ooc-server/config.yaml)
# 查看配置文件最后修改时间
```

#### 10.3 追踪命令执行

```bash
# 使用 strace 追踪系统调用
sudo strace -p $(pgrep ooc-server) 2>&1 | grep execve
```

---

### 11. 告警规则

建议使用 Prometheus + Alertmanager 配置以下告警：

- `ooc_server_down`：`up{job="ooc-server"} == 0`
- `ooc_concurrent_tasks_high`：并发数 > 90% 阈值持续 5 分钟
- `ooc_audit_log_disk_full`：日志磁盘使用率 > 85%

**Phase 2 将提供内置指标端点**。

---

### 12. 联系与支持

- 项目仓库：`github.com/user/exec-server`
- 问题反馈：在仓库创建 Issue
- 安全漏洞：请通过安全渠道报告

<!-- AUTO-GENERATED -->