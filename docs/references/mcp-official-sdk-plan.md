# 基于 modelcontextprotocol/go-sdk 的 MCP 实现计划

> 使用官方 MCP Go SDK 重构和完善现有实现，确保协议兼容性、减少维护成本。

---

## 一、官方 SDK 简介

### 1.1 仓库与文档

- **GitHub**: https://github.com/modelcontextprotocol/go-sdk
- **Go Module**: `modelcontextprotocol.io/sdk`
- **Current Version**: v0.6.0+ (检查最新版本)

### 1.2 SDK 提供的核心能力

**服务器端 (Server)**
- ✅ 完整的 JSON-RPC 2.0 协议实现
- ✅ Stdio 传输层
- ✅ 工具注册与调用
- ✅ 资源注册与读取
- ✅ Prompt 模板注册与渲染
- ✅ 初始化握手处理
- ✅ 错误处理与标准错误码
- ✅ 日志记录
- ✅ 进度通知

**客户端端 (Client)**
- ✅ 完整的客户端 SDK
- ✅ 多传输模式支持
- ✅ 工具调用 API
- ✅ 资源读取 API
- ✅ Prompt 渲染 API

---

## 二、重构架构

### 2.1 新架构概览

```
┌─────────────────────────────────────────────────────────────┐
│           Claude Desktop / 自定义客户端                   │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│      modelcontextprotocol/go-sdk (Transport Layer)       │
│  ┌──────────────────┐  ┌──────────────────┐                │
│  │ StdioServer      │  │ StdioClient      │                │
│  └──────────────────┘  └──────────────────┘                │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│         Our MCP Server (Business Logic)                   │
│  ┌─────────────────────────────────────────────────────┐  │
│  │ Tool Handlers (8+ tools)                           │  │
│  │ Resource Handlers (static + dynamic)               │  │
│  │ Prompt Templates (parameterized)                   │  │
│  └──────────────────────┬──────────────────────────────┘  │
│                         │                                  │
│  ┌──────────────────────▼──────────────────────────────┐  │
│  │ Memory Graph │ Git Storage │ Bitable │ LLM Agent    │  │
│  └─────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 代码结构重组

```
internal/mcp/
├── server/
│   ├── server.go          # 使用 SDK 的 MCP 服务器
│   ├── tools.go           # 工具注册与实现
│   ├── resources.go       # 资源注册与实现
│   └── prompts.go         # Prompt 注册与实现
├── types/
│   └── types.go           # 补充类型定义（如需要）
└── mcp.go                 # 导出的 API

pkg/mcpclient/
└── client.go              # 基于 SDK 的客户端封装
```

---

## 三、实施步骤

### 阶段一：SDK 集成与基础重构（1 周）

#### 1.1 更新 go.mod

```go
module feishu-mem

go 1.24

require (
    github.com/joho/godotenv v1.5.1
    github.com/sergi/go-diff v1.3.1
    github.com/stretchr/testify v1.11.1
    modelcontextprotocol.io/sdk v0.6.0  // 新增
)
```

执行：
```bash
go get modelcontextprotocol.io/sdk
```

#### 1.2 创建新的基于 SDK 的服务器

**文件**: `internal/mcp/server/server.go`

```go
package server

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "os"

    mcp "modelcontextprotocol.io/sdk"
    
    "feishu-mem/internal/core"
    "feishu-mem/internal/decision"
    "feishu-mem/internal/llm"
    "feishu-mem/internal/storage/bitable"
    "feishu-mem/internal/storage/git"
)

// MemoryMCPServer 我们的 MCP 服务器（封装 SDK）
type MemoryMCPServer struct {
    sdkServer   *mcp.Server
    memoryGraph *core.MemoryGraph
    gitStorage  *git.GitStorage
    bitableStore bitable.Store
    llmAgent    *llm.MemoryAgent
}

// NewMemoryMCPServer 创建 MCP 服务器
func NewMemoryMCPServer(
    memoryGraph *core.MemoryGraph,
    gitStorage *git.GitStorage,
    bitableStore bitable.Store,
) (*MemoryMCPServer, error) {
    
    // 1. 创建 SDK 服务器
    srv := &MemoryMCPServer{
        memoryGraph:  memoryGraph,
        gitStorage:   gitStorage,
        bitableStore: bitableStore,
        llmAgent:     llm.NewMemoryAgent(),
    }
    
    // 2. 初始化 SDK Server
    var err error
    srv.sdkServer, err = mcp.NewServer(
        mcp.ServerInfo{
            Name:    "Feishu Memory Agent",
            Version: "2.0.0",
        },
        mcp.ServerCapabilities{
            Tools: &mcp.ToolsCapability{
                ListChanged: true,
            },
            Resources: &mcp.ResourcesCapability{
                Subscribe:   true,
                ListChanged: true,
            },
            Prompts: &mcp.PromptsCapability{
                ListChanged: true,
            },
        },
        nil, // 日志 handler，可自定义
    )
    if err != nil {
        return nil, err
    }
    
    // 3. 初始化握手处理
    srv.sdkServer.SetRequestHandler(mcp.InitializeRequest, func(ctx context.Context, req mcp.InitializeRequest) (*mcp.InitializeResult, error) {
        slog.Info("Client initialized", 
            "client_name", req.ClientInfo.Name,
            "client_version", req.ClientInfo.Version,
        )
        return &mcp.InitializeResult{
            ProtocolVersion: mcp.LatestProtocolVersion,
            Capabilities:    srv.sdkServer.Capabilities(),
            ServerInfo:      srv.sdkServer.Info(),
        }, nil
    })
    
    // 4. 注册工具
    srv.registerTools()
    
    // 5. 注册资源
    srv.registerResources()
    
    // 6. 注册 Prompts
    srv.registerPrompts()
    
    return srv, nil
}

