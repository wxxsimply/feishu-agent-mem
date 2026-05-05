# OpenClaw MCP 配置指南

基于 https://docs.openclaw.ac.cn/cli/mcp

## 快速配置步骤

### 方法 1: 使用 OpenClaw CLI（推荐）

```bash
# 进入 Docker 容器
docker exec -it openclaw-zh bash

# 设置 MCP 服务器
openclaw mcp set feishu-memory-agent '{
  "command": "/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server",
  "args": [],
  "env": {
    "CONFIG_PATH": "/root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml"
  },
  "cwd": "/root/openclaw-workspace/feishu-agent-mem"
}'

# 验证配置
openclaw mcp list
openclaw mcp show feishu-memory-agent --json
```

### 方法 2: 直接编辑 openclaw.json

在配置文件中添加：
```json
{
  ...其他配置...,
  "mcp": {
    "servers": {
      "feishu-memory-agent": {
        "command": "/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server",
        "args": [],
        "env": {
          "CONFIG_PATH": "/root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml"
        },
        "cwd": "/root/openclaw-workspace/feishu-agent-mem"
      }
    }
  }
}
```

## 可用的 MCP 工具

我们的 MCP 服务器提供以下工具：

1. **search** - 搜索记忆系统中的决策记录
2. **topic** - 查询指定议题的所有决策记录
3. **decision** - 获取单个决策的详细信息
4. **timeline** - 获取决策历史时间线
5. **create_decision** - 创建新决策
6. **update_decision** - 更新已有决策
7. **extract_decision** - 从文本中提取决策信息（简化版）

## 可用的资源

1. **docs://design** - 系统设计文档
2. **docs://prompts** - LLM 提示词模板

## 完整的一键配置脚本

在 Docker 容器内执行：
```bash
cat > /tmp/config-mcp.sh << 'EOF'
#!/bin/bash

echo "Configuring OpenClaw MCP..."

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

echo "Setting feishu-memory-agent..."
openclaw mcp set feishu-memory-agent "$CONFIG"

echo ""
echo "✅ Configured!"
echo ""
echo "Listing MCP servers..."
openclaw mcp list
EOF

chmod +x /tmp/config-mcp.sh
/tmp/config-mcp.sh
```
