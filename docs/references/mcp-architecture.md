# MCP Server 架构与飞书客户端关系

## 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Claude Desktop (MCP Client)                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │  LLM (Claude Opus)                                        │ │
│  │  - 理解用户意图                                           │ │
│  │  - 决定调用哪个 MCP 工具                                 │ │
│  │  - 处理工具返回结果                                       │ │
│  └─────────────────────┬───────────────────────────────────────┘ │
└────────────────────────┼───────────────────────────────────────────┘
                         │ JSON-RPC 2.0 协议
                         │ (stdio / stdin/stdout)
                         │
┌────────────────────────▼───────────────────────────────────────────┐
│          Feishu Memory MCP Server (Go 实现)                   │
│                                                                 │
│  ┌──────────────────────┐ ┌───────────────────────────────┐    │
│  │   Tools (8个工具)   │ │   Resources (2个资源)        │    │
│  │  - search           │ │  - docs://design             │    │
│  │  - topic            │ │  - docs://prompts            │    │
│  │  - decision         │ └───────────────────────────────┘    │
│  │  - extract_decision ├─────────────┐                        │
│  │  - classify_topic   │             │                        │
│  │  - detect_crosstopic│             │                        │
│  │  - check_conflict   │             │                        │
│  │  - timeline         │             │                        │
│  └──────────────────────┘             │                        │
│                                      │                        │
│  ┌──────────────────────────────────▼────────────────────────┐  │
│  │               LLM Module (internal/llm)                   │  │
│  │  - MemoryAgent (主入口)                                   │  │
│  │  - Prompt Manager (提示词管理)                            │  │
│  │  - 4种 LLM 处理能力                                       │  │
│  │    - 决策提取 (extract_decision)                         │  │
│  │    - 议题分类 (classify_topic)                           │  │
│  │    - 跨议题检测 (detect_crosstopic)                       │  │
│  │    - 冲突评估 (check_conflict)                            │  │
│  └──────────────────────────────────┬────────────────────────┘  │
│                                     │                             │
│                         ┌───────────▼───────────────┐         │
│                         │  Memory Graph (core)      │         │
│                         │  - 决策节点存储           │         │
│                         │  - 索引与查询             │         │
│                         └───────────┬───────────────┘         │
│                                     │                             │
│                    ┌────────────────┼─────────────────┐        │
│                    ▼                ▼                ▼        │
│              ┌──────────┐    ┌──────────┐    ┌───────────┐   │
│              │Git Storage│    │ Bitable  │    │(可选)飞书│   │
│              └──────────┘    └──────────┘    │Adapter     │   │
│                                                └───────────┘   │
└────────────────────────────────────────────────────────────────┘
                         │
┌─────────────────────────▼────────────────────────────┐
│            Lark (飞书) - 外部服务                   │
│  - Lark IM (消息)                                    │
│  - Lark Calendar (日历)                              │
│  - Lark Wiki (文档)                                  │
│  - Lark VC (会议)                                    │
│  - Lark Task (任务)                                  │
│  - Lark OKR                                          │
│  - Lark Minutes (会议纪要)                           │
│  - Lark Contacts (联系人)                            │
└──────────────────────────────────────────────────────┘
```

## 组件关系详解

### 1. Claude Desktop ↔ MCP Server

**通信方式**
- 协议：JSON-RPC 2.0
- 传输：stdio (标准输入/输出)
- 流程：
  1. Claude 发送请求到 stdout → 被 MCP Server 接收
  2. MCP Server 处理请求
  3. MCP Server 发送响应到 stdin → Claude 读取

**能力交换**
```
初始化流程:
1. Claude → initialize 请求 (包含 client capabilities)
2. MCP Server → initialize 响应 (包含 server capabilities)
3. Claude: 我知道你有8个工具、2个资源
4. Claude: 现在可以开始调用工具了
```

### 2. MCP Server ↔ LLM Module

**集成方式**
- MCP Server 调用内部 LLM 模块
- LLM 模块内部调用 Volcengine Ark API (豆包大模型)
- 降级策略：LLM 不可用时使用关键词匹配

**工具调用流程**
```
用户: "帮我从这段文字里提取决策"

Claude: 
  分析用户请求 → 需要调用 extract_decision 工具
  调用 tools/call: {name: "extract_decision", arguments: {...}}
  ↓
MCP Server:
  收到请求 → 调用内部 MemoryAgent.ExtractDecision()
  ↓
LLM Module:
  构建提示词 (Prompt Manager)
  调用 Volcengine Ark API
  解析响应
  ↓
MCP Server:
  格式化结果为 Text Content
  返回给 Claude
  ↓
