#!/bin/bash
# One-click MCP configuration script for OpenClaw

echo "============================================="
echo "  OpenClaw MCP Configuration"
echo "============================================="
echo ""

cd /root

# Create our config
echo "Creating MCP configuration..."
CONFIG=$(cat << 'JSON'
{
  "command": "/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server",
  "args": [],
  "env": {
    "CONFIG_PATH": "/root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml"
  },
  "cwd": "/root/openclaw-workspace/feishu-agent-mem"
}
JSON
)

# Set the MCP server
echo "Setting feishu-memory-agent..."
openclaw mcp set feishu-memory-agent "$CONFIG"

# Verify
echo ""
echo "✅ Configuration done!"
echo ""
echo "Listing configured MCP servers:"
openclaw mcp list

echo ""
echo "============================================="
echo "  Done!"
echo "============================================="
echo ""
echo "MCP server 'feishu-memory-agent' is now configured!"
echo ""
echo "To view full config:"
echo "  openclaw mcp show feishu-memory-agent --json"
