#!/bin/bash

# ooc-skill-setup.sh - 交互式安装和配置脚本
# 用于设置 .claude/skills/ooc-exec 技能和客户端配置

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 工具函数
print_header() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# 检查先决条件
check_prerequisites() {
    print_header "检查先决条件"

    # 检查是否在项目根目录（有 Makefile 和 .claude 目录）
    if [[ ! -f "Makefile" ]] || [[ ! -d ".claude" ]]; then
        print_error "请在项目根目录运行此脚本"
        exit 1
    fi

    # 检查 ooc-client 二进制是否存在
    if [[ ! -f "ooc-client" ]]; then
        print_error "未找到 ooc-client 二进制文件，请先构建项目"
        exit 1
    fi

    # 检查技能目录
    if [[ ! -d ".claude/skills/ooc-exec" ]]; then
        print_error "未找到技能目录 .claude/skills/ooc-exec"
        exit 1
    fi

    print_success "先决条件检查通过"
}

# 安装技能二进制
install_skill_binary() {
    print_header "安装技能二进制文件"

    local skill_bin_path=".claude/skills/ooc-exec/bin/ooc-client"

    # 复制二进制文件
    echo "复制 ooc-client 到技能目录..."
    cp ooc-client "$skill_bin_path"
    chmod +x "$skill_bin_path"

    # 验证
    if [[ -x "$skill_bin_path" ]]; then
        print_success "技能二进制文件安装成功: $skill_bin_path"
    else
        print_error "技能二进制文件安装失败"
        exit 1
    fi
}

# 配置向导
configure_client() {
    print_header "配置客户端"

    local config_dir="$HOME/.config/ooc-client"
    local config_file="$config_dir/config.yaml"

    # 创建配置目录
    mkdir -p "$config_dir"

    # 检查是否已存在配置
    if [[ -f "$config_file" ]]; then
        echo -e "${YELLOW}配置文件已存在: $config_file${NC}"
        read -p "是否覆盖? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_success "使用现有配置文件"
            return
        fi
    fi

    # 获取用户输入
    echo "请提供以下配置信息:"

    # 服务器 URL
    read -p "服务器 URL (例如: http://192.168.1.100:8080): " server_url
    while [[ -z "$server_url" ]]; do
        print_warning "服务器 URL 不能为空"
        read -p "服务器 URL (例如: http://192.168.1.100:8080): " server_url
    done

    # API Token
    echo "API Token 可以在宿主机的 ooc-server 配置文件中找到"
    read -p "API Token: " api_token
    while [[ -z "$api_token" ]]; do
        print_warning "API Token 不能为空"
        read -p "API Token: " api_token
    done

    # 超时时间（可选，默认 35）
    read -p "超时时间（秒，默认 35）: " timeout_seconds
    timeout_seconds=${timeout_seconds:-35}

    # 创建配置文件
    cat > "$config_file" <<EOF
# ooc-client 配置文件
# 由 setup-exec-skill.sh 脚本自动生成

# 服务器 URL - 宿主机上 ooc-server 的地址
server_url: "$server_url"

# API Token - 从 ooc-server 配置文件获取
api_token: "$api_token"

# 超时时间（秒）- 建议比 server 的 timeout 大 5 秒
timeout_seconds: $timeout_seconds
EOF

    print_success "配置文件创建成功: $config_file"
}

# 验证配置
verify_configuration() {
    print_header "验证配置"

    local config_file="$HOME/.config/ooc-client/config.yaml"
    local skill_bin_path=".claude/skills/ooc-exec/bin/ooc-client"

    # 检查配置文件
    if [[ ! -f "$config_file" ]]; then
        print_warning "配置文件不存在: $config_file"
        print_warning "请运行此脚本的配置部分或手动创建配置文件"
        return 1
    fi

    # 检查技能二进制
    if [[ ! -x "$skill_bin_path" ]]; then
        print_warning "技能二进制文件不存在或不可执行: $skill_bin_path"
        print_warning "请运行此脚本的安装部分"
        return 1
    fi

    # 检查配置文件格式
    if command -v yq &>/dev/null; then
        if yq eval '.server_url' "$config_file" &>/dev/null; then
            print_success "配置文件格式正确"
        else
            print_warning "配置文件格式可能有问题"
        fi
    else
        print_warning "未安装 yq，跳过配置文件格式检查"
    fi

    print_success "配置验证完成"
}

# 测试连接
test_connection() {
    print_header "测试连接"

    local config_file="$HOME/.config/ooc-client/config.yaml"

    # 检查配置文件
    if [[ ! -f "$config_file" ]]; then
        print_warning "配置文件不存在，跳过连接测试"
        return 1
    fi

    # 读取服务器 URL
    local server_url=""
    if command -v yq &>/dev/null; then
        server_url=$(yq eval '.server_url' "$config_file" 2>/dev/null)
    else
        # 使用 grep 和 sed 提取 server_url
        server_url=$(grep "server_url:" "$config_file" | sed -E 's/.*server_url:[[:space:]]*["'"'"']?([^"'"'"']*)["'"'"']?.*/\1/')
    fi

    if [[ -z "$server_url" ]]; then
        print_warning "无法从配置文件中读取服务器 URL"
        return 1
    fi

    # 测试连接
    local health_url="${server_url%/}/health"
    echo "测试服务器连接: $health_url"

    if command -v curl &>/dev/null; then
        if curl -s --max-time 5 "$health_url" &>/dev/null; then
            print_success "服务器连接成功"
        else
            print_warning "服务器连接失败，请检查服务器是否运行以及网络连接"
        fi
    else
        print_warning "未安装 curl，跳过连接测试"
    fi
}

# 主函数
main() {
    print_header "Exec-Client 技能安装向导"

    # 解析命令行参数
    local action="full"  # 默认执行完整安装

    while [[ $# -gt 0 ]]; do
        case $1 in
            --install|-i)
                action="install"
                shift
                ;;
            --configure|-c)
                action="configure"
                shift
                ;;
            --verify|-v)
                action="verify"
                shift
                ;;
            --test|-t)
                action="test"
                shift
                ;;
            --help|-h)
                echo "用法: $0 [选项]"
                echo "选项:"
                echo "  --install, -i     只安装技能二进制文件"
                echo "  --configure, -c   只配置客户端"
                echo "  --verify, -v      只验证配置"
                echo "  --test, -t        只测试连接"
                echo "  --help, -h        显示此帮助信息"
                echo ""
                echo "默认行为是执行完整安装（安装 + 配置 + 验证 + 测试）"
                exit 0
                ;;
            *)
                print_error "未知选项: $1"
                exit 1
                ;;
        esac
    done

    # 执行相应操作
    case $action in
        "install")
            check_prerequisites
            install_skill_binary
            ;;
        "configure")
            configure_client
            ;;
        "verify")
            verify_configuration
            ;;
        "test")
            test_connection
            ;;
        "full")
            check_prerequisites
            install_skill_binary
            configure_client
            verify_configuration
            test_connection
            print_header "安装完成"
            echo -e "${GREEN}Exec-Client 技能已成功安装和配置！${NC}"
            echo ""
            echo "现在你可以使用以下命令测试技能："
            echo "  /ooc-exec command=\"echo\" args=\"Hello from container!\" cwd=\"/\""
            echo ""
            echo "或者在项目根目录运行："
            echo "  make test-exec-skill"
            ;;
    esac
}

# 执行主函数
main "$@"