Claude:
  展示结果给用户
```

### 3. MCP Server ↔ Memory Graph

**数据存储**
- Memory Graph 是内存中的数据结构
- 存储决策节点、议题组织
- 支持查询、搜索、索引

**Git Storage**
- 持久化 Memory Graph 数据
- 版本管理、历史记录
- 从 Git 加载已有决策

### 4. MCP Server ↔ 飞书适配器

**关系说明**
- MCP Server 主要用于查询/处理已有的决策
- 飞书适配器 (lark-adapter) 主要用于从飞书获取新数据
- 两者是协作关系，不是依赖关系

**数据流程**
```
飞书事件 (新消息/文档变更):
  ↓
飞书适配器 (lark-adapter)
  ↓
提取决策
  ↓
Signal Engine
  ↓
写入 Memory Graph
  ↓
通过 Git Storage 持久化
  ↓
用户通过 MCP Server 查询决策
```

## 完整的端到端流程示例

### 场景1：从飞书消息提取决策

```
时间线:

T1: 飞书群聊
  张三: "我们决定用 PostgreSQL 作为主数据库，张四负责"

T2: 飞书适配器 (后台服务)
  检测到新消息
  调用 LLM 模块提取决策
  生成 Decision Node
  保存到 Memory Graph
  持久化到 Git

T3: 用户在 Claude 中问
  用户: "我们最近关于数据库做了什么决策？"

T4: Claude 调用 MCP Server
  Claude 决定调用 search 工具
  请求: {"query": "数据库", ...}

T5: MCP Server 处理
  调用 Memory Graph 搜索
  返回匹配的决策

T6: Claude 展示给用户
  "你们之前决定使用 PostgreSQL，..."
```

### 场景2：智能分析新决策

```
时间线:

T1: 用户在 Claude 中上传内容
  用户: "帮我分析这段文字，提取决策信息"

T2: Claude 调用 MCP Server
  Claude: 这是新内容，需要调用 extract_decision
  请求: {"content": "我们决定用 Redis", "topics": [...]}

T3: MCP Server 处理
  调用 MemoryAgent.ExtractDecision()
  调用真实的 LLM API (豆包大模型)
  解析结果

T4: 返回给 Claude
  包含决策标题、内容、建议议题、负责人等

T5: Claude 展示结果
  用户看到结构化的决策提取结果
```

## 工具调用映射关系

| MCP 工具 | 对应 LLM 方法 | 说明 |
|---------|------------|------|
| `search` | (不调用 LLM) | 直接查询 Memory Graph |
| `topic` | (不调用 LLM) | 直接查询 Memory Graph |
| `decision` | (不调用 LLM) | 直接查询 Memory Graph |
| `extract_decision` | MemoryAgent.ExtractDecision() | 调用 LLM |
| `classify_topic` | MemoryAgent.ClassifyTopic() | 调用 LLM |
| `detect_crosstopic` | MemoryAgent.DetectCrossTopic() | 调用 LLM |
| `check_conflict` | MemoryAgent.ResolveConflict() | 调用 LLM |
| `timeline` | (不调用 LLM) | 直接查询 Memory Graph |

## 关键数据结构

### MCP Request
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "extract_decision",
    "arguments": {
      "content": "...",
      "topics": [...]
    }
  }
}
```

### MCP Response
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "提取到的决策内容..."
      }
    ]
  }
}
```

## 部署模式

### 模式1：分离部署（推荐）
```
┌─────────────────┐     ┌──────────────────────┐
│  飞书监控服务    │     │  MCP Server         │
│  (lark-adapter)  │────▶│  (stdio mode)       │
└─────────────────┘     └─────────┬────────────┘
                                │
                            ┌───▼────────┐
                            │ Claude    │
                            └────────────┘
```

### 模式2：一体化部署
```
┌─────────────────────────────────────┐
│    mem-service (主服务)             │
│  ┌─────────────────────────────┐   │
│  │ - 飞书监控 + LLM 提取      │   │
│  │ - Memory Graph + Git        │   │
│  └─────────────────────────────┘   │
│  ┌─────────────────────────────┐   │
│  │ - MCP Server (可选 HTTP)    │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
```

## 测试要点

### 测试 MCP 协议时需要考虑
1. 请求/响应格式正确性 (JSON-RPC 2.0)
2. 工具调用流程
3. 真实 LLM 集成
4. 错误处理
5. 降级策略 (LLM 不可用时)

### 测试飞书集成时需要考虑
1. 飞书 API 调用
2. 决策提取准确性
3. 存储一致性
4. 信号处理流程
