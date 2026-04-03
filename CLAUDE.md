# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目说明

这是一个容器远程命令执行系统，用于解决容器内 AI agent 无法在宿主机上执行命令的问题。

## 技术栈

- **Server**: Go 1.18+（静态编译，兼容老系统）
- **Client**: Go（与 server 同技术栈）
- **协议**: HTTP API（支持后续扩展 SSE）

## 项目结构

```
.
├── .claude/
│   ├── skills/          # Claude Code 技能（符号链接到全局 gstack）
│   │   └── gstack -> ~/.claude/skills/gstack
│   └── settings.local.json  # 项目级配置
├── .gstack/             # gstack 项目数据
│   └── resources-shown.jsonl  # 已显示的资源记录
├── design/              # 设计文档
│   └── zhaozeyu-main-design-20260401-050215.md
├── CLAUDE.md            # 本文件
└── README.md
```

## 设计文档

详细设计文档在 `design/zhaozeyu-main-design-20260401-050215.md`，包含：
- 问题陈述和需求分析
- 三种技术方案对比
- 推荐方案：渐进式架构（Phase 1 → Phase 2 → Phase 3）
- API 接口规范（Phase 1 同步执行）
- 安全性边界（认证、白名单、审计、并发）
- 配置文件示例
- 实现步骤

## 开发说明

- 使用简体中文进行所有回复和注释
- 变量名、函数名保持英文
- 所有设计和开发工作都基于设计文档执行

## 可用技能

项目已安装 [gstack](https://github.com/garrytan/gstack) 技能（符号链接模式），提供以下技能：

**项目管理：**
- `/ship` - 创建 PR，构建、测试、发布（代码完成后用）
- `/review` - PR diff 审查（合并前用）

**设计与评审：**
- `/plan-eng-review` - 架构和测试评审（实现前调用）
- `/plan-ceo-review` - 扩展性评审（功能设计用）

**测试与验证：**
- `/qa` - 系统测试（安全性、并发、超时等）
- `/investigate` - 问题调试和根因分析

**开发辅助：**
- `/office-hours` - 产品思维和设计规划（已完成本次设计）
- `/simplify` - 代码质量审查

## 技能路由

当用户请求匹配可用技能时，始终使用 Skill 工具调用它作为第一个动作。

关键路由规则：
- 错误、bug、"为什么坏了" → 调用 `/investigate`
- QA、测试网站 → 调用 `/qa`
- 代码审查 → 调用 `/review`
- 发布、部署 → 调用 `/ship`

## 下一步

根据设计文档实现 Phase 1：
1. 实现 Executor、WhitelistChecker、Auditor 接口
2. 实现 HTTP server（/ooc-exec API）
3. 实现 Go 客户端
4. 集成测试

实现前推荐运行 `/plan-eng-review` 锁定技术细节。
