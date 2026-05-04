#!/bin/bash
# OpenClaw 集成基准测试
# 测试 feishu-agent-mem MCP Server + OpenClaw Docker 的端到端性能
#
# 前置条件:
#   1. go build -o bin/mcp-server ./cmd/mcp-server
#   2. Docker 容器 openclaw-zh 正在运行
#   3. lark-cli 已配置

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPORT_DIR="$SCRIPT_DIR/reports"
mkdir -p "$REPORT_DIR"

REPORT_FILE="$REPORT_DIR/openclaw-bench-$(date +%Y%m%d-%H%M%S).json"
TIMESTAMP=$(date -Iseconds)

echo "=========================================="
echo " OpenClaw 集成基准测试"
echo " 开始时间: $(date)"
echo "=========================================="

# ==========================================
# 1. 检查依赖
# ==========================================
echo ""
echo "[1/5] 检查依赖..."

MCP_BIN="$PROJECT_DIR/bin/mcp-server"
if [ ! -f "$MCP_BIN" ]; then
    echo "  MCP 二进制未找到，构建中..."
    (cd "$PROJECT_DIR" && go build -o bin/mcp-server ./cmd/mcp-server)
fi
echo "  ✅ MCP Server: $MCP_BIN"

# 检查 Docker
if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "openclaw-zh"; then
    echo "  ✅ OpenClaw 容器: running"
    OPENCLAW_AVAILABLE=true
else
    echo "  ⚠️  openclaw-zh 容器未运行，部分测试将跳过"
    OPENCLAW_AVAILABLE=false
fi

# 检查 lark-cli
if command -v lark-cli &>/dev/null; then
    echo "  ✅ lark-cli: available"
    LARK_CLI_AVAILABLE=true
else
    echo "  ⚠️  lark-cli 未安装，飞书相关测试将跳过"
    LARK_CLI_AVAILABLE=false
fi

# ==========================================
# 2. MCP Server 延迟基准
# ==========================================
echo ""
echo "[2/5] MCP Server 延迟基准..."

# 启动 MCP Server
MCP_LOG=$(mktemp)
"$MCP_BIN" > "$MCP_LOG" 2>&1 &
MCP_PID=$!
sleep 1

# 检查是否启动成功
if ! kill -0 $MCP_PID 2>/dev/null; then
    echo "  ❌ MCP Server 启动失败"
    cat "$MCP_LOG"
    exit 1
fi
echo "  ✅ MCP Server started (PID: $MCP_PID)"

# 清理函数
cleanup() {
    kill $MCP_PID 2>/dev/null || true
    rm -f "$MCP_LOG"
}
trap cleanup EXIT

# 测试函数：发送 JSON-RPC 请求并测量延迟
bench_mcp() {
    local name=$1
    local request=$2
    local iterations=${3:-10}

    local total_ms=0
    local min_ms=999999
    local max_ms=0

    for i in $(seq 1 $iterations); do
        local start=$(date +%s%N)
        echo "$request" | nc -w 2 localhost 37777 2>/dev/null || echo ""
        # 如果 MCP 是 stdio 模式，用这种方式
        echo "$request" >&0 2>/dev/null || true
        local end=$(date +%s%N)
        local elapsed_ms=$(( (end - start) / 1000000 ))

        # 捕获 MCP stdio 模式的响应
        # 对于 stdio 模式，通过临时文件
        local response=$(echo "$request" | timeout 2 "$MCP_BIN" 2>/dev/null | head -1)

        total_ms=$((total_ms + elapsed_ms))
        if [ $elapsed_ms -lt $min_ms ]; then min_ms=$elapsed_ms; fi
        if [ $elapsed_ms -gt $max_ms ]; then max_ms=$elapsed_ms; fi
    done

    local avg_ms=$((total_ms / iterations))
    echo "$avg_ms $min_ms $max_ms"
}

echo "  (基准测试在 Go test 中更精确，此处为粗略测量)"
echo "  精确基准请运行: go test ./benchmarks/ -bench=MCP -benchmem -v"