// Start 启动服务器（stdio 模式）
func (s *MemoryMCPServer) Start(ctx context.Context) error {
    slog.Info("Starting MCP server")
    
    // SDK 的 Stdio 传输已内置，直接使用
    return s.sdkServer.Serve(ctx, os.Stdin, os.Stdout)
}

// Stop 停止服务器
func (s *MemoryMCPServer) Stop() {
    // SDK 会通过 ctx 取消来处理
}

// ========== 工具注册与实现 ==========

func (s *MemoryMCPServer) registerTools() {
    // 1. Search Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "search",
        Description: "搜索记忆系统中的决策记录，支持关键词、议题过滤",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "搜索关键词"},
                "topic": {"type": "string", "description": "议题过滤"},
                "limit": {"type": "number", "description": "结果限制", "default": 20}
            }
        }`),
    }, s.handleSearch)
    
    // 2. Topic Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "topic",
        Description: "查询指定议题的所有决策记录",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "topic": {"type": "string", "description": "议题名称"}
            },
            "required": ["topic"]
        }`),
    }, s.handleTopic)
    
    // 3. Decision Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "decision",
        Description: "获取单个决策的详细信息",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "sdr_id": {"type": "string", "description": "决策ID"}
            },
            "required": ["sdr_id"]
        }`),
    }, s.handleDecision)
    
    // 4. Extract Decision Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "extract_decision",
        Description: "从文本内容中智能提取决策信息",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "content": {"type": "string", "description": "待分析的文本"},
                "topics": {"type": "array", "items": {"type": "string"}, "description": "候选议题"}
            },
            "required": ["content"]
        }`),
    }, s.handleExtractDecision)
    
    // 5. Classify Topic Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "classify_topic",
        Description: "将决策智能分类到正确的议题",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "decision": {"type": "string", "description": "决策内容"},
                "topics": {"type": "array", "items": {"type": "string"}, "description": "候选议题"}
            },
            "required": ["decision", "topics"]
        }`),
    }, s.handleClassifyTopic)
    
    // 6. Detect Cross-Topic Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "detect_crosstopic",
        Description: "检测决策是否会影响多个议题",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "title": {"type": "string", "description": "决策标题"},
                "decision": {"type": "string", "description": "决策内容"},
                "candidate_topics": {"type": "array", "items": {"type": "string"}, "description": "候选议题"}
            },
            "required": ["title", "decision", "candidate_topics"]
        }`),
    }, s.handleDetectCrosstopic)
    
    // 7. Check Conflict Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "check_conflict",
        Description: "评估两个决策之间是否存在冲突",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "decision_a": {"type": "string", "description": "新决策"},
                "decision_b": {"type": "string", "description": "已有决策"}
            },
            "required": ["decision_a", "decision_b"]
        }`),
    }, s.handleCheckConflict)
    
    // 8. Timeline Tool
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "timeline",
        Description: "获取决策历史时间线",
        InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
    }, s.handleTimeline)
    
    // 9. Create Decision Tool (新增写操作)
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "create_decision",
        Description: "创建新决策",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "title": {"type": "string"},
                "decision": {"type": "string"},
                "rationale": {"type": "string"},
                "topic": {"type": "string"},
                "phase": {"type": "string"},
                "impact_level": {"type": "string", "enum": ["advisory", "minor", "major", "critical"]}
            },
            "required": ["title", "decision", "topic"]
        }`),
    }, s.handleCreateDecision)
    
    // 10. Update Decision Tool (新增写操作)
    s.sdkServer.RegisterTool(mcp.Tool{
        Name:        "update_decision",
        Description: "更新已有决策",
        InputSchema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "sdr_id": {"type": "string"},
                "title": {"type": "string"},
                "decision": {"type": "string"},
                "rationale": {"type": "string"},
                "status": {"type": "string", "enum": ["pending", "in_discussion", "decided", "executing", "completed", "shelved", "rejected", "superseded", "deprecated"]}
            },
            "required": ["sdr_id"]
        }`),
    }, s.handleUpdateDecision)
}

// ========== 工具处理函数 ==========

func (s *MemoryMCPServer) handleSearch(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    query, _ := args["query"].(string)
    topic, _ := args["topic"].(string)
    limit := 20
    if l, ok := args["limit"].(float64); ok {
        limit = int(l)
    }
    
    var results []*decision.DecisionNode
    if s.memoryGraph != nil {
        results = s.memoryGraph.SearchByKeywords(query, topic)
        if len(results) > limit {
            results = results[:limit]
        }
    }
    
    // 格式化为文本
    text := "## 搜索结果\n\n"
    if len(results) == 0 {
        text += "未找到匹配的决策记录"
    } else {
        for _, r := range results {
            text += fmt.Sprintf("- [%s] %s (%s) - %s\n",
                r.Status, r.Title, r.SDRID, r.Topic)
        }
    }
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            {
                Type: "text",
                Text: text,
            },
        },
    }, nil
}

func (s *MemoryMCPServer) handleTopic(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    topic, _ := args["topic"].(string)
    
    var decisions []*decision.DecisionNode
    if s.memoryGraph != nil {
        decisions = s.memoryGraph.QueryByTopic("", topic)
    }
    
    text := fmt.Sprintf("## 议题: %s\n\n", topic)
    text += fmt.Sprintf("共 %d 个决策\n\n", len(decisions))
    for _, d := range decisions {
        text += fmt.Sprintf("- [%s] %s (%s)\n", d.Status, d.Title, d.SDRID)
    }
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleDecision(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    sdrID, _ := args["sdr_id"].(string)
    
    var d *decision.DecisionNode
    var found bool
    if s.memoryGraph != nil {
        d, found = s.memoryGraph.GetDecision(sdrID)
    }
    
    var text string
    if !found || d == nil {
        text = "未找到指定的决策"
    } else {
        text = fmt.Sprintf("## %s\n\n", d.Title)
        text += fmt.Sprintf("- **SDR ID**: %s\n", d.SDRID)
        text += fmt.Sprintf("- **议题**: %s\n", d.Topic)
        text += fmt.Sprintf("- **决策**: %s\n", d.Decision)
        text += fmt.Sprintf("- **依据**: %s\n", d.Rationale)
        text += fmt.Sprintf("- **状态**: %s\n", d.Status)
        text += fmt.Sprintf("- **影响等级**: %s\n", d.ImpactLevel)
    }
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleExtractDecision(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    content, _ := args["content"].(string)
    topics, _ := args["topics"].([]interface{})
    
    // 转换 topics 为 []string
    var topicList []string
    for _, t := range topics {
        if s, ok := t.(string); ok {
            topicList = append(topicList, s)
        }
    }
    
    result, err := s.llmAgent.ExtractDecision(content, topicList)
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("提取失败: %v", err)}},
            IsError: true,
        }, nil
    }
    
    var text string
    if !result.HasDecision {
        text = "未检测到决策信息"
    } else {
        text = "## 决策提取结果\n\n"
        text += fmt.Sprintf("- **置信度**: %.2f\n", result.Confidence)
        if result.Decision != nil {
            text += fmt.Sprintf("- **标题**: %s\n", result.Decision.Title)
            text += fmt.Sprintf("- **决策**: %s\n", result.Decision.Decision)
            text += fmt.Sprintf("- **建议议题**: %s\n", result.Decision.SuggestedTopic)
        }
    }
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleClassifyTopic(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    decisionStr, _ := args["decision"].(string)
    topics, _ := args["topics"].([]interface{})
    
    var topicList []string
    for _, t := range topics {
        if s, ok := t.(string); ok {
            topicList = append(topicList, s)
        }
    }
    
    result, err := s.llmAgent.ClassifyTopic(decisionStr, topicList)
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("分类失败: %v", err)}},
            IsError: true,
        }, nil
    }
    
    text := "## 议题分类结果\n\n"
    text += fmt.Sprintf("- **建议议题**: %s\n", result.Topic)
    text += fmt.Sprintf("- **置信度**: %.2f\n", result.Confidence)
    text += fmt.Sprintf("- **说明**: %s\n", result.Reasoning)
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleDetectCrosstopic(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    title, _ := args["title"].(string)
    decisionStr, _ := args["decision"].(string)
    candidateTopics, _ := args["candidate_topics"].([]interface{})
    
    var topicList []string
    for _, t := range candidateTopics {
        if s, ok := t.(string); ok {
            topicList = append(topicList, s)
        }
    }
    
    result, err := s.llmAgent.DetectCrossTopic(map[string]interface{}{
        "title":            title,
        "decision":         decisionStr,
        "candidate_topics": topicList,
    })
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("检测失败: %v", err)}},
            IsError: true,
        }, nil
    }
    
    text := "## 跨议题检测结果\n\n"
    if result.IsCrossTopic {
        text += "⚠️ **检测到跨议题影响**\n"
        text += fmt.Sprintf("- **受影响议题**: %v\n", result.CrossTopicRefs)
    } else {
        text += "✅ **无跨议题影响**\n"
    }
    text += fmt.Sprintf("\n- **置信度**: %.2f\n", result.Confidence)
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleCheckConflict(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    decisionA, _ := args["decision_a"].(string)
    decisionB, _ := args["decision_b"].(string)
    
    result, err := s.llmAgent.ResolveConflict(decisionA, decisionB)
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{{Type: "text", Text: fmt.Sprintf("冲突评估失败: %v", err)}},
            IsError: true,
        }, nil
    }
    
    text := "## 冲突评估结果\n\n"
    text += fmt.Sprintf("- **冲突分数**: %.2f\n", result.ContradictionScore)
    text += fmt.Sprintf("- **冲突类型**: %s\n", result.ContradictionType)
    text += fmt.Sprintf("- **描述**: %s\n", result.Description)
    text += fmt.Sprintf("- **建议**: %s\n", result.Action)
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleTimeline(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    var items []struct {
        Timestamp time.Time
        Event     string
        SDRID     string
    }
    
    if s.memoryGraph != nil {
        for _, d := range s.memoryGraph.GetAllDecisions() {
            ts := d.CreatedAt
            if d.DecidedAt != nil {
                ts = *d.DecidedAt
            }
            items = append(items, struct {
                Timestamp time.Time
                Event     string
                SDRID     string
            }{ts, d.Title, d.SDRID})
        }
    }
    
    text := "## 决策时间线\n\n"
    for _, item := range items {
        text += fmt.Sprintf("- %s: %s (%s)\n",
            item.Timestamp.Format("2006-01-02 15:04"),
            item.Event,
            item.SDRID)
    }
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleCreateDecision(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    // 实现创建决策逻辑
    title, _ := args["title"].(string)
    decisionStr, _ := args["decision"].(string)
    rationale, _ := args["rationale"].(string)
    topic, _ := args["topic"].(string)
    phase, _ := args["phase"].(string)
    impactLevel, _ := args["impact_level"].(string)
    
    d := decision.NewDecisionNode(
        decision.GenerateSDRID(),
        title,
        "", // project (使用默认)
        topic,
    )
    d.Decision = decisionStr
    d.Rationale = rationale
    d.Phase = phase
    d.ImpactLevel = decision.ImpactLevel(impactLevel)
    d.Status = decision.StatusPending
    
    if s.memoryGraph != nil {
        s.memoryGraph.UpsertDecision(d, "")
    }
    
    // 同步到 Git
    if s.gitStorage != nil {
        if err := s.gitStorage.SaveDecision("", topic, d); err != nil {
            slog.Error("Failed to save to git", "error", err)
        }
    }
    
    text := fmt.Sprintf("✅ **决策创建成功**\n\n- **SDR ID**: %s\n- **标题**: %s\n- **议题**: %s", 
        d.SDRID, d.Title, d.Topic)
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

func (s *MemoryMCPServer) handleUpdateDecision(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error) {
    sdrID, _ := args["sdr_id"].(string)
    
    d, found := s.memoryGraph.GetDecision(sdrID)
    if !found {
        return &mcp.CallToolResult{
            Content: []mcp.Content{{Type: "text", Text: "未找到指定的决策"}},
            IsError: true,
        }, nil
    }
    
    // 更新字段
    if title, ok := args["title"].(string); ok && title != "" {
        d.Title = title
    }
    if decisionStr, ok := args["decision"].(string); ok && decisionStr != "" {
        d.Decision = decisionStr
    }
    if rationale, ok := args["rationale"].(string); ok && rationale != "" {
        d.Rationale = rationale
    }
    if status, ok := args["status"].(string); ok && status != "" {
        d.Status = decision.DecisionStatus(status)
    }
    
    // 保存
    s.memoryGraph.UpsertDecision(d, "")
    if s.gitStorage != nil {
        s.gitStorage.SaveDecision("", d.Topic, d)
    }
    
    text := fmt.Sprintf("✅ **决策更新成功**\n\n- **SDR ID**: %s\n- **标题**: %s\n- **状态**: %s",
        d.SDRID, d.Title, d.Status)
    
    return &mcp.CallToolResult{
        Content: []mcp.Content{{Type: "text", Text: text}},
    }, nil
}

// ========== 资源注册与实现 ==========

func (s *MemoryMCPServer) registerResources() {
    // 1. System Design Document
    s.sdkServer.RegisterResource(mcp.Resource{
        URI:      "docs://design",
        Name:     "System Design Document",
        MimeType: "text/markdown",
    }, s.handleReadDesignDoc)
    
    // 2. Prompt Templates
    s.sdkServer.RegisterResource(mcp.Resource{
        URI:      "docs://prompts",
        Name:     "LLM Prompt Templates",
        MimeType: "text/markdown",
    }, s.handleReadPromptsDoc)
    
    // 3. Git 存储中的决策文件（动态资源）
    if s.gitStorage != nil {
        // 可以注册动态资源模板
    }
}

func (s *MemoryMCPServer) handleReadDesignDoc(ctx context.Context, uri mcp.URI) (*mcp.ReadResourceResult, error) {
    content := `# Feishu Memory Agent 系统设计

