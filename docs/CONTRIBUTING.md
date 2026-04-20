## 开发指南

本项目是一个容器外命令执行系统，用于解决容器内 AI agent 无法在宿主机上执行命令的问题。

### 开发环境设置

1. **Go 环境**：确保已安装 Go 1.23+ 
2. **依赖管理**：项目使用 go mod，依赖会自动下载
3. **代码格式化**：使用 `make fmt` 自动格式化代码
4. **静态检查**：使用 `make lint` 运行 golangci-lint 检查

### 编译与运行

#### 编译二进制文件

```bash
make build
```

此命令将构建 server 和 client 两个二进制文件：
- `ooc-server`：服务器端
- `ooc-client`：客户端

#### 为 Linux 构建（静态链接）

```bash
make build-linux
```

为生产环境构建兼容 CentOS 7 的静态二进制文件：
- `ooc-server-linux-amd64`
- `ooc-server-linux-arm64`
- `ooc-client-linux-amd64`
- `ooc-client-linux-arm64`

#### 运行服务器

```bash
make run-server
```

服务器默认从 `~/.config/ooc-server/config.yaml` 加载配置。首次运行时可使用 `make init` 生成默认配置文件。

#### 运行客户端

```bash
make run-client
```

此命令执行 `echo` 命令并返回结果。

### 测试

运行所有测试：

```bash
make test
```

此命令会：
- 运行所有单元测试和集成测试
- 使用 `-race` 检测数据竞争
- 生成覆盖率报告 `coverage.html`

运行快速测试（跳过慢测试）：

```bash
make test-short
```

### 代码质量

- 所有代码必须通过 `make lint` 检查
- 保持函数简洁（建议不超过 50 行）
- 使用 Go 标准库，避免不必要的第三方依赖
- 所有错误必须被明确处理
- 使用 `fmt.Errorf("%w", err)` 包装错误以保留堆栈信息

### 提交代码

1. 在本地分支上开发
2. 运行 `make fmt` 和 `make lint` 确保代码质量
3. 运行 `make test` 确保所有测试通过
4. 提交代码并创建 Pull Request

### 开发建议

- 优先使用 `golang-patterns` 技能获取 Go 最佳实践
- 使用 `golang-testing` 技能获取测试最佳实践
- 使用 `security-review` 技能检查安全问题
- 使用 `plan-eng-review` 技能进行架构评审

### 文档更新

本文件的生成源为：
- `Makefile`：构建和测试命令
- `CLAUDE.md`：项目概述
- `cmd/ooc-server/main.go`：服务器启动逻辑
- `cmd/ooc-client/main.go`：客户端启动逻辑
- `internal/handlers/`：API 端点实现

不要直接编辑此文件，使用 `make update-docs` 重新生成。

<!-- AUTO-GENERATED -->