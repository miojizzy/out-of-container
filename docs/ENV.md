## 环境变量参考

本项目不使用环境变量进行核心配置，所有配置均通过 YAML 配置文件管理。但以下环境变量可用于开发和部署：

### 开发与构建

| 环境变量 | 描述 | 默认值 | 示例 |
|----------|------|--------|------|
| `CGO_ENABLED` | 是否启用 CGO（用于 cgo 代码） | `0` | `CGO_ENABLED=0` 用于静态编译 |
| `GOOS` | 目标操作系统 | `linux` | `GOOS=linux` 构建 Linux 二进制 |
| `GOARCH` | 目标架构 | `amd64` | `GOARCH=arm64` 构建 ARM64 二进制 |
| `VERSION` | 版本号（用于构建时注入） | 从 git 标签获取 | `VERSION=v1.2.3` |
| `BUILD_TIME` | 构建时间（用于构建时注入） | 当前 UTC 时间 | `BUILD_TIME=2026-04-17T01:30:00Z` |

### 部署与运行

| 环境变量 | 描述 | 默认值 | 示例 |
|----------|------|--------|------|
| `OOC_SERVER_CONFIG` | 指定配置文件路径 | `~/.config/ooc-server/config.yaml` | `OOC_SERVER_CONFIG=/etc/ooc-server/config.yaml` |
| `OOC_CLIENT_CONFIG` | 指定客户端配置文件路径 | `~/.config/ooc-client/config.yaml` | `OOC_CLIENT_CONFIG=/etc/ooc-client/config.yaml` |

### 使用示例

#### 构建 Linux 静态二进制

```bash
# 一次性设置并构建
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build
```

#### 使用自定义配置路径运行服务器

```bash
# 启动时指定配置文件
OOC_SERVER_CONFIG=/etc/ooc-server/config.yaml ./ooc-server
```

#### 使用自定义版本号构建

```bash
# 指定版本号
VERSION=v1.0.0 BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) make build
```

### 配置文件优先级

1. **命令行参数**（`--config`）
2. **环境变量**（`OOC_SERVER_CONFIG`）
3. **默认路径**（`~/.config/ooc-server/config.yaml`）

客户端同理。

### 安全建议

- 不要将敏感信息（如 API Token）存储在环境变量中，使用配置文件
- 配置文件权限应设为 600：`chmod 600 ~/.config/ooc-server/config.yaml`
- 在 Docker 容器中，通过 `--mount` 挂载配置文件，而非使用环境变量

<!-- AUTO-GENERATED -->