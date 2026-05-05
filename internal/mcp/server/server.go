package server

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"feishu-mem/internal/core"
	"feishu-mem/internal/decision"
	"feishu-mem/internal/llm"
	"feishu-mem/internal/storage/git"
)

// MemoryMCPServer 基于 SDK 的 MCP 服务器
type MemoryMCPServer struct {
	sdkServer   *mcp.Server
	memoryGraph *core.MemoryGraph
	gitStorage  *git.GitStorage
	llmAgent    *llm.MemoryAgent
}

// NewMemoryMCPServer 创建 MCP 服务器
func NewMemoryMCPServer(memoryGraph *core.MemoryGraph, gitStorage *git.GitStorage) (*MemoryMCPServer, error) {
	srv := &MemoryMCPServer{
		memoryGraph: memoryGraph,
		gitStorage:  gitStorage,
		llmAgent:    llm.NewMemoryAgent(),
	}

	srv.sdkServer = mcp.NewServer(&mcp.Implementation{
		Name:    "Feishu Memory Agent",
		Version: "2.0.0",
	}, nil)

	srv.registerTools()
	srv.registerResources()

	return srv, nil
}

// Run 启动服务器
func (s *MemoryMCPServer) Run(ctx context.Context) error {
	slog.Info("Starting MCP server")
	return s.sdkServer.Run(ctx, &mcp.StdioTransport{})
}

func (s *MemoryMCPServer) registerTools() {
	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "search",
		Description: "搜索记忆系统中的决策记录，支持关键词、议题过滤",
	}, s.handleSearch)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "topic",
		Description: "查询指定议题的所有决策记录",
	}, s.handleTopic)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "decision",
		Description: "获取单个决策的详细信息",
	}, s.handleDecision)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "timeline",
		Description: "获取决策历史时间线",
	}, s.handleTimeline)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "create_decision",
		Description: "创建新决策",
	}, s.handleCreateDecision)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "update_decision",
		Description: "更新已有决策",
	}, s.handleUpdateDecision)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "extract_decision",
		Description: "从文本内容中智能提取决策信息 (非LLM版本)",
	}, s.handleExtractDecision)
}

type searchArgs struct {
	Query string  `json:"query" jsonschema:"搜索关键词"`
	Topic string  `json:"topic" jsonschema:"议题过滤"`
	Limit float64 `json:"limit" jsonschema:"结果限制"`
}

type topicArgs struct {
	Topic string `json:"topic" jsonschema:"议题名称"`
}

type decisionArgs struct {
	SdrID string `json:"sdr_id" jsonschema:"决策ID"`
}

type extractDecisionArgs struct {
	Content string   `json:"content" jsonschema:"待分析的文本"`
	Topics  []string `json:"topics" jsonschema:"候选议题"`
}

type createDecisionArgs struct {
	Title       string `json:"title" jsonschema:"决策标题"`
	Decision    string `json:"decision" jsonschema:"决策内容"`
	Rationale   string `json:"rationale" jsonschema:"决策依据"`
	Topic       string `json:"topic" jsonschema:"议题"`
	Phase       string `json:"phase" jsonschema:"阶段"`
	ImpactLevel string `json:"impact_level" jsonschema:"影响等级"`
}

type updateDecisionArgs struct {
	SdrID     string `json:"sdr_id" jsonschema:"决策ID"`
	Title      string `json:"title" jsonschema:"决策标题"`
	Decision   string `json:"decision" jsonschema:"决策内容"`
	Rationale  string `json:"rationale" jsonschema:"决策依据"`
	Status     string `json:"status" jsonschema:"状态"`
}

type emptyArgs struct{}

type emptyResult struct{}