## 核心概念
- **Decision Node**: 决策节点，记录项目决策
- **Topic**: 议题，用于组织决策
- **Signal**: 信号，触发决策提取的事件

## 主要模块
1. **internal/core**: 核心数据结构
2. **internal/decision**: 决策树管理
3. **internal/llm**: LLM 智能处理模块
4. **internal/storage/git**: Git 持久化存储
5. **internal/mcp**: MCP 服务器
`
    return &mcp.ReadResourceResult{
        Contents: []mcp.ResourceContents{
            {
                URI:      uri,
                MimeType: "text/markdown",
                Text:     content,
            },
        },
    }, nil
}

func (s *MemoryMCPServer) handleReadPromptsDoc(ctx context.Context, uri mcp.URI) (*mcp.ReadResourceResult, error) {
    content := `# LLM 提示词模板

## 1. 决策提取 (extraction)
用于从文本中识别和提取决策信息

## 2. 议题分类 (classification)
将决策归类到正确的议题

## 3. 跨议题检测 (crosstopic)
检测决策是否影响多个议题

## 4. 冲突评估 (conflict)
评估两个决策之间的冲突类型和程度
`
    return &mcp.ReadResourceResult{
        Contents: []mcp.ResourceContents{
            {
                URI:      uri,
                MimeType: "text/markdown",
                Text:     content,
            },
        },
    }, nil
}

// ========== Prompt 注册与实现 ==========

func (s *MemoryMCPServer) registerPrompts() {
    s.sdkServer.RegisterPrompt(mcp.Prompt{
        Name:        "extract_decision",
        Description: "从文本内容中智能提取决策信息",
        Arguments: []mcp.PromptArgument{
            {Name: "content", Description: "待分析的文本", Required: true},
            {Name: "topics", Description: "候选议题列表", Required: false},
        },
    }, s.handleGetExtractDecisionPrompt)
    
    s.sdkServer.RegisterPrompt(mcp.Prompt{
        Name:        "classify_topic",
        Description: "将决策智能分类到正确的议题",
        Arguments: []mcp.PromptArgument{
            {Name: "decision", Description: "决策内容", Required: true},
            {Name: "topics", Description: "候选议题列表", Required: true},
        },
    }, s.handleGetClassifyTopicPrompt)
    
    s.sdkServer.RegisterPrompt(mcp.Prompt{
        Name:        "detect_crosstopic",
        Description: "检测决策是否会影响多个议题",
        Arguments: []mcp.PromptArgument{
            {Name: "title", Description: "决策标题", Required: true},
            {Name: "decision", Description: "决策内容", Required: true},
            {Name: "candidate_topics", Description: "候选议题列表", Required: true},
        },
    }, s.handleGetDetectCrosstopicPrompt)
    
    s.sdkServer.RegisterPrompt(mcp.Prompt{
        Name:        "check_conflict",
        Description: "评估两个决策之间是否存在冲突",
        Arguments: []mcp.PromptArgument{
            {Name: "decision_a", Description: "新决策", Required: true},
            {Name: "decision_b", Description: "已有决策", Required: true},
        },
    }, s.handleGetCheckConflictPrompt)
}

func (s *MemoryMCPServer) handleGetExtractDecisionPrompt(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
    content, _ := args["content"]
    topics, _ := args["topics"]
    
    promptText := fmt.Sprintf(`从以下文本中提取决策信息，包括决策标题、内容、建议议题、负责人等。

