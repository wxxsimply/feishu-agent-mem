#!/bin/bash
# 正确测试 MCP server 工具列表

cd /root/openclaw-workspace/feishu-agent-mem

echo "============================================="
echo "  MCP 工具完整测试"
echo "============================================="
echo ""

# 创建命名管道
rm -f /tmp/mcp-in /tmp/mcp-out
mkfifo /tmp/mcp-in /tmp/mcp-out

# 启动服务器
echo "[1/4] 启动 MCP server..."
bin/mcp-server < /tmp/mcp-in > /tmp/mcp-out 2>&1 &
SERVER_PID=$!
echo "   Server PID: $SERVER_PID"
sleep 0.5

# 发送 initialize
echo ""
echo "[2/4] 发送初始化请求..."
cat > /tmp/mcp-in << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
EOF

# 等待响应
sleep 0.5
echo "   ✅ 已发送 initialize"

# 发送 initialized 通知
echo ""
echo "[3/4] 发送 initialized 通知..."
cat > /tmp/mcp-in << 'EOF'
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
EOF
sleep 0.3

# 请求工具列表
echo ""
echo "[4/4] 请求工具列表..."
cat > /tmp/mcp-in << 'EOF'
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF

# 等待响应
sleep 1

# 读取输出并清理
echo ""
echo "=== 服务器响应 ==="
echo ""
cat /tmp/mcp-out | head -100

# 清理
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
rm -f /tmp/mcp-in /tmp/mcp-out

echo ""
echo "============================================="
echo "  测试完成!"
echo "============================================="
echo ""
echo "📝 如要在 OpenClaw 中使用:"
echo "  ./configure-mcp.sh"
echo ""
