# TODO
Project: out-of-container (容器远程命令执行系统)

---

## Phase 2: 异步执行模式

### /ooc-exec/async API

**What:** 实现异步任务提交和查询接口，解决长命令超时问题。

**Why:** Phase 1 的 30 秒超时无法满足长编译、测试套件等耗时操作。

**Pros:**
- 长命令不再被超时中断
- Client 可以轮询任务状态
- 任务队列支持并发管理

**Cons:**
- 增加复杂度（任务状态管理）
- 需要处理任务清理（LRU 淘汰）

**Context:**
- 新增 API: `/ooc-exec/async` (提交任务，返回 task_id)
- 新增 API: `/ooc-exec/status?id=xxx` (查询任务状态)
- 实现内存任务队列，最多 100 个任务（LRU 淘汰）
- Client 支持轮询模式

**Depends on / blocked by:**
- Phase 1 完成（Executor、WhitelistChecker、Auditor 接口已有）

---

## Phase 3: 流式输出

### SSE Real-time Streaming

**What:** 实现 SSE (Server-Sent Events) 接口，实时流式返回命令输出。

**Why:** 某些场景需要实时看到命令执行过程（如编译进度、测试输出）。

**Pros:**
- 实时反馈，提升用户体验
- 复用 Phase 2 的 task_id 机制

**Cons:**
- 增加连接管理复杂度
- 需要处理连接断开和重连

**Context:**
- 新增 API: `/ooc-exec/stream?id=xxx`
- HTTP handler 检查 `Accept: text/event-stream` 头
- 复用 `MemoryTaskManager` 的任务状态

**Depends on / blocked by:**
- Phase 2 完成 (task_id 机制)

---

## Security Enhancements

### Token Rotation

**What:** 实现 token 轮换命令，允许用户定期更换 API Token。

**Why:** Phase 1 token 是明文存储在配置文件，如果 token 泄露需要手动更换，缺少自动化工具。

**Pros:**
- 自动化 token 更换流程
- 支持紧急 token 撤销

**Cons:**
- 增加配置管理复杂度
- 需要处理 token 更换期间的服务可用性

**Context:**
- 新增命令: `ooc-server rotate-token --config ~/.config/ooc-server/config.yaml`
- 生成新 token，更新配置文件
- 旧 token 在 5 分钟内仍然有效（平滑过渡）

**Depends on / blocked by:**
- Phase 1 完成

---

### Environment Variable Whitelist (Phase 1.5)

**What:** 支持用户定义允许的环境变量白名单，在命令执行时注入。

**Why:** Phase 1 完全禁止自定义环境变量，某些场景需要设置构建环境（如 `BUILD_TYPE=Release`）。

**Pros:**
- 满足常见构建场景需求
- 白名单机制保证安全性

**Cons:**
- 增加配置复杂度
- 环境变量注入存在安全风险

**Context:**
- 在配置文件中添加 `allowed_env` 字段：
  ```yaml
  allowed_env:
      - "BUILD_TYPE"
      - "CC"
      - "CXX"
  ```
- 请求中的 `env` 字段必须匹配白名单，否则返回 400

**Depends on / blocked by:**
- Phase 1 完成

---

### Path-based Whitelist Rules

**What:** 支持基于工作目录的细粒度白名单规则，不同目录允许不同命令。

**Why:** 当前白名单是全局的，无法区分"生产环境目录"和"测试目录"的权限差异。

**Pros:**
- 细粒度权限控制
- 满足复杂环境的安全需求

**Cons:**
- 增加配置和实现复杂度
- 降低配置文件的可读性

**Context:**
- 在配置文件中添加 `path_rules` 字段：
  ```yaml
  path_rules:
      - path: "/home/user/projects/production"
        commands: ["make", "gcc"]
      - path: "/home/user/projects/testing"
        commands: ["make", "gcc", "pytest", "rm"]
  ```

**Depends on / blocked by:**
- Phase 1 完成

---

## Platform Support

### Windows Support

