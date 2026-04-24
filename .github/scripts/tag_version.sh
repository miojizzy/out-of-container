#!/usr/bin/env bash
# .github/scripts/tag_version.sh
# 版本标签规则的唯一真相来源
# 格式: vMAJOR.MINOR.PATCH (MINOR 两位补零, PATCH 三位补零)
# 示例: v1.02.003
#
# 用法:
#   tag_version.sh next          — 基于最新 tag 计算下一个 patch 版本
#   tag_version.sh validate TAG  — 校验 TAG 是否符合格式规范
#   tag_version.sh current       — 输出当前最新合法 tag (不存在时输出默认值)
set -euo pipefail

# === 常量 ===
DEFAULT_TAG="v0.00.000"
TAG_PATTERN='^v[0-9]+\.[0-9]{2}\.[0-9]{3}$'

# === 函数 ===

# 校验 tag 格式是否合法
validate_tag() {
  local tag="$1"
  if [[ "$tag" =~ $TAG_PATTERN ]]; then
    return 0
  else
    echo "ERROR: 非法 tag 格式: '$tag' (期望格式: vMAJOR.MINOR.PATCH, 例如 v1.02.003)" >&2
    return 1
  fi
}

# 获取当前最新合法 tag
get_latest_tag() {
  local latest
  latest=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
  if [[ -z "$latest" ]]; then
    echo "$DEFAULT_TAG"
    return
  fi
  # 如果最新 tag 不符合格式，回退到默认值
  if [[ "$latest" =~ $TAG_PATTERN ]]; then
    echo "$latest"
  else
    echo "$DEFAULT_TAG"
  fi
}

# 格式化版本号: MAJOR 无填充, MINOR 两位补零, PATCH 三位补零
format_version() {
  local major="$1" minor="$2" patch="$3"
  printf "v%d.%02d.%03d\n" "$major" "$minor" "$patch"
}

# 解析 tag 为 MAJOR MINOR PATCH (去前导零, 避免八进制)
parse_tag() {
  local tag="$1"
  local version_str="${tag#v}"
  local major minor patch
  major=$(echo "$version_str" | awk -F'.' '{print $1}')
  minor=$(echo "$version_str" | awk -F'.' '{print $2}')
  patch=$(echo "$version_str" | awk -F'.' '{print $3}')
  # 10# 强制十进制解析, 避免前导零被当作八进制
  echo "$((10#$major)) $((10#$minor)) $((10#$patch))"
}

# 计算下一个 patch 版本
next_version() {
  local latest
  latest=$(get_latest_tag)
  local parts
  read -r major minor patch <<< "$(parse_tag "$latest")"
  patch=$((patch + 1))
  format_version "$major" "$minor" "$patch"
}

# === 主入口 ===
cmd="${1:-}"
case "$cmd" in
  next)
    next_version
    ;;
  validate)
    tag="${2:-}"
    if [[ -z "$tag" ]]; then
      echo "ERROR: validate 需要提供 tag 参数" >&2
      exit 1
    fi
    validate_tag "$tag"
    echo "OK: '$tag' 格式合法"
    ;;
  current)
    get_latest_tag
    ;;
  *)
    echo "用法: $0 {next|validate TAG|current}" >&2
    exit 1
    ;;
esac
