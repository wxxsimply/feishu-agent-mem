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
	"feishu-mem/internal/signal"
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
		Name:        "extract_decision",
		Description: "从文本内容中智能提取决策信息 (使用LLM)",
	}, s.handleExtractDecision)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "classify_topic",
		Description: "将决策智能分类到正确的议题",
	}, s.handleClassifyTopic)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "detect_crosstopic",
		Description: "检测决策是否会影响多个议题",
	}, s.handleDetectCrossTopic)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "check_conflict",
		Description: "评估两个决策之间是否存在冲突",
	}, s.handleCheckConflict)

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
		Name:        "list_topics",
		Description: "列出所有议题",
	}, s.handleListTopics)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "get_relations",
		Description: "获取决策的关系网络",
	}, s.handleGetRelations)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "stats",
		Description: "获取系统统计信息",
	}, s.handleStats)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "hot_decisions",
		Description: "按热点值查询决策，从高到低排序",
	}, s.handleHotDecisions)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "forgotten_decisions",
		Description: "获取被遗忘的决策（低热点值）",
	}, s.handleForgottenDecisions)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "related_decisions",
		Description: "获取与指定决策相关的决策",
	}, s.handleRelatedDecisions)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "recent_decisions",
		Description: "获取最近的决策",
	}, s.handleRecentDecisions)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "git_history",
		Description: "获取Git提交历史",
	}, s.handleGitHistory)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "git_search",
		Description: "在Git中搜索内容",
	}, s.handleGitSearch)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "git_blame",
		Description: "追溯决策的Git修改历史",
	}, s.handleGitBlame)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "decision_card",
		Description: "获取决策的飞书卡片 JSON",
	}, s.handleDecisionCard)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "search_fulltext",
		Description: "全文搜索决策记录",
	}, s.handleFulltextSearch)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "conflict_list",
		Description: "列出所有决策冲突，或指定决策的冲突",
	}, s.handleConflictList)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "objection_list",
		Description: "列出反对意见",
	}, s.handleObjectionList)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "evaluate_dedup",
		Description: "评估新决策是否与现有决策重复或冲突",
	}, s.handleEvaluateDedup)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "resolve_conflict_action",
		Description: "获取冲突解决建议（合并或保留双方）",
	}, s.handleResolveConflictAction)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "decision_history",
		Description: "获取决策的历史版本",
	}, s.handleDecisionHistory)

	mcp.AddTool(s.sdkServer, &mcp.Tool{
		Name:        "revert_decision",
		Description: "回溯决策到指定版本",
	}, s.handleRevertDecision)
}

type searchArgs struct {
	Query string  `json:"query" jsonschema:"搜索关键词"`
	Topic string  `json:"topic,omitempty" jsonschema:"议题过滤"`
	Limit float64 `json:"limit,omitempty" jsonschema:"结果限制"`
}

type topicArgs struct {
	Topic string `json:"topic" jsonschema:"议题名称"`
}

type decisionArgs struct {
	SdrID string `json:"sdr_id" jsonschema:"决策ID"`
}

type listTopicsArgs struct {
	Project string `json:"project,omitempty" jsonschema:"项目名称"`
}

type getRelationsArgs struct {
	SdrID string `json:"sdr_id" jsonschema:"决策ID"`
}

type statsArgs struct{}

type hotDecisionsArgs struct {
	MinScore float64 `json:"min_score,omitempty" jsonschema:"最低热点值"`
	Limit    float64 `json:"limit,omitempty" jsonschema:"结果限制"`
}

type forgottenDecisionsArgs struct {
	Threshold float64 `json:"threshold,omitempty" jsonschema:"阈值，低于此值被认为遗忘"`
}

type relatedDecisionsArgs struct {
	SdrID string `json:"sdr_id" jsonschema:"决策ID"`
}

type recentDecisionsArgs struct {
	Hours float64 `json:"hours,omitempty" jsonschema:"最近多少小时内的决策"`
}

type gitHistoryArgs struct {
	Path  string  `json:"path,omitempty" jsonschema:"文件路径"`
	Limit float64 `json:"limit,omitempty" jsonschema:"结果限制"`
}

type gitSearchArgs struct {
	Query   string `json:"query" jsonschema:"搜索关键词"`
	Project string `json:"project,omitempty" jsonschema:"项目名称"`
}

