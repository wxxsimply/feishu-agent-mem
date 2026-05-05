# MCP SDK 集成文档

## 概述

本项目已成功迁移到使用官方的 `modelcontextprotocol/go-sdk`，替代了之前手写的 MCP 协议实现。

## 已完成的工作

### 1. SDK 集成
- ✅ 下载并配置 `github.com/modelcontextprotocol/go-sdk`
- ✅ 更新 go.mod，增加 SDK 依赖
- ✅ 成功编译并运行

### 2. MCP 服务器重构
- ✅ 创建 `internal/mcp/server/` 包结构
- ✅ 基于 SDK API 实现服务器
- ✅ 实现 8 个工具：
  - `search` - 搜索决策记录
  - `topic` - 查询议题下的决策
  - `decision` - 获取单个决策详情
  - `timeline` - 获取决策时间线
  - `create_decision` - 创建新决策
  - `update_decision` - 更新已有决策
  - `extract_decision` - 从文本提取决策
- ✅ 实现 2 个资源：
  - `docs://design` - 系统设计文档
  - `docs://prompts` - LLM 提示词模板

### 3. 客户端 SDK
- ✅ 创建 `pkg/mcpclient/` 包
- ✅ 封装常用操作

### 4. 配置和部署
- ✅ mcporter 配置示例
- ✅ Claude Desktop 配置示例
- ✅ 编译成功，二进制文件位于 `bin/mcp-server`

## 项目结构

```
feishu-agent-mem/
├── cmd/mcp-server/
│   └── main.go              # MCP 服务器入口
├── internal/mcp/server/
│   └── server.go            # 基于 SDK 的 MCP 服务器实现
├── pkg/mcpclient/
│   └── client.go            # MCP 客户端 SDK
├── docs/examples/
│   ├── mcporter-config.yaml     # mcporter 配置示例
│   └── claude-desktop-config.json # Claude Desktop 配置示例
└── bin/mcp-server          # 编译好的二进制文件
```

## 编译和运行

### 编译服务器
```bash
# 在 Docker 容器中
docker exec -it openclaw-zh bash
cd /root/openclaw-workspace/feishu-agent-mem
export PATH=/usr/local/go/bin:$PATH
go build -o bin/mcp-server ./cmd/mcp-server/main.go
```

### 运行服务器
```bash
# 直接运行（stdio 模式）
bin/mcp-server
```

## 配置说明

### Claude Desktop 配置
在 Claude Desktop 的配置文件中添加：
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

位置：
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

### mcporter 配置
见 `docs/examples/mcporter-config.yaml`

## 可用的 MCP 工具

### 1. search
搜索记忆系统中的决策记录

**参数：**
- `query`: 搜索关键词
- `topic`: 议题过滤（可选）
- `limit`: 结果限制（可选）

### 2. topic
查询指定议题的所有决策记录

**参数：**
- `topic`: 议题名称

### 3. decision
获取单个决策的详细信息

**参数：**
- `sdr_id`: 决策 ID

### 4. timeline
获取决策历史时间线

**参数：** 无

### 5. create_decision
创建新决策

**参数：**
- `title`: 决策标题
- `decision`: 决策内容
- `rationale`: 决策依据（可选）
- `topic`: 议题
- `phase`: 阶段（可选）
- `impact_level`: 影响等级（可选）

### 6. update_decision
更新已有决策

**参数：**
- `sdr_id`: 决策 ID
- `title`: 决策标题（可选）
- `decision`: 决策内容（可选）
- `rationale`: 决策依据（可选）
- `status`: 状态（可选）

### 7. extract_decision
从文本内容中提取决策信息（当前为简化版本）

**参数：**
- `content`: 待分析文本
- `topics`: 候选议题（可选）

## 可用的 MCP 资源

### docs://design
系统设计文档

### docs://prompts
LLM 提示词模板

## 后续工作

### 短期
- [ ] 完善客户端 SDK（修复 API 调用问题）
- [ ] 添加 Prompts 功能
- [ ] 添加更多测试
- [ ] 完善 LLM 工具（集成真实的 LLM）

### 长期
- [ ] HTTP/SSE 传输模式
- [ ] WebSocket 传输模式
- [ ] 更丰富的工具集
- [ ] 性能优化
- [ ] 监控和指标

## 优势

使用官方 SDK 的优势：
1. **协议兼容性** - 完整兼容 MCP 规范
2. **维护成本低** - 社区维护，自动跟进协议变更
3. **功能完整** - 包含所有标准功能
4. **更好的互操作性** - 与其他 MCP 客户端/服务器兼容
