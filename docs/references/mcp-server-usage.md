# MCP 服务器调用指南

## 概述

本项目提供两种 MCP 服务器启动方式：
1. **独立 MCP 服务器模式** - 推荐用于 Claude Desktop 等 MCP 客户端
2. **混合服务模式** - 作为完整服务运行，同时支持 MCP 功能

## 方式一：独立 MCP 服务器（推荐）

### 1. 编译

```bash
cd /path/to/feishu-agent-mem
go build -o bin/mcp-server ./cmd/mcp-server
```

### 2. 配置 Claude Desktop

#### macOS 配置位置
```
~/Library/Application Support/Claude/claude_desktop_config.json
```

#### Windows 配置位置
```
%APPDATA%\Claude\claude_desktop_config.json
```

#### Linux 配置位置
```
~/.config/Claude/claude_desktop_config.json
```

### 3. 配置内容

```json
{
  "mcpServers": {
    "feishu-memory": {
      "command": "/path/to/feishu-agent-mem/bin/mcp-server",
      "args": [],
      "env": {
        "CONFIG_PATH": "/path/to/feishu-agent-mem/config/openclaw.yaml",
        "GIT_WORK_DIR": "/path/to/git/repo"
      }
    }
  }
}
```

### 4. 重启 Claude Desktop

配置完成后重启 Claude Desktop，即可在聊天中使用 Feishu Memory 工具。

## 方式二：Docker 容器运行

### Dockerfile (可选)

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download
RUN go build -o /mcp-server ./cmd/mcp-server

FROM alpine:latest

COPY --from=builder /mcp-server /mcp-server
COPY config /config

CMD ["/mcp-server"]
```

### 配置 Claude Desktop 使用 Docker

```json
{
  "mcpServers": {
    "feishu-memory": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-v", "/path/to/config:/config",
        "-v", "/path/to/git:/git",
        "feishu-memory-mcp:latest"
      ],
      "env": {
        "CONFIG_PATH": "/config/openclaw.yaml"
      }
    }
  }
}
```

## 可用工具列表

### 1. search - 搜索决策
搜索记忆系统中的决策记录

**参数：**
- `query` (string, 可选): 搜索关键词
- `topic` (string, 可选): 议题过滤
- `limit` (number, 可选): 结果限制，默认20

**示例：**
```json
{
  "name": "search",
  "arguments": {
    "query": "数据库",
    "limit": 10
  }
}
```

### 2. topic - 查询议题
获取指定议题下的所有决策

**参数：**
- `topic` (string, 必填): 议题名称

### 3. decision - 决策详情
获取单个决策的详细信息

**参数：**
- `sdr_id` (string, 必填): 决策 ID

### 4. extract_decision - 提取决策（LLM）
从文本内容中智能提取决策信息

**参数：**
- `content` (string, 必填): 待分析文本
- `topics` (string[], 可选): 候选议题列表

**示例：**
```json
{
  "name": "extract_decision",
  "arguments": {
    "content": "我们决定使用 PostgreSQL 作为主数据库，负责人是张三",
    "topics": ["数据库架构", "基础设施"]
  }
}
```

### 5. classify_topic - 分类议题（LLM）
将决策智能分类到正确的议题

**参数：**
- `decision` (string, 必填): 决策内容
- `topics` (string[], 必填): 候选议题列表

### 6. detect_crosstopic - 跨议题检测（LLM）
检测决策是否影响多个议题

**参数：**
- `title` (string, 必填): 决策标题
- `decision` (string, 必填): 决策内容
- `rationale` (string, 可选): 决策依据
- `candidate_topics` (string[], 必填): 候选议题列表

### 7. check_conflict - 冲突评估（LLM）
评估两个决策之间是否存在冲突

**参数：**
- `decision_a` (string, 必填): 新决策
- `decision_b` (string, 必填): 已有决策

### 8. timeline - 时间线
获取决策历史时间线

**参数：** 无

## 可用资源列表

### docs://design
系统设计文档

### docs://prompts
LLM 提示词模板

## 完整示例对话

### 示例1：搜索决策
```
User: 我们之前做了哪些关于数据库的决定？

Claude: [调用 search 工具，query="数据库"]

Claude: 根据我的搜索，你们之前做了以下关于数据库的决定：
1. 选择 PostgreSQL 作为主数据库 - 数据库架构
2. 配置数据库主从复制 - 数据库架构
```

### 示例2：提取决策
```
User: 这是会议纪要，请帮我提取决策：
"今天讨论了缓存方案，我们决定使用 Redis Cluster，由李四负责实施。"

Claude: [调用 extract_decision 工具，content="..."]

Claude: 我提取到以下决策：
- 标题：选择 Redis Cluster 作为缓存方案
- 决定：使用 Redis Cluster
- 建议议题：缓存架构
- 负责人：李四
```

### 示例3：冲突检测
```
User: 这两个决策有冲突吗？
决策A: 使用 PostgreSQL 作为主数据库
决策B: 使用 MySQL 作为主数据库

Claude: [调用 check_conflict 工具]

Claude: 检测到冲突！
- 冲突分数：0.9（直接冲突）
- 冲突类型：direct
- 建议：选择其中一个方案
```

## 调试与故障排除

### 查看服务器日志
Claude Desktop 的 MCP 服务器日志通常在以下位置：

#### macOS
```
~/Library/Logs/Claude/mcp-server-feishu-memory-.log
```

#### Windows
```
%APPDATA%\Claude\Logs\mcp-server-feishu-memory-.log
```

### 手动测试 MCP 协议

你可以使用以下方式手动测试：

```bash
# 使用提供的输入测试
cat << 'EOF' | ./bin/mcp-server
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
EOF
```

### 常见问题

**Q: 服务器启动失败？**
- 检查 CONFIG_PATH 是否正确
- 检查 Git 仓库路径是否存在
- 查看 Claude Desktop 的 MCP 日志

**Q: LLM 功能不工作？**
- 检查是否有 .env 文件配置了 ARK_API_KEY
- LLM 不可用时会自动使用降级策略（关键词匹配）

**Q: 决策数据没加载？**
- 检查 Git 仓库路径配置
- 确认 decisions/ 目录下有决策文件
- 检查内存图是否正确加载

## 高级配置

### 在 VS Code 中使用

如果你使用的是 VS Code 的 Claude 插件，配置位置类似：

#### macOS
```
~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings.json
```

### 同时运行多个 MCP 服务器

在配置文件中添加多个配置：

```json
{
  "mcpServers": {
    "feishu-memory": {
      "command": "/path/to/mcp-server",
      "env": {...}
    },
    "another-server": {
      "command": "/path/to/another",
      "env": {...}
    }
  }
}
```
