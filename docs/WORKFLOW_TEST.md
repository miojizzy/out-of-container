# 流水线测试文档

此文档用于验证统一的 CI/Tag/Release 流水线是否正常工作。

## 测试场景

### 1. CI 触发验证
- ✅ PR 提交时应自动触发 `ci.yml`（lint + test）
- ✅ main 分支直推时应触发 CI
- ✅ 手动触发 CI 应该可行

### 2. 自动 Tag 触发验证
- ✅ PR 合并到 main 时应自动触发 `tag.yml`
- ✅ 应自动计算下一个 patch 版本
- ✅ 应在 `concurrency` 串行化控制下避免并发重复

### 3. 手动 Tag 触发验证
- ✅ 可以在 GitHub Actions 手动触发 `tag.yml`
- ✅ 需要输入完整 tag（如 v2.00.000）
- ✅ 应校验 tag 格式合法性

### 4. Release 触发验证
- ✅ tag push 后应自动触发 `release.yml`
- ✅ 应基于 `github.ref_name` 获取版本
- ✅ 应构建二进制并发布到 GitHub Releases

## 测试流程

1. 合并此 PR 到 main
2. 观察 tag.yml 是否自动触发
3. 等待 tag 推送完成
4. 确认 release.yml 自动触发
5. 验证 Release 页面是否正确创建

## 备注

- 版本规则唯一来源: `.github/scripts/tag_version.sh`
- 格式: `vMAJOR.MINOR.PATCH` (MINOR 两位, PATCH 三位)
