#!/bin/bash

# 飞书检测器并发测试脚本
# 启动两个进程：一个定时检测，一个模拟状态变化

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

# 加载环境变量
if [ -f ".env" ]; then
    export $(grep -v '^#' .env | xargs)
fi

echo "=========================================="
echo "  飞书检测器并发测试"
echo "=========================================="
echo ""

TEST_TARGET=${1:-all}

function echo_banner() {
    echo ""
    echo "╔════════════════════════════════════════╗"
    echo "║  $1"
    echo "╚════════════════════════════════════════╝"
}

function echo_info() {
    echo -e "\033[1;34mℹ $1\033[0m"
}

function echo_hint() {
    echo -e "\033[1;36m💡 $1\033[0m"
}

function echo_warning() {
    echo -e "\033[1;33m⚠ $1\033[0m"
}

# 检查 lark-cli
function check_lark_cli() {
    if command -v lark-cli &> /dev/null; then
        return 0
    else
        echo_warning "lark-cli 未找到，并发测试需要 lark-cli"
        return 1
    fi
}

# 测试说明
function show_test_intro() {
    echo_banner "测试说明"
    echo ""
    echo "这个测试会启动两个并行进程："
    echo ""
    echo "  🔍 进程1: 检测器"
    echo "      - 每 30 秒运行一次检测"
    echo "      - 持续 2 分钟"
    echo ""
    echo "  🎮 进程2: 模拟器（你）"
    echo "      - 在飞书中手动操作"
    echo "      - 观察检测器是否捕捉到变化"
    echo ""
    echo_hint "建议在测试开始后立即进行操作"
    echo ""
}

# 运行单个并发测试
function run_concurrent_test() {
    local name=$1
    local test_func=$2

    echo_banner "测试 $name 检测器"
    echo_info "准备启动并发测试..."
    echo ""

    read -p "按 Enter 开始测试，或 Ctrl+C 取消... " -n 1 -r
    echo ""

    show_test_intro

    echo_info "启动测试中..."
    echo ""

    go test -v -tags=concurrent ./test/detector -run "$test_func"
}

# 运行所有并发测试
function run_all_concurrent() {
    echo_banner "所有检测器并发测试"
    echo_info "这个测试会同时运行所有检测器"
    echo ""

    read -p "按 Enter 开始测试，或 Ctrl+C 取消... " -n 1 -r
    echo ""

    echo_banner "测试说明"
    echo ""
    echo "这个测试会同时启动所有检测器："
    echo ""
    echo "  IM, VC, Docs, Calendar, Task, Wiki"
    echo ""
    echo "你可以在飞书中进行各种操作，观察哪些被检测到！"
    echo ""
    echo_hint "测试时长 2 分钟"
    echo ""

    read -p "准备好了吗？按 Enter 开始... " -n 1 -r
    echo ""

    go test -v -tags=concurrent ./test/detector -run TestConcurrentAll
}

# 主菜单
function show_menu() {
    echo_banner "请选择要测试的检测器"
    echo ""
    echo "  1) IM (消息)"
    echo "  2) VC (会议)"
    echo "  3) Docs (文档)"
    echo "  4) Calendar (日程)"
    echo "  5) Task (任务)"
    echo "  6) Wiki (知识库)"
    echo "  7) All (全部一起运行)"
    echo ""
    echo "  h) 帮助"
    echo "  q) 退出"
    echo ""
}

# 主函数
function main() {
    if ! check_lark_cli; then
        echo ""
        echo "请先安装并配置 lark-cli"
        exit 1
    fi

    case $TEST_TARGET in
        im)
            run_concurrent_test "IM" "TestConcurrentIM"
            ;;
        vc)
            run_concurrent_test "VC" "TestConcurrentVC"
            ;;
        docs)
            run_concurrent_test "Docs" "TestConcurrentDocs"
            ;;
        calendar)
            run_concurrent_test "Calendar" "TestConcurrentCalendar"
            ;;
        task)
            run_concurrent_test "Task" "TestConcurrentTask"
            ;;
        wiki)
            run_concurrent_test "Wiki" "TestConcurrentWiki"
            ;;
        all)
            run_all_concurrent
            ;;
        menu|interactive)
            while true; do
                show_menu
                read -p "请选择 [1-7/h/q]: " choice

                case $choice in
                    1) run_concurrent_test "IM" "TestConcurrentIM" ;;
                    2) run_concurrent_test "VC" "TestConcurrentVC" ;;
                    3) run_concurrent_test "Docs" "TestConcurrentDocs" ;;
                    4) run_concurrent_test "Calendar" "TestConcurrentCalendar" ;;
                    5) run_concurrent_test "Task" "TestConcurrentTask" ;;
                    6) run_concurrent_test "Wiki" "TestConcurrentWiki" ;;
                    7) run_all_concurrent ;;
                    h)
                        echo ""
                        echo "使用方法："
                        echo "  $0 [im|vc|docs|calendar|task|wiki|all|menu]"
                        echo ""
                        ;;
                    q)
                        echo "再见！"
                        exit 0
                        ;;
                    *)
                        echo "无效选择"
                        ;;
                esac

                echo ""
                read -p "继续测试？(y/n): " continue_choice
                if [[ ! $continue_choice =~ ^[Yy]$ ]]; then
                    echo "再见！"
                    exit 0
                fi
            done
            ;;
        help|--help|-h)
            echo "使用方法:"
            echo "  $0 [im|vc|docs|calendar|task|wiki|all|menu]"
            echo ""
            echo "示例:"
            echo "  $0 im              # 测试 IM 检测器"
            echo "  $0 all             # 测试所有检测器"
            echo "  $0 menu            # 交互式菜单"
            echo ""
            ;;
        *)
            echo "未知选项: $TEST_TARGET"
            echo "使用 $0 help 查看帮助"
            exit 1
            ;;
    esac

    echo ""
    echo "=========================================="
    echo "  测试完成！"
    echo "=========================================="
}

main
