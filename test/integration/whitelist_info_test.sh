#!/bin/bash

# 集成测试脚本：测试白名单信息端点和客户端发现功能
# 测试场景：
# 1. 启动服务器
# 2. 测试白名单信息端点的认证（有效token、无效token、缺失token）
# 3. 测试白名单信息端点的响应格式
# 4. 测试客户端命令执行功能，确认它能够使用发现功能列出可用命令和路径

# 不使用set -e，因为我们想运行所有测试并报告结果

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试结果计数器
PASSED=0
FAILED=0

# 打印测试结果
function print_result {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}PASS${NC}: $2"
        ((PASSED++))
    else
        echo -e "${RED}FAIL${NC}: $2"
        ((FAILED++))
    fi
}

# 清理函数
function cleanup {
    echo "清理测试环境..."
    if [ ! -z "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    if [ -d "$TEMP_DIR" ]; then
        rm -rf "$TEMP_DIR"
    fi
    echo "清理完成"
}

# 设置清理函数在脚本退出时执行
trap cleanup EXIT

# 创建临时目录
TEMP_DIR=$(mktemp -d)
echo "使用临时目录: $TEMP_DIR"

# 创建测试配置文件
CONFIG_FILE="$TEMP_DIR/config.yaml"
cat > "$CONFIG_FILE" << EOF
server:
  listen: "127.0.0.1:8080"
  timeout_seconds: 30
  max_output_mb: 10
  max_concurrent: 5
  api_token: "test-token-12345"

whitelist:
  literal_commands:
    - "ls"
    - "pwd"
    - "echo"
  regex_commands:
    - "^make.*"
    - "^go.*"
  allowed_paths:
    - "$TEMP_DIR"
  reload_interval_seconds: 5

audit:
  enabled: true
  log_file: "$TEMP_DIR/audit.log"
  rotation_max_mb: 10
  rotation_count: 10
EOF

echo "创建的配置文件内容:"
cat "$CONFIG_FILE"
echo

# 构建服务器和客户端（如果尚未构建）
echo "检查服务器和客户端二进制文件..."
if [ ! -f "./ooc-server" ]; then
    echo "构建服务器..."
    go build -o ooc-server ./cmd/ooc-server
fi

if [ ! -f "./ooc-client" ]; then
    echo "构建客户端..."
    go build -o ooc-client ./cmd/ooc-client
fi

# 启动服务器
echo "启动服务器..."
./ooc-server -config "$CONFIG_FILE" &
SERVER_PID=$!

# 等待服务器启动
sleep 2

# 检查服务器是否正在运行
if kill -0 $SERVER_PID 2>/dev/null; then
    echo "服务器已启动 (PID: $SERVER_PID)"
else
    echo "服务器启动失败"
    exit 1
fi

# 创建客户端配置文件
CLIENT_CONFIG="$TEMP_DIR/client-config.yaml"
cat > "$CLIENT_CONFIG" << EOF
server_url: "http://127.0.0.1:8080"
api_token: "test-token-12345"
timeout_seconds: 30
EOF

echo "创建的客户端配置文件:"
cat "$CLIENT_CONFIG"
echo

# 测试1: 有效token的白名单信息端点访问
echo "测试1: 有效token的白名单信息端点访问"
RESPONSE=$(curl -s -H "Authorization: Bearer test-token-12345" http://127.0.0.1:8080/whitelist-info)
if [ $? -eq 0 ] && echo "$RESPONSE" | grep -q '"literal_commands"'; then
    print_result 0 "有效token可以访问白名单信息端点"

    # 验证响应格式
    if echo "$RESPONSE" | grep -q '"regex_commands"' && echo "$RESPONSE" | grep -q '"allowed_paths"'; then
        print_result 0 "白名单信息端点响应格式正确"
    else
        print_result 1 "白名单信息端点响应格式不正确"
    fi
else
    print_result 1 "有效token无法访问白名单信息端点"
fi

# 测试2: 无效token的白名单信息端点访问
echo "测试2: 无效token的白名单信息端点访问"
RESPONSE=$(curl -s -w "%{http_code}" -H "Authorization: Bearer invalid-token" http://127.0.0.1:8080/whitelist-info)
HTTP_CODE=$(echo "$RESPONSE" | tail -c 4)
if [ "$HTTP_CODE" -eq 401 ]; then
    print_result 0 "无效token被正确拒绝（HTTP 401）"
else
    print_result 1 "无效token未被正确拒绝（HTTP状态码: $HTTP_CODE）"
fi

# 测试3: 缺失token的白名单信息端点访问
echo "测试3: 缺失token的白名单信息端点访问"
RESPONSE=$(curl -s -w "%{http_code}" http://127.0.0.1:8080/whitelist-info)
HTTP_CODE=$(echo "$RESPONSE" | tail -c 4)
if [ "$HTTP_CODE" -eq 401 ]; then
    print_result 0 "缺失token被正确拒绝（HTTP 401）"
else
    print_result 1 "缺失token未被正确拒绝（HTTP状态码: $HTTP_CODE）"
fi

# 测试4: 客户端发现功能 - 列出命令
echo "测试4: 客户端发现功能 - 列出命令"
OUTPUT=$(./ooc-client -config "$CLIENT_CONFIG" -list-commands 2>&1)
if [ $? -eq 0 ] && echo "$OUTPUT" | grep -q "可用命令"; then
    print_result 0 "客户端可以列出可用命令"
else
    print_result 1 "客户端无法列出可用命令，输出: $OUTPUT"
fi

# 测试5: 客户端发现功能 - 列出路径
echo "测试5: 客户端发现功能 - 列出路径"
OUTPUT=$(./ooc-client -config "$CLIENT_CONFIG" -list-paths 2>&1)
if [ $? -eq 0 ] && echo "$OUTPUT" | grep -q "允许的路径"; then
    print_result 0 "客户端可以列出允许的路径"
else
    print_result 1 "客户端无法列出允许的路径，输出: $OUTPUT"
fi

# 测试6: 客户端命令执行功能
echo "测试6: 客户端命令执行功能"
OUTPUT=$(./ooc-client -config "$CLIENT_CONFIG" -command "echo" -args "hello world" -cwd "$TEMP_DIR" 2>&1)
if [ $? -eq 0 ] && echo "$OUTPUT" | grep -q "hello world"; then
    print_result 0 "客户端可以成功执行命令"
else
    print_result 1 "客户端无法执行命令，输出: $OUTPUT"
fi

# 测试7: 客户端命令执行失败情况（不允许的命令）
echo "测试7: 客户端命令执行失败情况（不允许的命令）"
OUTPUT=$(./ooc-client -config "$CLIENT_CONFIG" -command "rm" -args "-rf /" 2>&1)
if echo "$OUTPUT" | grep -q "Error" || echo "$OUTPUT" | grep -q "error"; then
    print_result 0 "客户端正确拒绝了不允许的命令"
else
    print_result 1 "客户端未正确拒绝不允许的命令，输出: $OUTPUT"
fi

# 输出测试总结
echo
echo "=========================="
echo "测试总结"
echo "=========================="
echo "通过: $PASSED"
echo "失败: $FAILED"
echo "总计: $((PASSED + FAILED))"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}所有测试通过！${NC}"
    exit 0
else
    echo -e "${RED}有 $FAILED 个测试失败！${NC}"
    exit 1
fi