## 文本内容
%s

## 候选议题
%s

请以 JSON 格式输出结果。`, content, topics)
    
    return &mcp.GetPromptResult{
        Description: "从文本中提取决策信息",
        Messages: []mcp.PromptMessage{
            {
                Role: "user",
                Content: mcp.TextContent{
                    Type: "text",
                    Text: promptText,
                },
            },
        },
    }, nil
}

func (s *MemoryMCPServer) handleGetClassifyTopicPrompt(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
    decisionStr, _ := args["decision"]
    topics, _ := args["topics"]
    
    promptText := fmt.Sprintf(`将给定的决策内容归类到最匹配的议题分类中。

## 决策内容
%s

## 候选议题
%s

请选择最合适的议题，并给出置信度和理由。`, decisionStr, topics)
    
    return &mcp.GetPromptResult{
        Description: "将决策归类到正确的议题",
        Messages: []mcp.PromptMessage{
            {
                Role: "user",
                Content: mcp.TextContent{
                    Type: "text",
                    Text: promptText,
                },
            },
        },
    }, nil
}

func (s *MemoryMCPServer) handleGetDetectCrosstopicPrompt(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
    title, _ := args["title"]
    decisionStr, _ := args["decision"]
    candidateTopics, _ := args["candidate_topics"]
    
    promptText := fmt.Sprintf(`判断给定的决策是否会对其他议题产生影响，并列出受影响的议题。

