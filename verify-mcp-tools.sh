#!/bin/bash
# 验证 MCP server 工具是否可用

cd /root/openclaw-workspace/feishu-agent-mem

echo "============================================="
echo "  MCP Server 工具验证"
echo "============================================="
echo ""

# 1. 检查二进制文件
echo "[1/5] 检查二进制文件..."
if [ ! -f bin/mcp-server ]; then
    echo "❌ bin/mcp-server 未找到"
    exit 1
fi
echo "✅ bin/mcp-server 已就绪 ($(du -h bin/mcp-server))"

# 2. 检查是否可执行
echo ""
echo "[2/5] 检查执行权限..."
if [ ! -x bin/mcp-server ]; then
    chmod +x bin/mcp-server
    echo "ℹ️ 已添加执行权限"
fi
echo "✅ 执行权限正常"

# 3. 创建测试输入
echo ""
echo "[3/5] 创建测试请求..."
cat > /tmp/mcp-test-input.json << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF

# 4. 运行服务器并获取工具列表
echo ""
echo "[4/5] 测试服务器并获取工具列表..."
echo "   (超时时间: 3秒)..."

# 使用超时运行并捕获输出
TIMEOUT=3
OUTPUT=$(timeout $TIMEOUT bin/mcp-server < /tmp/mcp-test-input.json 2>&1 || true)

# 5. 解析并显示工具
echo ""
echo "[5/5] 分析工具列表..."
echo ""

echo "=== 服务器日志 ==="
echo "$OUTPUT" | grep -E "(Feishu Memory|Available|Starting|⚠️|❌)" || echo "$OUTPUT" | head -30
echo ""

echo "=== 工具列表 ==="
# 尝试从输出中提取工具名称
TOOLS=$(echo "$OUTPUT" | grep -o '"name":"[^"]*"' | sed 's/"name":"//;s/"$//' | sort -u)

if [ -z "$TOOLS" ]; then
    echo "⚠️ 无法自动解析工具列表"
    echo ""
    echo "手动检查方式:"
    echo "  1. 运行: bin/mcp-server"
    echo "  2. 发送 initialize + tools/list 请求"
else
    COUNT=$(echo "$TOOLS" | wc -l | xargs)
    echo "✅ 共发现 $COUNT 个工具:"
    echo ""
    echo "$TOOLS" | nl -w2 -s". "
fi

echo ""
echo "============================================="
echo "  验证完成!"
echo "============================================="
echo ""
echo "📝 使用方式:"
echo "  1. 配置 OpenClaw 使用此 MCP server:"
echo "     ./configure-mcp.sh"
echo ""
echo "  2. 或在 Claude Desktop 中配置:"
echo "     command: /root/openclaw-workspace/feishu-agent-mem/bin/mcp-server"
echo ""
echo "  3. 测试工具:"
echo "     使用 'list_topics' 查看所有议题"
echo "     使用 'stats' 查看系统统计"
echo "     使用 'hot_decisions' 查看热点决策"
echo ""
