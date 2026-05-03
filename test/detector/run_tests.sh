#!/bin/bash

# 检测器测试运行脚本
# 使用真实的飞书环境进行测试

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

echo "=========================================="
echo "飞书检测器测试套件"
echo "=========================================="
echo ""

# 检查 .env 文件
if [ -f ".env" ]; then
    echo "✓ 加载 .env 文件"
    export $(grep -v '^#' .env | xargs)
else
    echo "⚠ 未找到 .env 文件，使用环境变量"
fi

# 测试选项
TEST_TYPE=${1:-all}

echo "测试类型: $TEST_TYPE"
echo ""

function run_single_test() {
    local test_name=$1
    echo ""
    echo "----------------------------------------"
    echo "运行 $test_name 测试"
    echo "----------------------------------------"
    go test -v ./test/detector -run "$test_name"
}

function run_all_detectors() {
    echo ""
    echo "=========================================="
    echo "运行所有检测器测试"
    echo "=========================================="

    # 逐个运行每个检测器测试
    run_single_test "TestIMDetector"
    run_single_test "TestVCDetector"
    run_single_test "TestDocDetector"
    run_single_test "TestCalendarDetector"
    run_single_test "TestTaskDetector"
    run_single_test "TestWikiDetector"
}

function run_integration_test() {
    echo ""
    echo "=========================================="
    echo "运行集成测试"
    echo "=========================================="

    run_single_test "TestAllDetectorsTogether"
    run_single_test "TestDetectorsParallel"
    run_single_test "TestDetectorEmitterChain"
}

function run_full_suite() {
    echo ""
    echo "=========================================="
    echo "运行完整测试套件"
    echo "=========================================="

    go test -v ./test/detector/...
}

# 根据参数运行测试
case $TEST_TYPE in
    im)
        run_single_test "TestIMDetector"
        ;;
    vc)
        run_single_test "TestVCDetector"
        ;;
    doc)
        run_single_test "TestDocDetector"
        ;;
    calendar)
        run_single_test "TestCalendarDetector"
        ;;
    task)
        run_single_test "TestTaskDetector"
        ;;
    wiki)
        run_single_test "TestWikiDetector"
        ;;
    integration)
        run_integration_test
        ;;
    all)
        run_full_suite
        ;;
    help|--help|-h)
        echo "使用方法:"
        echo "  $0 [测试类型]"
        echo ""
        echo "测试类型:"
        echo "  im          - 仅测试 IM 检测器"
        echo "  vc          - 仅测试 VC 检测器"
        echo "  doc         - 仅测试 Doc 检测器"
        echo "  calendar    - 仅测试 Calendar 检测器"
        echo "  task        - 仅测试 Task 检测器"
        echo "  wiki        - 仅测试 Wiki 检测器"
        echo "  integration - 仅运行集成测试"
        echo "  all         - 运行所有测试（默认）"
        echo "  help        - 显示帮助信息"
        echo ""
        echo "示例:"
        echo "  $0              # 运行所有测试"
        echo "  $0 im           # 仅测试 IM 检测器"
        echo "  $0 integration  # 仅运行集成测试"
        ;;
    *)
        echo "未知的测试类型: $TEST_TYPE"
        echo "使用 $0 help 查看帮助"
        exit 1
        ;;
esac

echo ""
echo "=========================================="
echo "测试完成！"
echo "=========================================="