## 决策标题
%s

## 决策内容
%s

## 候选议题
%s

请输出是否跨议题、受影响的议题列表和理由。`, title, decisionStr, candidateTopics)
    
    return &mcp.GetPromptResult{
        Description: "检测跨议题影响",
        Messages: []mcp.PromptMessage{
            {
                Role: "user",
                Content: mcp.TextContent{
                    Type: "text",
                    Text: promptText,
                },
            },
        },
    }, nil
}

func (s *MemoryMCPServer) handleGetCheckConflictPrompt(ctx context.Context, args map[string]string) (*mcp.GetPromptResult, error) {
    decisionA, _ := args["decision_a"]
    decisionB, _ := args["decision_b"]
    
    promptText := fmt.Sprintf(`评估两个决策之间是否存在语义矛盾，给出矛盾分数和类型。

## 决策 A (新决策)
%s

## 决策 B (已有决策)
%s

请输出冲突分数 (0-1)、冲突类型、描述和建议操作。`, decisionA, decisionB)
    
    return &mcp.GetPromptResult{
        Description: "评估决策冲突",
        Messages: []mcp.PromptMessage{
            {
                Role: "user",
                Content: mcp.TextContent{
                    Type: "text",
                    Text: promptText,
                },
            },
        },
    }, nil
}
```

