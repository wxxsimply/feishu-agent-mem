#!/bin/bash

# 飞书检测器场景测试脚本
# 这个脚本展示如何使用 lark-cli 创建状态变化并验证检测器

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

# 加载环境变量
if [ -f ".env" ]; then
    export $(grep -v '^#' .env | xargs)
fi

echo "=========================================="
echo "飞书检测器场景测试"
echo "=========================================="
echo ""

SCENARIO=${1:-detect-only}

function echo_step() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo " $1"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

function echo_info() {
    echo -e "\033[1;34mℹ $1\033[0m"
}

function echo_success() {
    echo -e "\033[1;32m✓ $1\033[0m"
}

function echo_warning() {
    echo -e "\033[1;33m⚠ $1\033[0m"
}

# 检查 lark-cli
function check_lark_cli() {
    echo_step "检查 lark-cli"

    if command -v lark-cli &> /dev/null; then
        echo_success "lark-cli 已安装"
        lark-cli --version 2>/dev/null || echo "lark-cli 可用"
        return 0
    else
        echo_warning "lark-cli 未找到，将跳过需要 lark-cli 的测试"
        return 1
    fi
}

# 场景1: 仅检测现有数据
function run_detect_only() {
    echo_step "场景1: 检测现有数据"
    echo_info "运行所有检测器，检测过去24小时的变化"
    echo ""

    go test -v ./test/detector -run TestAllDetectorsTogether
}

# 场景2: 测试完整链路
function run_full_chain() {
    echo_step "场景2: 测试完整链路 (Detector → Emitter)"
    echo_info "测试检测器和 Emitter 的协作"
    echo ""

    go test -v ./test/detector -run TestDetectorEmitterChain
}

# 场景3: 实时测试（需要 lark-cli）
function run_live_test() {
    echo_step "场景3: 实时测试"
    echo_info "使用 live build tag 运行实时测试"
    echo ""

    if ! check_lark_cli; then
        echo_warning "跳过实时测试"
        return
    fi

    go test -v -tags=live ./test/detector -run TestLive
}

# 场景4: 并行测试
function run_parallel_test() {
    echo_step "场景4: 并行测试"
    echo_info "测试多个检测器同时运行"
    echo ""

    go test -v ./test/detector -run TestDetectorsParallel
}

# 场景5: 完整测试套件
function run_full_suite() {
    echo_step "场景5: 完整测试套件"
    echo_info "运行所有测试"
    echo ""

    go test -v ./test/detector/...
}

# 场景6: 演示如何手动使用 lark-cli 检查状态
function demo_lark_cli() {
    echo_step "场景6: lark-cli 状态检查演示"
    echo_info "展示如何使用 lark-cli 检查各个模块的状态"
    echo ""

    if ! check_lark_cli; then
        return
    fi

    echo_info "注意: 这些是演示命令，可能需要根据实际的 lark-cli 命令调整"
    echo ""

    echo "📱 检查 IM 状态"
    echo "  # 查看聊天列表"
    echo "  lark-cli im chats"
    echo ""
    echo "  # 查看消息历史"
    echo "  lark-cli im messages --chat <chat_id>"
    echo ""

    echo "🎥 检查 VC 状态"
    echo "  # 查看会议记录"
    echo "  lark-cli vc list"
    echo ""

    echo "📄 检查文档状态"
    echo "  # 搜索文档"
    echo "  lark-cli docs +search --query ''"
    echo ""

    echo "📅 检查日程状态"
    echo "  # 查看今日日程"
    echo "  lark-cli calendar +agenda"
    echo ""

    echo "✅ 检查任务状态"
    echo "  # 查看我的任务"
    echo "  lark-cli task +get-my-tasks"
    echo ""

    echo "📚 检查知识库状态"
    echo "  # 列出知识空间"
    echo "  lark-cli wiki spaces list"
    echo ""
}

# 主函数
function main() {
    case $SCENARIO in
        detect-only)
            run_detect_only
            ;;
        full-chain)
            run_full_chain
            ;;
        live)
            run_live_test
            ;;
        parallel)
            run_parallel_test
            ;;
        full)
            run_full_suite
            ;;
        demo)
            demo_lark_cli
            ;;
        help|--help|-h)
            echo "使用方法:"
            echo "  $0 [场景名]"
            echo ""
            echo "场景:"
            echo "  detect-only  - 仅检测现有数据（默认）"
            echo "  full-chain   - 测试 Detector → Emitter 完整链路"
            echo "  live         - 实时测试（需要 lark-cli）"
            echo "  parallel     - 并行测试"
            echo "  full         - 完整测试套件"
            echo "  demo         - lark-cli 命令演示"
            echo "  help         - 显示帮助"
            echo ""
            echo "示例:"
            echo "  $0"
            echo "  $0 full"
            echo "  $0 demo"
            ;;
        *)
            echo "未知场景: $SCENARIO"
            echo "使用 $0 help 查看帮助"
            exit 1
            ;;
    esac

    echo ""
    echo "=========================================="
    echo "测试完成！"
    echo "=========================================="
}

main
