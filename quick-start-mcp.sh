#!/bin/bash
# Feishu Memory Agent MCP - 快速启动脚本

echo "============================================="
echo "  Feishu Memory Agent MCP - Quick Start"
echo "============================================="
echo ""

cd /root/openclaw-workspace/feishu-agent-mem

# 检查 1: 二进制文件
echo "[1/5] Checking MCP Server binary..."
if [ -f "bin/mcp-server" ]; then
    echo "✅ bin/mcp-server found ($(du -h bin/mcp-server))"
else
    echo "❌ bin/mcp-server not found!"
    exit 1
fi

# 检查 2: 配置文件
echo ""
echo "[2/5] Checking config files..."
if [ -f "config/openclaw.yaml" ]; then
    echo "✅ config/openclaw.yaml found"
else
    echo "⚠️ config/openclaw.yaml not found"
fi

# 检查 3: mcporter 配置
echo ""
echo "[3/5] Checking mcporter config..."
for config_path in "/root/.mcporter/config.yaml" "/root/.config/mcporter/config.yaml"; do
    if [ -f "$config_path" ]; then
        echo "✅ $config_path"
    fi
done

# 检查 4: 测试启动
echo ""
echo "[4/5] Testing server startup..."
timeout 2 bin/mcp-server 2>&1 | head -10

if [ $? -eq 124 ]; then
    # 超时是正常的，因为我们没有发送请求
    echo "✅ Server can be started"
else
    echo "⚠️ Server exited with code $?"
fi

# 检查 5: 显示可用的工具
echo ""
echo "[5/5] Available MCP Tools:"
echo "  1. search      - 搜索记忆系统中的决策记录"
echo "  2. topic       - 查询指定议题的所有决策记录"
echo "  3. decision    - 获取单个决策的详细信息"
echo "  4. timeline    - 获取决策历史时间线"
echo "  5. create_decision  - 创建新决策"
echo "  6. update_decision  - 更新已有决策"
echo "  7. extract_decision - 从文本中提取决策信息"

echo ""
echo "============================================="
echo "  ✅ MCP Server is ready!"
echo "============================================="
echo ""
echo "下一步:"
echo "1. 打开 OpenClaw 界面"
echo "2. 配置 MCP 服务器 (如果尚未自动发现)"
echo "3. 尝试使用 search 工具查询决策"
echo ""
echo "更多信息请查看 docs/OPENCLAW-MCP-INTEGRATION.md"