#### 1.3 更新 cmd/mcp-server/main.go

```go
package main

import (
    "context"
    "log"
    "log/slog"
    "os"
    "os/signal"
    "syscall"

    "feishu-mem/internal/config"
    "feishu-mem/internal/core"
    "feishu-mem/internal/mcp/server"
    "feishu-mem/internal/storage/bitable"
    "feishu-mem/internal/storage/git"
)

func main() {
    // 配置日志
    slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })))
    
    // 加载配置
    larkadapter.LoadEnv()
    settings := config.DefaultSettings()
    if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
        if s, err := config.LoadSettings(cfgPath); err == nil {
            settings = s
        }
    } else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
        settings = s
    }
    
    // 初始化存储
    var gitStorage *git.GitStorage
    if settings.Git.WorkDir != "" {
        var err error
        gitStorage, err = git.NewGitStorage(git.Config{
            WorkDir:  settings.Git.WorkDir,
            Remote:   settings.Git.Remote,
            AutoPush: false,
            Branch:   settings.Git.Branch,
        })
        if err != nil {
            slog.Warn("Git storage init failed", "error", err)
        }
    }
    
    memoryGraph := core.NewMemoryGraph()
    if gitStorage != nil && settings.Memory.PreloadOnStart {
        if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
            slog.Warn("Failed to load from Git", "error", err)
        }
    }
    
    var bitableStore bitable.Store
    if settings.Bitable.BaseToken != "" {
        bitableStore = bitable.NewBitableStore(bitable.Config{
            BaseToken: settings.Bitable.BaseToken,
            Tables: bitable.TablesConfig{
                Decision: settings.Bitable.Tables.Decision,
                Topic:    settings.Bitable.Tables.Topic,
                Phase:    settings.Bitable.Tables.Phase,
                Relation: settings.Bitable.Tables.Relation,
            },
        }, nil)
    }
    
    // 创建 MCP 服务器（使用 SDK 版本）
    srv, err := server.NewMemoryMCPServer(memoryGraph, gitStorage, bitableStore)
    if err != nil {
        log.Fatalf("Failed to create MCP server: %v", err)
    }
    
    slog.Info("Feishu Memory MCP Server starting",
        "project", settings.Project.Name,
        "decisions_loaded", memoryGraph.Count())
    
    // 设置 context 与信号处理
    ctx, cancel := context.WithCancel(context.Background())
    
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    go func() {
        sig := <-sigChan
        slog.Info("Received signal, shutting down", "signal", sig)
        cancel()
    }()
    
    // 启动服务器
    if err := srv.Start(ctx); err != nil && err != context.Canceled {
        log.Fatalf("MCP server error: %v", err)
    }
    
    slog.Info("MCP server stopped")
}
```

---

### 阶段二：客户端 SDK（1 周）

基于官方 SDK 创建客户端封装：

**文件**: `pkg/mcpclient/client.go`