type decisionCardArgs struct {
	SdrID string `json:"sdr_id" jsonschema:"决策ID"`
	ChatID string `json:"chat_id,omitempty" jsonschema:"飞书群聊ID（可选）"`
}

type fulltextSearchArgs struct {
	Query   string `json:"query" jsonschema:"搜索关键词"`
	Project string `json:"project,omitempty" jsonschema:"项目过滤"`
}

type conflictListArgs struct {
	SdrID string `json:"sdr_id,omitempty" jsonschema:"决策ID（可选，返回该决策的所有冲突）"`
}

type listObjectionsArgs struct {
	Topic string  `json:"topic,omitempty" jsonschema:"议题过滤"`
	SdrID string  `json:"sdr_id,omitempty" jsonschema:"关联决策ID"`
	Limit float64 `json:"limit,omitempty" jsonschema:"结果限制"`
}

type extractDecisionArgs struct {
	Content string   `json:"content" jsonschema:"待分析的文本"`
	Topics  []string `json:"topics" jsonschema:"候选议题"`
}

type evaluateDedupArgs struct {
	NewTitle       string `json:"new_title" jsonschema:"新决策标题"`
	NewDecision    string `json:"new_decision" jsonschema:"新决策内容"`
	ExistingTitle  string `json:"existing_title" jsonschema:"现有决策标题"`
	ExistingDecision string `json:"existing_decision" jsonschema:"现有决策内容"`
}

type resolveConflictActionArgs struct {
	NewTitle         string `json:"new_title" jsonschema:"新决策标题"`
	NewDecision      string `json:"new_decision" jsonschema:"新决策内容"`
	ExistingTitle    string `json:"existing_title" jsonschema:"现有决策标题"`
	ExistingDecision string `json:"existing_decision" jsonschema:"现有决策内容"`
}

type classifyTopicArgs struct {
	Decision string   `json:"decision" jsonschema:"决策内容"`
	Topics   []string `json:"topics" jsonschema:"候选议题"`
}

type detectCrossTopicArgs struct {
	Title           string   `json:"title" jsonschema:"决策标题"`
	Decision        string   `json:"decision" jsonschema:"决策内容"`
	CandidateTopics []string `json:"candidate_topics" jsonschema:"候选议题"`
}

type checkConflictArgs struct {
	DecisionA string `json:"decision_a" jsonschema:"新决策"`
	DecisionB string `json:"decision_b" jsonschema:"现有决策"`
}

type gitBlameArgs struct {
	SdrID   string `json:"sdr_id" jsonschema:"决策ID"`
	Project string `json:"project,omitempty" jsonschema:"项目名称，默认为空"`
	Topic   string `json:"topic,omitempty" jsonschema:"议题名称，默认为空"`
}

type decisionHistoryArgs struct {
	SdrID   string `json:"sdr_id" jsonschema:"决策ID"`
	Project string `json:"project,omitempty" jsonschema:"项目名称，默认为feishu-mem"`
	Topic   string `json:"topic,omitempty" jsonschema:"议题名称，默认为general"`
}

type revertDecisionArgs struct {
	SdrID       string `json:"sdr_id" jsonschema:"决策ID"`
	TargetCommit string `json:"target_commit" jsonschema:"目标提交哈希"`
	Reason       string `json:"reason" jsonschema:"回溯原因"`
}

type createDecisionArgs struct {
	Title       string `json:"title" jsonschema:"决策标题"`
	Decision    string `json:"decision" jsonschema:"决策内容"`
	Rationale   string `json:"rationale,omitempty" jsonschema:"决策依据"`
	Topic       string `json:"topic" jsonschema:"议题"`
	Phase       string `json:"phase,omitempty" jsonschema:"阶段"`
	ImpactLevel string `json:"impact_level,omitempty" jsonschema:"影响等级"`
}

