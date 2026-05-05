# OpenClaw 集成测试指南

## ✅ MCP 服务器测试状态

已验证：
- ✅ `mem-service --mode=mcp` 正常启动
- ✅ initialize 请求响应正确
- ✅ Server Info: `Feishu Memory Agent v2.0.0`

## 🚀 在 OpenClaw 中启用

### 方式 1: 直接在 OpenClaw 配置中添加 MCP 服务器

您的 `~/.openclaw/openclaw.json` 已经配置好 MCP 服务器了！

配置内容：
```json
"mcp": {
  "servers": {
    "feishu-memory-agent": {
      "command": "/root/openclaw-workspace/feishu-agent-mem/bin/mem-service",
      "args": ["--mode=mcp"],
      "env": {
        "CONFIG_PATH": "/root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml"
      },
      "cwd": "/root/openclaw-workspace/feishu-agent-mem"
    }
  }
}
```

### 方式 2: 作为插件加载

1. 将 `plugin.json` 复制到 OpenClaw 的插件目录，或者
2. 在 OpenClaw 配置中添加插件加载路径：
   ```json
   "plugins": {
     "load": {
       "paths": [
         "/Users/halllo/openclaw-workspace/feishu-agent-mem"
       ]
     }
   }
   ```

## 🧪 测试 MCP 工具

重启 OpenClaude / Claude Desktop 后，您应该能看到这些 MCP 工具：

- `search` - 搜索记忆系统中的决策记录
- `topic` - 查询指定议题的所有决策记录
- `decision` - 获取单个决策的详细信息
- `timeline` - 获取决策历史时间线
- `create_decision` - 创建新决策
- `update_decision` - 更新已有决策
- `extract_decision` - 从文本内容中智能提取决策信息

## 📝 测试步骤

1. **重启 OpenClaw**（或 Claude Desktop）
2. 查看是否有新的 MCP 工具可用
3. 尝试使用 `search` 工具
4. 尝试使用 `create_decision` 创建一个测试决策

## 🔍 调试

如果 MCP 服务器没有正常工作：

1. 检查 OpenClaw 日志：`~/.openclaw/logs/`
2. 手动测试命令：
   ```bash
   docker exec -it openclaw-zh bash
   cd /root/openclaw-workspace/feishu-agent-mem
   ./bin/mem-service --mode=mcp
   ```
3. 确保 config/openclaw.yaml 存在且配置正确