**What:** 在 Windows 平台上实现 server 和 client。

**Why:** Phase 1 仅支持 Linux (CentOS 7)，Windows 用户无法使用。

**Pros:**
- 扩大用户群体
- Windows 是常见的开发环境

**Cons:**
- 进程管理 API 差异大（`SysProcAttr` 不存在）
- 文件路径分隔符差异（`\` vs `/`）
- 需要额外的测试和维护成本

**Context:**
- 适配 `os/exec` 的差异：Windows 上没有进程组概念
- 适配文件路径：`filepath` 包已处理大部分差异，但需要测试
- 构建脚本需要支持 Windows：`GOOS=windows GOARCH=amd64`

**Depends on / blocked by:**
- Phase 1 完成
- 需要明确的需求和优先级

---

### macOS Support

**What:** 在 macOS 平台上实现 server 和 client。

**Why:** macOS 是常见的开发环境，部分团队使用 Mac 工作站。

**Pros:**
- 覆盖更多开发环境
- macOS 是 Unix-like 系统，适配成本低于 Windows

**Cons:**
- 测试和维护成本增加
- 某些 Linux 特性在 macOS 上不可用（如某些文件系统特性）

**Context:**
- `SysProcAttr.Setpgid` 在 macOS 上可用
- 大部分代码可复用
- 构建：`GOOS=darwin GOARCH=amd64`

**Depends on / blocked by:**
- Phase 1 完成
- 需要 macOS 环境进行测试

---

## Monitoring & Observability

### Prometheus Metrics Export

**What:** 实现 Prometheus metrics 端点，暴露运行时指标（命令执行次数、成功率、P99 延迟等）。

**Why:** Phase 1 仅通过审计日志监控，无法实时查看服务健康状态。

**Pros:**
- 实时监控和告警
- 与 Grafana 等工具集成
- 标准化的监控体系

**Cons:**
- 增加依赖（`prometheus/client_golang`）
- 需要配置 metrics 端点

**Context:**
- 新增 API: `/metrics`（Prometheus 格式）
- 指标：
  - `exec_requests_total`（总请求数）
  - `exec_duration_seconds`（执行时长histogram）
  - `exec_concurrent_current`（当前并发数）
  - `exec_errors_total`（错误数）

**Depends on / blocked by:**
- Phase 1 完成

---

### Health Check Endpoint

**What:** 实现 `/health` 端点，返回服务健康状态。

**Why:** 部署系统（如 Kubernetes）需要 health check 来判断服务是否可用。

**Pros:**
- 标准化的健康检查机制
- 支持自动重启和负载均衡

**Cons:**
- 需要定义"健康"的判定标准

**Context:**
- 新增 API: `/health`
- 返回格式：
  ```json
  {
      "status": "ok",
      "uptime_seconds": 1234,
      "version": "v0.1.0"
  }
  ```

**Depends on / blocked by:**
- Phase 1 完成

---

## Documentation

### User Guide

**What:** 编写完整的用户使用指南，包括安装、配置、常见问题排查。

**Why:** 设计文档面向开发者，普通用户需要更友好的使用指南。

**Pros:**
- 降低用户使用门槛
- 减少重复问题咨询

**Cons:**
- 需要持续的文档维护

**Context:**
- 创建 `docs/user-guide.md`
- 内容：
  - 下载和安装（Linux binaries, Homebrew tap?）
  - 配置文件说明
  - 常见使用场景
  - 故障排查
  - FAQ

**Depends on / blocked by:**
- Phase 1 完成

---

## Testing

### Integration Test Suite

**What:** 编写完整的集成测试套件，覆盖所有安全边界和边缘情况。

**Why:** 测试计划（plan-eng-review）识别了 37 个缺失的测试路径，需要补全。

**Pros:**
- 提高代码质量和可靠性
- 回归测试保护

**Cons:**
- 需要额外开发和维护成本

**Context:**
- 参考 `zhaozeyu-main-eng-review-test-plan-20260401-063000.md`
- 补全 P0 和 P1 优先级的测试：
  - Shell 元字符检测
  - 工作目录验证
  - Token 常量时间比较
  - 输出大小限制
  - 进程组清理
  - 配置热重载
  - 并发控制

**Depends on / blocked by:**
- Phase 1 代码实现

---

## CLI Enhancements

### Client Auto-discovery

**What:** Client 支持自动发现 server 地址（如 mDNS/Avahi），无需手动配置 `server_url`。

**Why:** 当前 `server_url` 需要手动配置，增加部署复杂度。

**Pros:**
- 简化配置流程
- 支持动态 IP 环境（如 Docker 内外通信）

**Cons:**
- 增加依赖（mDNS 协议）
- 安全性考虑（如何验证发现的 server 是合法的？）

**Context:**
- 使用 mDNS广播：`_exec-server._tcp.local`
- Client 扫描并询问：`ooc-client discover`
- 显示可用 server 列表

**Depends on / blocked by:**
- Phase 1 完成

---

### Interactive Mode

**What:** Client 支持交互模式，可以连续执行多个命令，保持会话上下文。

**Why:** 当前每次调用 `ooc-client exec` 都是独立请求，无法复用会话（环境变量、工作目录）。

**Pros:**
- 提升使用体验
- 减少重复输入

**Cons:**
- 增加客户端复杂度
- 需要管理会话状态

**Context:**
- 新增命令：`ooc-client shell`
- 交互式提示符：`exec-host> `
- 命令：
  - `setwd /path/to/dir`（设置工作目录）
  - `setenv KEY=VALUE`（设置环境变量）
  - `make`（执行命令）

**Depends on / blocked by:**
- Phase 1 完成
- Token 轮换（可选，交互模式会保持 token）

---

## Distribution

### Homebrew Formula

**What:** 创建 Homebrew formula，支持 `brew install exec-server` 和 `brew install exec-client`。

**Why:** macOS 用户习惯用 Homebrew 安装工具，手动下载 binary 不方便。

**Pros:**
- 简化 macOS 用户的安装流程
- 自动更新支持

**Cons:**
- 需要维护 Homebrew tap
- 需要为 macOS 提供持续构建

**Context:**
- 创建 `homebrew-exec-server` tap
- Formula 指向 GitHub Releases 的二进制文件

**Depends on / blocked by:**
- macOS Support 完成
- CI/CD 支持 macOS 构建

---

### Docker Image

**What:** 构建 Docker image，支持 `docker run exec-server`。

**Why:** 某些场景下 server 需要以容器方式部署（如 Kubernetes）。

**Pros:**
- 容器化部署
- 与现有云基础设施集成

**Cons:**
- 增加构建和发布复杂度
- 需要处理容器内外的权限映射

**Context:**
- Dockerfile:
  ```dockerfile
  FROM golang:1.18-alpine AS builder
  WORKDIR /app
  COPY . .
  RUN go build -o exec-server ./cmd/server
  FROM alpine:latest
  COPY --from=builder /app/exec-server /usr/local/bin/
  EXPOSE 8080
  CMD ["exec-server", "--config", "/config/config.yaml"]
  ```
- 挂载配置文件：`-v ~/.config/exec-server:/config`

**Depends on / blocked by:**
- Phase 1 完成

---

## Backlog

以下项没有明确的优先级或需求支持，暂时记录在 backlog 中：

- **TLS/HTTPS 支持**：加密通信
- **Client 自动更新**：类似 `brew upgrade`
- **Web UI 管理界面**：可视化配置和监控
- **User-level Whitelist**：不同用户有不同权限
- **RBAC (Role-Based Access Control)**：角色和权限管理
- **Plugin System**：支持自定义命令验证和审计逻辑
- **Command Result Caching**：相同命令的输出缓存
- **Rate Limiting per User**：防止单个用户滥用
- **Audit Log Export**：导出审计日志到外部系统
- **Integrations**：与 Prometheus, ELK, Slack 的集成
