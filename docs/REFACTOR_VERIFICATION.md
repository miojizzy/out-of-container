# 流水线验证说明

本分支用于验证统一的 CI/Tag/Release 流水线改动是否正常工作。

## 验证步骤

### 1. CI 工作流验证
- 此 PR 提交时应自动触发 `ci.yml`
- 应运行 `lint` 和 `test` 两个 job
- CI 应通过

### 2. Tag 工作流验证（PR 合并后）
- 合并此 PR 到 main 后应自动触发 `tag.yml`
- 应调用 `.github/scripts/tag_version.sh next` 计算下一个 patch 版本
- 应在 main 的最新提交上创建 annotated tag
- 应推送 tag 到远端

### 3. Release 工作流验证（tag 推送后）
- tag 推送后应自动触发 `release.yml`
- 应基于 `github.ref_name` 获取版本（不再用 git describe）
- 应构建 server 和 client 二进制
- 应生成 checksums
- 应创建 GitHub Release

## 预期结果

合并此 PR 应该观察到：
1. PR 提交时 CI 通过
2. PR 合并时自动打出 tag（v0.01.012 或更新版本）
3. tag 推送后自动构建并发布 Release

## 额外手动验证

可选的手动测试：
1. 在 GitHub Actions 页面手动触发 `tag.yml`
2. 输入一个合法的 tag（如 v2.00.000）
3. 验证手动 tag 工作流的完整链路
