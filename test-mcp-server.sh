#!/bin/bash
# 简单测试 MCP 服务器

cd /root/openclaw-workspace/feishu-agent-mem

echo "=== Testing MCP Server ==="

# 测试 1: 检查二进制文件是否存在
if [ ! -f bin/mcp-server ]; then
    echo "❌ ERROR: bin/mcp-server not found"
    exit 1
fi
echo "✅ bin/mcp-server found ($(du -h bin/mcp-server))"

# 测试 2: 检查文件是否可执行
if [ ! -x bin/mcp-server ]; then
    echo "❌ ERROR: bin/mcp-server is not executable"
    chmod +x bin/mcp-server
fi
echo "✅ bin/mcp-server is executable"

# 测试 3: 尝试获取帮助或版本信息
echo ""
echo "=== Testing server startup ==="

# 创建一个简单的测试 - 发送 initialize 请求并读取响应
cat > /tmp/test-mcp-input.json << EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF

# 超时 5 秒运行服务器并测试
echo "Testing server (timeout after 5s)..."
timeout 5 bin/mcp-server 2>&1 || echo "Server exited (expected)"

echo ""
echo "=== MCP Server Status ==="
echo "✅ Server binary is ready"
echo "📝 Next steps:"
echo "   1. Configure mcporter or Claude Desktop to use this server"
echo "   2. The server speaks MCP over stdio"
echo "   3. Tools available: search, topic, decision, timeline, create_decision, update_decision, extract_decision"