type updateDecisionArgs struct {
	SdrID    string `json:"sdr_id" jsonschema:"决策ID"`
	Title    string `json:"title,omitempty" jsonschema:"决策标题"`
	Decision string `json:"decision,omitempty" jsonschema:"决策内容"`
	Rationale string `json:"rationale,omitempty" jsonschema:"决策依据"`
	Status   string `json:"status,omitempty" jsonschema:"状态"`
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
		// 记录访问
		for _, r := range results {
			_ = s.memoryGraph.UpdateAccessStats(r.SDRID)
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
		// 记录访问
		for _, d := range decisions {
			_ = s.memoryGraph.UpdateAccessStats(d.SDRID)
		}
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
		if found && d != nil {
			_ = s.memoryGraph.UpdateAccessStats(args.SdrID) // 记录访问
		}
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

func (s *MemoryMCPServer) handleListTopics(ctx context.Context, req *mcp.CallToolRequest, args listTopicsArgs) (*mcp.CallToolResult, emptyResult, error) {
	var topics []string
	if s.memoryGraph != nil {
		topics = s.memoryGraph.ListAllTopics(args.Project)
	}

	text := "## 所有议题\n\n"
	if len(topics) == 0 {
		text += "暂无议题"
	} else {
		for _, t := range topics {
			text += fmt.Sprintf("- %s\n", t)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleGetRelations(ctx context.Context, req *mcp.CallToolRequest, args getRelationsArgs) (*mcp.CallToolResult, emptyResult, error) {
	var relations []decision.Relation
	var relatedDecisions []*decision.DecisionNode
	if s.memoryGraph != nil {
		relations = s.memoryGraph.GetRelations(args.SdrID)
		relatedDecisions = s.memoryGraph.GetRelatedDecisions(args.SdrID)
	}

	text := fmt.Sprintf("## 决策 %s 的关系网络\n\n", args.SdrID)
	text += "### 直接关系\n\n"
	if len(relations) == 0 {
		text += "暂无关系\n"
	} else {
		for _, rel := range relations {
			text += fmt.Sprintf("- [%s] → %s: %s\n", rel.Type, rel.TargetSDRID, rel.Description)
		}
	}
	text += "\n### 相关决策\n\n"
	if len(relatedDecisions) == 0 {
		text += "暂无相关决策"
	} else {
		for _, d := range relatedDecisions {
			text += fmt.Sprintf("- [%s] %s (%s)\n", d.Status, d.Title, d.SDRID)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleStats(ctx context.Context, req *mcp.CallToolRequest, args statsArgs) (*mcp.CallToolResult, emptyResult, error) {
	var decisionCount, topicCount int
	if s.memoryGraph != nil {
		decisionCount = s.memoryGraph.Count()
		topicCount = s.memoryGraph.TopicCount("")
	}

	text := "## 系统统计\n\n"
	text += fmt.Sprintf("- **总决策数**: %d\n", decisionCount)
	text += fmt.Sprintf("- **议题数**: %d\n", topicCount)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleHotDecisions(ctx context.Context, req *mcp.CallToolRequest, args hotDecisionsArgs) (*mcp.CallToolResult, emptyResult, error) {
	minScore := 0.0
	if args.MinScore > 0 {
		minScore = args.MinScore
	}
	limit := 20
	if args.Limit > 0 {
		limit = int(args.Limit)
	}

	var decisions []*decision.DecisionNode
	if s.memoryGraph != nil {
		decisions = s.memoryGraph.GetDecisionsByHotScore(minScore)
		if len(decisions) > limit {
			decisions = decisions[:limit]
		}
		// 记录访问
		for _, d := range decisions {
			_ = s.memoryGraph.UpdateAccessStats(d.SDRID)
		}
	}

	text := "## 热点决策排行\n\n"
	if len(decisions) == 0 {
		text += "暂无数据"
	} else {
		for i, d := range decisions {
			cat := "遗忘"
			if d.AccessStats.HotScore >= 80 {
				cat = "活跃"
			} else if d.AccessStats.HotScore >= 50 {
				cat = "正常"
			} else if d.AccessStats.HotScore >= 20 {
				cat = "模糊"
			}
			text += fmt.Sprintf("%d. [%s] %s (%.0f分, %s)\n", i+1, d.Status, d.Title, d.AccessStats.HotScore, cat)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleForgottenDecisions(ctx context.Context, req *mcp.CallToolRequest, args forgottenDecisionsArgs) (*mcp.CallToolResult, emptyResult, error) {
	threshold := 20.0
	if args.Threshold > 0 {
		threshold = args.Threshold
	}

	var forgotten []*decision.DecisionNode
	if s.memoryGraph != nil {
		allDecisions := s.memoryGraph.GetAllDecisions()
		for _, d := range allDecisions {
			if d.AccessStats.HotScore < threshold {
				forgotten = append(forgotten, d)
			}
		}
	}

	text := "## 💤 被遗忘的决策\n\n"
	if len(forgotten) == 0 {
		text += "暂无被遗忘的决策（或阈值设置过高）"
	} else {
		for _, d := range forgotten {
			text += fmt.Sprintf("- [热点值: %.0f] %s (%s) - %s\n", d.AccessStats.HotScore, d.Title, d.SDRID, d.Topic)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleRelatedDecisions(ctx context.Context, req *mcp.CallToolRequest, args relatedDecisionsArgs) (*mcp.CallToolResult, emptyResult, error) {
	var related []*decision.DecisionNode
	if s.memoryGraph != nil {
		related = s.memoryGraph.GetRelatedDecisions(args.SdrID)
	}

	text := fmt.Sprintf("## 与 %s 相关的决策\n\n", args.SdrID)
	if len(related) == 0 {
		text += "暂无相关决策"
	} else {
		for _, d := range related {
			text += fmt.Sprintf("- [%s] %s (%s)\n", d.Status, d.Title, d.SDRID)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleRecentDecisions(ctx context.Context, req *mcp.CallToolRequest, args recentDecisionsArgs) (*mcp.CallToolResult, emptyResult, error) {
	hours := 24.0
	if args.Hours > 0 {
		hours = args.Hours
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	var decisions []*decision.DecisionNode
	if s.memoryGraph != nil {
		decisions = s.memoryGraph.GetRecentDecisions(since)
		// 记录访问
		for _, d := range decisions {
			_ = s.memoryGraph.UpdateAccessStats(d.SDRID)
		}
	}

	text := fmt.Sprintf("## 最近 %.0f 小时的决策\n\n", hours)
	if len(decisions) == 0 {
		text += "暂无最近决策"
	} else {
		for _, d := range decisions {
			text += fmt.Sprintf("- %s [%s] %s (%s)\n", d.CreatedAt.Format("15:04"), d.Status, d.Title, d.SDRID)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleGitHistory(ctx context.Context, req *mcp.CallToolRequest, args gitHistoryArgs) (*mcp.CallToolResult, emptyResult, error) {
	limit := 10
	if args.Limit > 0 {
		limit = int(args.Limit)
	}

	var history []git.CommitLogEntry
	if s.gitStorage != nil {
		var err error
		history, err = s.gitStorage.GetCommitLog(args.Path, limit)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("获取Git历史失败: %v", err)}},
			}, emptyResult{}, nil
		}
	}

	text := "## Git提交历史\n\n"
	if len(history) == 0 {
		text += "暂无提交记录"
	} else {
		for _, h := range history {
			text += fmt.Sprintf("- %s: %s\n", h.Hash, h.Message)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleGitSearch(ctx context.Context, req *mcp.CallToolRequest, args gitSearchArgs) (*mcp.CallToolResult, emptyResult, error) {
	var hits []git.SearchHit
	if s.gitStorage != nil {
		var err error
		hits, err = s.gitStorage.SearchContent(args.Project, args.Query)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Git搜索失败: %v", err)}},
			}, emptyResult{}, nil
		}
	}

	text := fmt.Sprintf("## Git搜索: \"%s\"\n\n", args.Query)
	if len(hits) == 0 {
		text += "未找到匹配内容"
	} else {
		for _, h := range hits {
			text += fmt.Sprintf("- %s:%d: %s\n", h.File, h.LineNum, h.Content)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleDecisionCard(ctx context.Context, req *mcp.CallToolRequest, args decisionCardArgs) (*mcp.CallToolResult, emptyResult, error) {
	d, found := s.memoryGraph.GetDecision(args.SdrID)
	if !found || d == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "未找到指定的决策"}},
		}, emptyResult{}, nil
	}

	text := fmt.Sprintf("## 决策卡片: %s\n\n", d.Title)
	text += fmt.Sprintf("- **SDR ID**: %s\n", d.SDRID)
	text += fmt.Sprintf("- **议题**: %s\n", d.Topic)
	text += fmt.Sprintf("- **状态**: %s\n", d.Status)
	text += fmt.Sprintf("- **影响等级**: %s\n", d.ImpactLevel)
	text += fmt.Sprintf("- **热点值**: %.0f\n", d.AccessStats.HotScore)
	if d.AccessStats.HotScore >= 80 {
		text += "\n🔥 **活跃决策** — 近期频繁被引用/访问"
	} else if d.AccessStats.HotScore >= 50 {
		text += "\n✅ **正常决策** — 处于常规使用状态"
	} else if d.AccessStats.HotScore >= 20 {
		text += "\n🌫️ **模糊决策** — 回忆度较低，建议回顾"
	} else {
		text += "\n💤 **遗忘决策** — 长期未被引用，可能已过时"
	}
	text += fmt.Sprintf("\n\n### 决策内容\n\n%s\n\n", d.Decision)
	if d.Rationale != "" {
		text += fmt.Sprintf("**依据**: %s\n\n", d.Rationale)
	}
	text += fmt.Sprintf("**提出人**: %s | **执行人**: %s\n", d.Proposer, d.Executor)

	relations := s.memoryGraph.GetRelations(args.SdrID)
	for _, rel := range relations {
		if rel.Type == decision.RelationConflictsWith {
			text += fmt.Sprintf("\n⚠️ **冲突**: %s\n", rel.Description)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleFulltextSearch(ctx context.Context, req *mcp.CallToolRequest, args fulltextSearchArgs) (*mcp.CallToolResult, emptyResult, error) {
	var results []*decision.DecisionNode
	if s.memoryGraph != nil {
		results = s.memoryGraph.SearchByKeywords(args.Query, "")
	}

	text := fmt.Sprintf("## 全文搜索: %s\n\n", args.Query)
	if len(results) == 0 {
		text += "未找到匹配的决策"
	} else {
		text += fmt.Sprintf("共找到 %d 个匹配\n\n", len(results))
		for _, r := range results[:min(20, len(results))] {
			text += fmt.Sprintf("- [%s] **%s** (%s)\n  %s\n", r.Status, r.Title, r.SDRID, truncateStr(r.Decision, 100))
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleConflictList(ctx context.Context, req *mcp.CallToolRequest, args conflictListArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := "## 决策冲突列表\n\n"
	found := false

	if args.SdrID != "" {
		relations := s.memoryGraph.GetRelations(args.SdrID)
		text += fmt.Sprintf("### 决策 %s 的冲突\n\n", args.SdrID)
		for _, rel := range relations {
			if rel.Type == decision.RelationConflictsWith {
				found = true
				text += fmt.Sprintf("- ⚠️ %s → %s\n", args.SdrID, rel.TargetSDRID)
				text += fmt.Sprintf("  %s\n", rel.Description)
			}
		}
	} else {
		allDecisions := s.memoryGraph.GetAllDecisions()
		for _, d := range allDecisions {
			for _, rel := range d.Relations {
				if rel.Type == decision.RelationConflictsWith {
					found = true
					text += fmt.Sprintf("- ⚠️ %s ↔ %s\n", d.SDRID, rel.TargetSDRID)
					text += fmt.Sprintf("  %s: %s\n", d.Title, rel.Description)
				}
			}
		}
	}

	if !found {
		text += "未发现决策冲突"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleObjectionList(ctx context.Context, req *mcp.CallToolRequest, args listObjectionsArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := "## 反对意见列表\n\n"

	limit := 20
	if args.Limit > 0 {
		limit = int(args.Limit)
	}

	if s.gitStorage != nil {
		objections, err := s.gitStorage.ListObjections("feishu-mem", args.Topic)
		if err != nil {
			text += fmt.Sprintf("查询失败: %v", err)
		} else if len(objections) == 0 {
			text += "暂无反对意见"
		} else {
			text += fmt.Sprintf("共 %d 条反对意见\n\n", len(objections))
			count := 0
			for _, obj := range objections {
				if count >= limit {
					break
				}
				if args.SdrID != "" && obj.ReferencesDecision != args.SdrID {
					continue
				}
				text += fmt.Sprintf("- **%s**: %s\n", obj.OID, obj.ObjectionContent)
				text += fmt.Sprintf("  反对人: %s | 状态: %s | 来源: %s\n", obj.Objector, obj.Status, obj.SourceType)
				if obj.ReferencesDecision != "" {
					text += fmt.Sprintf("  关联决策: %s\n", obj.ReferencesDecision)
				}
				count++
			}
		}
	} else {
		text += "Git 存储未初始化"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleEvaluateDedup(ctx context.Context, req *mcp.CallToolRequest, args evaluateDedupArgs) (*mcp.CallToolResult, emptyResult, error) {
	result, err := s.llmAgent.EvaluateDedupAction(args.NewTitle, args.NewDecision, args.ExistingTitle, args.ExistingDecision)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("评估失败: %v", err)}},
		}, emptyResult{}, nil
	}

	text := "## 去重评估结果\n\n"
	text += fmt.Sprintf("- **动作**: %s\n", result.Action)
	text += fmt.Sprintf("- **原因**: %s\n", result.Reason)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleResolveConflictAction(ctx context.Context, req *mcp.CallToolRequest, args resolveConflictActionArgs) (*mcp.CallToolResult, emptyResult, error) {
	result, err := s.llmAgent.ResolveConflictAction(args.NewTitle, args.NewDecision, args.ExistingTitle, args.ExistingDecision)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("获取建议失败: %v", err)}},
		}, emptyResult{}, nil
	}

	text := "## 冲突解决建议\n\n"
	text += fmt.Sprintf("- **建议动作**: %s\n", result.Action)
	text += fmt.Sprintf("- **原因**: %s\n", result.Reason)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleCreateDecision(ctx context.Context, req *mcp.CallToolRequest, args createDecisionArgs) (*mcp.CallToolResult, emptyResult, error) {
	impactLevel := args.ImpactLevel
	if impactLevel == "" {
		impactLevel = "minor"
	}

	d := decision.NewDecisionNode(signal.GenerateSDRID(), args.Title, "", args.Topic)
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

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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

func (s *MemoryMCPServer) handleExtractDecision(ctx context.Context, req *mcp.CallToolRequest, args extractDecisionArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := "## 决策提取结果\n\n"

	if len(args.Content) == 0 {
		text += "未提供文本内容"
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	if !s.llmAgent.IsAvailable() {
		text += "### 输入内容\n" + args.Content + "\n\n"
		text += "⚠️ LLM不可用，无法进行智能提取\n\n"
		if len(args.Topics) > 0 {
			text += "### 候选议题\n"
			for _, t := range args.Topics {
				text += "- " + t + "\n"
			}
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	result, err := s.llmAgent.ExtractDecision(args.Content, args.Topics)
	if err != nil {
		text += "提取失败: " + err.Error()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	text += fmt.Sprintf("- **是否包含决策**: %v\n", result.HasDecision)
	text += fmt.Sprintf("- **置信度**: %.2f\n", result.Confidence)

	if result.HasDecision && result.Decision != nil {
		text += fmt.Sprintf("- **标题**: %s\n", result.Decision.Title)
		text += fmt.Sprintf("- **决策**: %s\n", result.Decision.Decision)
		text += fmt.Sprintf("- **依据**: %s\n", result.Decision.Rationale)
		text += fmt.Sprintf("- **建议议题**: %s\n", result.Decision.SuggestedTopic)
		text += fmt.Sprintf("- **影响级别**: %s\n", result.Decision.ImpactLevel)
		text += fmt.Sprintf("- **提出人**: %s\n", result.Decision.Proposer)
		text += fmt.Sprintf("- **执行人**: %s\n", result.Decision.Executor)
	}

	if result.HasObjections && len(result.Objections) > 0 {
		text += "\n### 反对意见\n"
		for _, obj := range result.Objections {
			text += fmt.Sprintf("- **%s**: %s\n", obj.Objector, obj.ObjectionContent)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleClassifyTopic(ctx context.Context, req *mcp.CallToolRequest, args classifyTopicArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := "## 议题分类结果\n\n"

	if !s.llmAgent.IsAvailable() {
		text += "⚠️ LLM不可用\n\n"
		text += "### 候选议题\n"
		for _, t := range args.Topics {
			text += "- " + t + "\n"
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	result, err := s.llmAgent.ClassifyTopic(args.Decision, args.Topics)
	if err != nil {
		text += "分类失败: " + err.Error()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	text += fmt.Sprintf("- **建议议题**: %s\n", result.Topic)
	text += fmt.Sprintf("- **置信度**: %.2f\n", result.Confidence)
	text += fmt.Sprintf("- **说明**: %s\n", result.Reasoning)

	if len(result.AlternativeTopics) > 0 {
		text += "\n### 替代议题\n"
		for _, t := range result.AlternativeTopics {
			text += "- " + t + "\n"
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleDetectCrossTopic(ctx context.Context, req *mcp.CallToolRequest, args detectCrossTopicArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := "## 跨议题检测结果\n\n"

	if !s.llmAgent.IsAvailable() {
		text += "⚠️ LLM不可用\n\n"
		text += "### 候选议题\n"
		for _, t := range args.CandidateTopics {
			text += "- " + t + "\n"
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	node := map[string]any{
		"title":           args.Title,
		"decision":        args.Decision,
		"candidate_topics": args.CandidateTopics,
	}

	result, err := s.llmAgent.DetectCrossTopic(node)
	if err != nil {
		text += "检测失败: " + err.Error()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	text += fmt.Sprintf("- **是否跨议题**: %v\n", result.IsCrossTopic)
	text += fmt.Sprintf("- **置信度**: %.2f\n", result.Confidence)

	if result.IsCrossTopic && len(result.CrossTopicRefs) > 0 {
		text += "\n### 受影响议题\n"
		for _, t := range result.CrossTopicRefs {
			text += "- " + t
			if reason, ok := result.Reasons[t]; ok {
				text += ": " + reason
			}
			text += "\n"
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleCheckConflict(ctx context.Context, req *mcp.CallToolRequest, args checkConflictArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := "## 冲突评估结果\n\n"

	if !s.llmAgent.IsAvailable() {
		text += "⚠️ LLM不可用\n"
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	result, err := s.llmAgent.ResolveConflict(args.DecisionA, args.DecisionB)
	if err != nil {
		text += "评估失败: " + err.Error()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	text += fmt.Sprintf("- **冲突分数**: %.2f\n", result.ContradictionScore)
	text += fmt.Sprintf("- **冲突类型**: %s\n", result.ContradictionType)
	text += fmt.Sprintf("- **描述**: %s\n", result.Description)
	text += fmt.Sprintf("- **建议动作**: %s\n", result.Action)
	text += fmt.Sprintf("- **需要用户介入**: %v\n", result.NeedsUser)

	if result.Suggestion != "" {
		text += fmt.Sprintf("\n### 具体建议\n%s\n", result.Suggestion)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleGitBlame(ctx context.Context, req *mcp.CallToolRequest, args gitBlameArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := fmt.Sprintf("## Git追溯: %s\n\n", args.SdrID)

	if s.gitStorage == nil {
		text += "Git存储未初始化"
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	project := args.Project
	if project == "" {
		project = "feishu-mem"
	}
	topic := args.Topic
	if topic == "" {
		topic = "general"
	}

	blame, err := s.gitStorage.BlameDecision(project, topic, args.SdrID)
	if err != nil {
		text += "追溯失败: " + err.Error()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	if len(blame) == 0 {
		text += "暂无追溯信息"
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	for _, b := range blame {
		text += fmt.Sprintf("- **%s** (%s) 行%d:\n", b.Commit[:7], b.Author, b.LineNum)
		text += fmt.Sprintf("  %s\n", b.Content)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleDecisionHistory(ctx context.Context, req *mcp.CallToolRequest, args decisionHistoryArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := fmt.Sprintf("## 决策 %s 的历史版本\n\n", args.SdrID)

	if s.gitStorage == nil {
		text += "Git存储未初始化"
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	project := args.Project
	if project == "" {
		project = "feishu-mem"
	}
	topic := args.Topic
	if topic == "" {
		topic = "general"
	}

	history, err := s.gitStorage.GetDecisionHistory(project, topic, args.SdrID)
	if err != nil {
		text += "获取历史失败: " + err.Error()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	if len(history) == 0 {
		text += "暂无历史记录"
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, emptyResult{}, nil
	}

	for _, entry := range history {
		text += fmt.Sprintf("- **%s**: %s\n", entry.Hash, entry.Message)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}

func (s *MemoryMCPServer) handleRevertDecision(ctx context.Context, req *mcp.CallToolRequest, args revertDecisionArgs) (*mcp.CallToolResult, emptyResult, error) {
	text := fmt.Sprintf("## 回溯决策 %s 到版本 %s\n\n", args.SdrID, args.TargetCommit)
	text += fmt.Sprintf("原因: %s\n\n", args.Reason)
	text += "⚠️ 注意: 此功能需要通过 PipelineEngine 执行，当前 MCP 工具未集成完整的 Pipeline 依赖。\n"
	text += "请使用 mem-service 内部的状态机来执行回溯操作。"

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, emptyResult{}, nil
}