func (s *MemoryMCPServer) handleSearch(ctx context.Context, req *mcp.CallToolRequest, args searchArgs) (*mcp.CallToolResult, emptyResult, error) {
	limit := 20
	if args.Limit > 0 {
		limit = int(args.Limit)
	}

	var results []*decision.DecisionNode
	if s.memoryGraph != nil {
		results = s.memoryGraph.SearchByKeywords(args.Query, args.Topic)
		if len(results) > limit {
			results = results[:limit]
		}
	}

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
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleTopic(ctx context.Context, req *mcp.CallToolRequest, args topicArgs) (*mcp.CallToolResult, emptyResult, error) {
	var decisions []*decision.DecisionNode
	if s.memoryGraph != nil {
		decisions = s.memoryGraph.QueryByTopic("", args.Topic)
	}

	text := fmt.Sprintf("## 议题: %s\n\n", args.Topic)
	text += fmt.Sprintf("共 %d 个决策\n\n", len(decisions))
	for _, d := range decisions {
		text += fmt.Sprintf("- [%s] %s (%s)\n", d.Status, d.Title, d.SDRID)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleDecision(ctx context.Context, req *mcp.CallToolRequest, args decisionArgs) (*mcp.CallToolResult, emptyResult, error) {
	var d *decision.DecisionNode
	var found bool
	if s.memoryGraph != nil {
		d, found = s.memoryGraph.GetDecision(args.SdrID)
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
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleTimeline(ctx context.Context, req *mcp.CallToolRequest, args emptyArgs) (*mcp.CallToolResult, emptyResult, error) {
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
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleCreateDecision(ctx context.Context, req *mcp.CallToolRequest, args createDecisionArgs) (*mcp.CallToolResult, emptyResult, error) {
	impactLevel := args.ImpactLevel
	if impactLevel == "" {
		impactLevel = "minor"
	}

	d := decision.NewDecisionNode("", args.Title, "", args.Topic)
	d.Decision = args.Decision
	d.Rationale = args.Rationale
	d.Phase = args.Phase
	d.ImpactLevel = decision.ImpactLevel(impactLevel)
	d.Status = decision.StatusPending

	if s.memoryGraph != nil {
		s.memoryGraph.UpsertDecision(d, "")
	}

	if s.gitStorage != nil {
		if _, err := s.gitStorage.WriteDecision(d); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("❌ 创建失败: %v", err)}},
			}, emptyResult{}, nil
		}
	}

	text := fmt.Sprintf("✅ **决策创建成功**\n\n- **SDR ID**: %s\n- **标题**: %s\n- **议题**: %s",
		d.SDRID, d.Title, d.Topic)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleUpdateDecision(ctx context.Context, req *mcp.CallToolRequest, args updateDecisionArgs) (*mcp.CallToolResult, emptyResult, error) {
	d, found := s.memoryGraph.GetDecision(args.SdrID)
	if !found {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "未找到指定的决策"}},
		}, emptyResult{}, nil
	}

	if args.Title != "" {
		d.Title = args.Title
	}
	if args.Decision != "" {
		d.Decision = args.Decision
	}
	if args.Rationale != "" {
		d.Rationale = args.Rationale
	}
	if args.Status != "" {
		d.Status = decision.DecisionStatus(args.Status)
	}

	s.memoryGraph.UpsertDecision(d, "")

	text := fmt.Sprintf("✅ **决策更新成功**\n\n- **SDR ID**: %s\n- **标题**: %s\n- **状态**: %s",
		d.SDRID, d.Title, d.Status)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleExtractDecision(ctx context.Context, req *mcp.CallToolRequest, args extractDecisionArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := "## 决策提取结果\n\n"

	if len(args.Content) == 0 {
		text += "未提供文本内容"
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	// 简单的关键词提取（非LLM版本）
	text += fmt.Sprintf("### 输入内容\n%s\n\n", args.Content)

	// 检查是否包含决策相关的关键词
	if len(args.Content) > 20 {
		title := args.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}

		text += "### 提取结果\n\n"
		text += fmt.Sprintf("- **标题**: %s\n", title)
		text += fmt.Sprintf("- **长度**: %d 字符\n", len(args.Content))

		if len(args.Topics) > 0 {
			text += fmt.Sprintf("- **候选议题**: %v\n", args.Topics)
			text += fmt.Sprintf("- **建议议题**: %s\n", args.Topics[0])
		}

		text += "\n*注意: 完整的 LLM 功能需要更多配置*"
	} else {
		text += "文本内容太短，无法提取决策"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) registerResources() {
	s.sdkServer.AddResource(&mcp.Resource{
		URI:      "docs://design",
		Name:     "System Design Document",
		MIMEType: "text/markdown",
	}, s.handleReadDesignDoc)

	s.sdkServer.AddResource(&mcp.Resource{
		URI:      "docs://prompts",
		Name:     "LLM Prompt Templates",
		MIMEType: "text/markdown",
	}, s.handleReadPromptsDoc)
}

func (s *MemoryMCPServer) handleReadDesignDoc(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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
5. **internal/mcp**: MCP 服务器 (基于官方 SDK)
`
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: "docs://design", MIMEType: "text/markdown", Text: content}},
	}, nil
}

func (s *MemoryMCPServer) handleReadPromptsDoc(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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
		Contents: []*mcp.ResourceContents{{URI: "docs://prompts", MIMEType: "text/markdown", Text: content}},
	}, nil
}