```go
package mcpclient

import (
    "context"
    "fmt"
    "os/exec"

    mcp "modelcontextprotocol.io/sdk"
)

// Client MCP 客户端（封装 SDK）
type Client struct {
    client *mcp.Client
}

// NewStdioClient 创建 stdio 模式的客户端
func NewStdioClient(cmdPath string, args ...string) (*Client, error) {
    cmd := exec.Command(cmdPath, args...)
    
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, err
    }
    
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }
    
    stderr, err := cmd.StderrPipe()
    if err != nil {
        return nil, err
    }
    
    // 启动服务器进程
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    // 创建 SDK 客户端
    client, err := mcp.NewClient(
        mcp.ClientInfo{
            Name:    "feishu-mem-client",
            Version: "1.0.0",
        },
        nil,
    )
    if err != nil {
        cmd.Process.Kill()
        return nil, err
    }
    
    // 启动协议处理
    go func() {
        if err := client.Connect(context.Background(), stdout, stdin); err != nil {
            fmt.Printf("Client connect error: %v\n", err)
        }
    }()
    
    return &Client{client: client}, nil
}

// Initialize 初始化客户端
func (c *Client) Initialize(ctx context.Context) error {
    _, err := c.client.Initialize(ctx, mcp.InitializeRequest{
        ProtocolVersion: mcp.LatestProtocolVersion,
        Capabilities:    mcp.ClientCapabilities{},
        ClientInfo:      c.client.Info(),
    })
    return err
}

// ListTools 列出所有工具
func (c *Client) ListTools(ctx context.Context) ([]mcp.Tool, error) {
    result, err := c.client.ListTools(ctx)
    if err != nil {
        return nil, err
    }
    return result.Tools, nil
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
    return c.client.CallTool(ctx, mcp.CallToolRequest{
        Name:      name,
        Arguments: args,
    })
}

// Search 便捷方法：搜索
func (c *Client) Search(ctx context.Context, query string, topic string, limit int) (string, error) {
    result, err := c.CallTool(ctx, "search", map[string]interface{}{
        "query": query,
        "topic": topic,
        "limit": limit,
    })
    if err != nil {
        return "", err
    }
    if result.IsError {
        return "", fmt.Errorf("tool error: %s", result.Content[0].Text)
    }
    return result.Content[0].Text, nil
}

// GetDecision 便捷方法：获取决策
func (c *Client) GetDecision(ctx context.Context, sdrID string) (string, error) {
    result, err := c.CallTool(ctx, "decision", map[string]interface{}{
        "sdr_id": sdrID,
    })
    if err != nil {
        return "", err
    }
    return result.Content[0].Text, nil
}

// CreateDecision 便捷方法：创建决策
func (c *Client) CreateDecision(ctx context.Context, title, decisionStr, rationale, topic, phase, impactLevel string) (string, error) {
    result, err := c.CallTool(ctx, "create_decision", map[string]interface{}{
        "title":         title,
        "decision":      decisionStr,
        "rationale":     rationale,
        "topic":         topic,
        "phase":         phase,
        "impact_level":  impactLevel,
    })
    if err != nil {
        return "", err
    }
    return result.Content[0].Text, nil
}

// ListResources 列出资源
func (c *Client) ListResources(ctx context.Context) ([]mcp.Resource, error) {
    result, err := c.client.ListResources(ctx)
    if err != nil {
        return nil, err
    }
    return result.Resources, nil
}

// ReadResource 读取资源
func (c *Client) ReadResource(ctx context.Context, uri string) (string, error) {
    result, err := c.client.ReadResource(ctx, mcp.ReadResourceRequest{URI: mcp.URI(uri)})
    if err != nil {
        return "", err
    }
    return result.Contents[0].Text, nil
}

// ListPrompts 列出 prompts
func (c *Client) ListPrompts(ctx context.Context) ([]mcp.Prompt, error) {
    result, err := c.client.ListPrompts(ctx)
    if err != nil {
        return nil, err
    }
    return result.Prompts, nil
}

// GetPrompt 获取 prompt
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*mcp.GetPromptResult, error) {
    return c.client.GetPrompt(ctx, mcp.GetPromptRequest{
        Name:      name,
        Arguments: args,
    })
}

// Close 关闭客户端
func (c *Client) Close() error {
    return c.client.Close()
}
```

**客户端使用示例**:

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "feishu-mem/pkg/mcpclient"
)

