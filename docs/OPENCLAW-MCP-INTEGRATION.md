# OpenClaw MCP 集成指南

## 概述

本文档介绍如何将飞书记忆代理的 MCP 服务器集成到 OpenClaw 中。

## 现状

✅ MCP 服务器已编译成功
✅ 服务器位置: `/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server`
✅ 配置文件已创建

## 配置方法

### 方式 1: 通过 OpenClaw 界面配置 (推荐)

1. 打开 OpenClaw 界面
2. 进入设置或插件管理
3. 找到 MCP 或 Tools 配置选项
4. 添加新的 MCP 服务器:
   - 名称: `feishu-memory-agent`
   - 命令: `/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server`
   - 环境变量:
     - `CONFIG_PATH`: `/root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml`

### 方式 2: 配置文件

MCP 配置可能位于以下位置之一:

```bash
# 位置 1: .mcporter 目录
/root/.mcporter/config.yaml

# 位置 2: .config/mcporter 目录  
/root/.config/mcporter/config.yaml

# 位置 3: .openclaw 下的某个配置目录
# (需要查看 OpenClaw 文档确认)
```

当前我们的配置已经放置在:
- `/root/.mcporter/config.yaml`
- `/root/.config/mcporter/config.yaml`

### 方式 3: 如果 OpenClaw 使用 Claude Desktop 风格配置

创建或编辑文件:
```json
{
  "mcpServers": {
    "feishu-memory-agent": {
      "command": "/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server",
      "env": {
        "CONFIG_PATH": "/root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml"
      }
    }
  }
}
```

可能的位置:
- `/root/.openclaw/mcp.json`
- `/root/.config/Claude/claude_desktop_config.json`
- 或其他 OpenClaw 配置目录

## 验证配置

配置完成后，可以按以下步骤验证:

### 1. 验证服务器可以启动

```bash
docker exec -it openclaw-zh bash
cd /root/openclaw-workspace/feishu-agent-mem
./bin/mcp-server
```

应该看到日志输出:
```
time=... level=INFO msg="Loading config..."
time=... level=INFO msg="Creating MCP server..."
time=... level=INFO msg="Feishu Memory MCP Server starting..."
time=... level=INFO msg="Starting MCP server"
```

### 2. 查看 OpenClaw 日志

查看 OpenClaw 的日志，看是否有 MCP 相关的输出:

```bash
cd /root/.openclaw/logs
ls -la
tail -f *.log
```

### 3. 测试工具调用

在 OpenClaw 中尝试使用 MCP 工具:
- 搜索: "用 search 工具查找关于数据库的决策"
- 列表: "列出所有可用的工具"

## 可用的 MCP 工具

### 工具列表

1. **search** - 搜索记忆系统中的决策记录
   - 参数: `query` (搜索关键词), `topic` (可选), `limit` (可选)

2. **topic** - 查询指定议题的所有决策记录
   - 参数: `topic` (议题名称)

3. **decision** - 获取单个决策的详细信息
   - 参数: `sdr_id` (决策 ID)

4. **timeline** - 获取决策历史时间线
   - 参数: 无

5. **create_decision** - 创建新决策
   - 参数: `title`, `decision`, `topic`, `rationale` (可选), `phase` (可选), `impact_level` (可选)

6. **update_decision** - 更新已有决策
   - 参数: `sdr_id`, `title` (可选), `decision` (可选), `rationale` (可选), `status` (可选)

7. **extract_decision** - 从文本中提取决策信息
   - 参数: `content` (待分析文本), `topics` (可选)

### 资源列表

1. **docs://design** - 系统设计文档
2. **docs://prompts** - LLM 提示词模板

## 故障排除

### 问题 1: MCP 服务器无法启动

检查:
- 二进制文件是否存在: `ls -la /root/openclaw-workspace/feishu-agent-mem/bin/mcp-server`
- 文件是否有执行权限: `chmod +x /root/openclaw-workspace/feishu-agent-mem/bin/mcp-server`
- 尝试手动运行查看错误

### 问题 2: OpenClaw 无法连接到 MCP 服务器

检查:
- 配置文件位置是否正确
- 路径是否是绝对路径
- 查看 OpenClaw 日志中的错误信息

### 问题 3: 工具无法调用

检查:
- 服务器是否正确初始化
- 查看 MCP 协议通信日志

## 下一步

1. ✅ 确认 OpenClaw 能看到 MCP 服务器
2. ✅ 测试基本工具调用 (`search`, `listTools`)
3. ✅ 创建一个测试决策验证功能
4. ✅ 配置 Git 存储持久化
5. ✅ 集成真实的 LLM (豆包大模型)

## 相关文件

- `config/mcporter.yaml` - MCP 配置示例
- `docs/examples/mcporter-config.yaml` - 另一个配置示例
- `docs/examples/claude-desktop-config.json` - Claude Desktop 配置风格
- `docs/MCP-SDK-INTEGRATION.md` - MCP SDK 集成文档