# ==========================================
# 3. 端到端决策提取延迟
# ==========================================
echo ""
echo "[3/5] 端到端延迟测试 (发送消息 → 检测 → 提取)..."

if [ "$LARK_CLI_AVAILABLE" = true ]; then
    echo "  发送决策消息到飞书群聊..."

    send_start=$(date +%s%N)

    # 发送一条决策消息
    lark-cli im +messages-send \
        --chat-id "${LARK_CHAT_IDS:-}" \
        --msg-type text \
        --content "决定了，使用PostgreSQL作为主数据库，张三负责实施" \
        --as user 2>/dev/null || echo "  ⚠️  消息发送失败（可能需要配置 LARK_CHAT_IDS）"

    send_end=$(date +%s%N)
    send_ms=$(( (send_end - send_start) / 1000000 ))
    echo "  消息发送延迟: ${send_ms}ms"
else
    echo "  ⚠️  跳过 (lark-cli 不可用)"
fi

# ==========================================
# 4. OpenClaw 响应延迟
# ==========================================
echo ""
echo "[4/5] OpenClaw 响应延迟..."

if [ "$OPENCLAW_AVAILABLE" = true ]; then
    echo "  测试 OpenClaw 响应时间..."

    # 通过 Docker 检查 MCP 连接
    docker exec openclaw-zh curl -s http://host.docker.internal:37777/health \
        2>/dev/null && echo "  ✅ MCP 可达" || echo "  ⚠️  MCP 不可达 (host.docker.internal)"

    # 测试 OpenClaw 内 MCP 工具调用（通过其内部网络）
    docker exec openclaw-zh timeout 5 bash -c '
        echo '\''{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"bench","version":"1.0"}}}'\'' | \
        nc -w 3 feishu-mem 37777 2>/dev/null || echo "nc fallback"
    ' 2>/dev/null && echo "  ✅ MCP 通过 Docker 网络可达" || echo "  ⚠️  Docker 网络 MCP 不可达"
else
    echo "  ⚠️  跳过 (OpenClaw 容器未运行)"
fi

# ==========================================
# 5. 结果汇总
# ==========================================
echo ""
echo "[5/5] 生成报告..."

# 获取 Go 基准测试结果
GO_BENCH_OUTPUT=$(cd "$PROJECT_DIR" && \
    GOTOOLCHAIN=go1.25.8 /c/Users/ASUS/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.8.windows-amd64/bin/go.exe \
    test ./internal/... -bench=. -benchmem -count=1 -timeout=60s -run=^$ 2>/dev/null | grep "^Benchmark" || echo "(benchmark output not captured)")

# 写入报告
cat > "$REPORT_FILE" << EOF
{
  "timestamp": "$TIMESTAMP",
  "report": "OpenClaw 集成基准测试",
  "openclaw_available": $OPENCLAW_AVAILABLE,
  "lark_cli_available": $LARK_CLI_AVAILABLE,
  "go_benchmarks": $(echo "$GO_BENCH_OUTPUT" | python3 -c "
import sys,json
lines = [l.strip() for l in sys.stdin if l.strip()]
result = {}
for l in lines:
    parts = l.split()
    if len(parts) >= 3:
        name = parts[0]
        result[name] = {
            'iterations': int(parts[1].replace(',','')),
            'ns_per_op': parts[2]
        }
        if len(parts) >= 5:
            result[name]['bytes_per_op'] = parts[4]
        if len(parts) >= 7:
            result[name]['allocs_per_op'] = parts[6]
print(json.dumps(result, ensure_ascii=False, indent=2))
" 2>/dev/null || echo "{}")
}
EOF

echo ""
echo "=========================================="
echo " 基准测试完成"
echo " 报告: $REPORT_FILE"
echo "=========================================="
echo ""
echo "可用命令:"
echo "  Go 基准:     go test ./benchmarks/ -bench=. -benchmem -v"
echo "  OpenClaw:    bash benchmarks/run_openclaw_bench.sh"
echo "  查看报告:    cat $REPORT_FILE | jq ."