func main() {
    ctx := context.Background()
    
    // 1. 创建客户端（启动服务器子进程）
    client, err := mcpclient.NewStdioClient(
        "/path/to/mcp-server",
        "--config=config/openclaw.yaml",
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // 2. 初始化
    if err := client.Initialize(ctx); err != nil {
        log.Fatal(err)
    }
    
    // 3. 列出工具
    tools, _ := client.ListTools(ctx)
    fmt.Printf("Available tools: %d\n", len(tools))
    for _, t := range tools {
        fmt.Printf("  - %s: %s\n", t.Name, t.Description)
    }
    
    // 4. 搜索
    result, _ := client.Search(ctx, "数据库", "", 10)
    fmt.Println(result)
    
    // 5. 创建决策
    createResult, _ := client.CreateDecision(
        ctx,
        "使用 PostgreSQL",
        "我们决定使用 PostgreSQL 作为主数据库",
        "需要支持 JSON 类型和复杂查询",
        "数据库架构",
        "lab1",
        "major",
    )
    fmt.Println(createResult)
}
```

---

### 阶段三：配置与部署（1 周）

#### 3.1 mcporter 配置

```yaml
# config/mcporter.yaml
mcpServers:
  - name: feishu-mem
    command:
      path: /root/openclaw-workspace/feishu-agent-mem/bin/mcp-server
      args: []
    env:
      CONFIG_PATH: /root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml
      ARK_API_KEY: ${ARK_API_KEY}
```

#### 3.2 Claude Desktop 配置

```json
{
  "mcpServers": {
    "feishu-mem": {
      "command": "/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server",
      "env": {
        "CONFIG_PATH": "/root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml",
        "ARK_API_KEY": "..."
      }
    }
  }
}
```

#### 3.3 Docker 部署脚本

```bash
#!/bin/bash
# scripts/deploy-mcp-sdk.sh

DOCKER_CONTAINER="openclaw-zh"
PROJECT_DIR="/root/openclaw-workspace/feishu-agent-mem"

echo "=== 部署基于 SDK 的 MCP 服务器 ==="

# 1. 下载依赖
echo "[1/4] 下载依赖..."
docker exec $DOCKER_CONTAINER bash -c "cd $PROJECT_DIR && export PATH=/usr/local/go/bin:\$PATH && go get modelcontextprotocol.io/sdk"

# 2. 编译
echo "[2/4] 编译二进制..."
docker exec $DOCKER_CONTAINER bash -c "cd $PROJECT_DIR && export PATH=/usr/local/go/bin:\$PATH && go build -o bin/mcp-server ./cmd/mcp-server/main.go"

# 3. 停止旧进程
echo "[3/4] 停止旧进程..."
docker exec $DOCKER_CONTAINER bash -c "pkill -f mcp-server 2>/dev/null || true"

# 4. 验证
echo "[4/4] 验证..."
docker exec $DOCKER_CONTAINER bash -c "cd $PROJECT_DIR && ls -la bin/mcp-server"

echo ""
echo "=== 部署完成 ==="
echo "二进制位置: bin/mcp-server"
```

---

### 阶段四：测试与完善（1 周）

#### 4.1 测试文件

```go
package server_test

import (
    "context"
    "io"
    "os"
    "testing"
    
    "feishu-mem/internal/core"
    "feishu-mem/internal/mcp/server"
    "feishu-mem/pkg/mcpclient"
    
    "github.com/stretchr/testify/assert"
)

func TestServerClientE2E(t *testing.T) {
    // 创建内存图
    memoryGraph := core.NewMemoryGraph()
    
    // 创建服务器
    srv, err := server.NewMemoryMCPServer(memoryGraph, nil, nil)
    assert.NoError(t, err)
    
    // 创建管道
    serverIn, clientOut := io.Pipe()
    clientIn, serverOut := io.Pipe()
    
    // 启动服务器
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    go func() {
        srv.sdkServer.Serve(ctx, serverIn, serverOut)
    }()
    
    // 创建客户端（直接连接管道）
    client, err := mcpclient.NewClientFromPipes(clientIn, clientOut)
    assert.NoError(t, err)
    defer client.Close()
    
    // 初始化
    err = client.Initialize(ctx)
    assert.NoError(t, err)
    
    // 列出工具
    tools, err := client.ListTools(ctx)
    assert.NoError(t, err)
    assert.GreaterOrEqual(t, len(tools), 8)
    
    // 测试搜索
    result, err := client.Search(ctx, "test", "", 5)
    assert.NoError(t, err)
    assert.Contains(t, result, "搜索结果")
}
```

---

## 四、迁移对比

| 方面 | 旧实现（手写） | 新实现（SDK） |
|-----|--------------|--------------|
| **协议正确性** | 手动实现，可能有遗漏 | 官方维护，100% 兼容 |
| **代码量** | ~800 行（server.go） | ~600 行（更简洁） |
| **维护成本** | 需跟踪协议变更 | SDK 自动跟进 |
| **错误处理** | 自定义 | 标准化 |
| **日志** | 自定义 | 结构化 slog |
| **进度通知** | 需自己实现 | SDK 内置 |
| **Prompt 渲染** | 简陋 | 完整支持 |
| **测试覆盖** | 需完整 E2E | SDK 已有测试覆盖 |

---

## 五、实施路线图

### 周 1：SDK 集成与基础重构
- [ ] 更新 go.mod，添加 SDK 依赖
- [ ] 创建新的 `internal/mcp/server/` 目录结构
- [ ] 实现基于 SDK 的 `MemoryMCPServer`
- [ ] 迁移 8 个工具的实现
- [ ] 迁移资源和 prompt 实现
- [ ] 更新 `cmd/mcp-server/main.go`

### 周 2：写操作与客户端
- [ ] 实现新增的写操作工具（create/update）
- [ ] 创建 `pkg/mcpclient` 客户端包
- [ ] 编写客户端使用示例
- [ ] 创建集成测试

### 周 3：配置与部署
- [ ] mcporter 配置
- [ ] Claude Desktop 配置
- [ ] Docker 部署脚本
- [ ] 文档更新

### 周 4：测试与完善
- [ ] 端到端测试
- [ ] 错误场景测试
- [ ] 性能测试
- [ ] 清理旧代码

---

## 六、验证清单

- [ ] 所有 8 个原有工具正常工作
- [ ] 新增的写操作工具正常工作
- [ ] Claude Desktop 可以成功连接
- [ ] mcporter 可以成功发现
- [ ] 客户端 SDK 可以正常调用
- [ ] Git 同步正常
- [ ] LLM 集成正常
- [ ] 错误处理正确
