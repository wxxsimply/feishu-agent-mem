package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/llm"
)

// MemoryGraphInterface 内存图接口
type MemoryGraphInterface interface {
	GetAllDecisions() []*decision.DecisionNode
	GetDecision(sdrID string) (*decision.DecisionNode, bool)
	QueryByTopic(project, topic string) []*decision.DecisionNode
	SearchByKeywords(query, topic string) []*decision.DecisionNode
}

// GitStorageInterface Git 存储接口
type GitStorageInterface interface {
	ReadDecision(project, topic, sdrID string) (*decision.DecisionNode, error)
}

// BitableStoreInterface Bitable 存储接口
type BitableStoreInterface interface {
	QueryByTopic(topic, status string) ([]*decision.DecisionNode, error)
}

// MCPServer MCP 服务器
type MCPServer struct {
	memoryGraph  MemoryGraphInterface
	gitStorage   GitStorageInterface
	bitableStore BitableStoreInterface
	llmAgent     *llm.MemoryAgent
	in           io.Reader
	out          io.Writer
	mu           sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	sema         chan struct{} // 并发限制 semaphore
	initialized  bool          // 是否已完成 initialize 握手
}

const maxConcurrency = 20

// NewMCPServer 创建 MCP Server
func NewMCPServer(
	mg MemoryGraphInterface,
	gs GitStorageInterface,
	bs BitableStoreInterface,
) *MCPServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &MCPServer{
		memoryGraph:  mg,
		gitStorage:   gs,
		bitableStore: bs,
		llmAgent:     llm.NewMemoryAgent(),
		in:           os.Stdin,
		out:          os.Stdout,
		ctx:          ctx,
		cancel:       cancel,
		sema:         make(chan struct{}, maxConcurrency),
	}
}

// SetIO 设置输入输出
func (s *MCPServer) SetIO(in io.Reader, out io.Writer) {
	s.in = in
	s.out = out
}

// Start 启动 MCP Server
func (s *MCPServer) Start() error {
	fmt.Fprintf(os.Stderr, "Feishu Memory MCP Server starting...\n")
	fmt.Fprintf(os.Stderr, "Available tools: search, topic, decision, extract_decision, classify_topic, detect_crosstopic, check_conflict, timeline\n")
	fmt.Fprintf(os.Stderr, "Available resources: docs://design, docs://prompts\n")

	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB 缓冲区，支持大文本
	for scanner.Scan() {
		if s.ctx.Err() != nil {
			break
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.sendError(nil, ErrCodeParseError, "parse error: "+err.Error())
			continue
		}

		// 控制并发：获取 semaphore 槽位
		select {
		case s.sema <- struct{}{}:
		case <-s.ctx.Done():
			break
		}
		s.wg.Add(1)
		go func(r Request) {
			defer func() {
				<-s.sema
				s.wg.Done()
			}()
			s.handleRequest(r)
		}(req)
	}

	s.wg.Wait()
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

// Stop 停止 MCP Server
func (s *MCPServer) Stop() error {
	s.cancel()
	return nil
}

func (s *MCPServer) handleRequest(req Request) {
	// 请求日志
	start := time.Now()
	fmt.Fprintf(os.Stderr, "[MCP] --> %s id=%v\n", req.Method, req.ID)

	// pre-initialization 检查：除 initialize 和 notifications 外拒绝所有请求
	if !s.initialized && req.Method != "initialize" && req.Method != "notifications/initialized" {
		s.sendError(req.ID, ErrCodeInvalidRequest, "server not initialized, call initialize first")
		fmt.Fprintf(os.Stderr, "[MCP] <-- %s id=%v error=not_initialized duration=%v\n", req.Method, req.ID, time.Since(start))
		return
	}

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// 客户端通知初始化完成，无需响应
	case "tools/list":
		s.handleListTools(req)
	case "tools/call":
		s.handleCallTool(req)
	case "resources/list":
		s.handleListResources(req)
	case "resources/read":
		s.handleReadResource(req)
	case "resources/templates/list":
		s.sendResponse(req.ID, map[string]any{"resourceTemplates": []any{}})
	case "prompts/list":
		s.handleListPrompts(req)
	case "prompts/get":
		s.handleGetPrompt(req)
	default:
		s.sendError(req.ID, ErrCodeMethodNotFound, "unknown method: "+req.Method)
	}
}

func (s *MCPServer) handleInitialize(req Request) {
	s.initialized = true
	s.sendResponse(req.ID, map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{
				"listChanged": true,
			},
			"resources": map[string]any{
				"listChanged": true,
			},
		},
		"serverInfo": map[string]any{
			"name":    "Feishu Memory Agent",
			"version": "1.0.0",
		},
		"instructions": "Feishu Memory Agent 提供决策记忆管理功能，包括搜索、分类、冲突检测等",
	})
}

func (s *MCPServer) handleListTools(req Request) {
	tools := []Tool{
		{
			Name:        "search",
			Description: "搜索记忆系统中的决策记录，支持关键词、议题过滤",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "搜索关键词"},
					"topic": map[string]any{"type": "string", "description": "议题过滤"},
					"limit": map[string]any{"type": "number", "description": "结果限制", "default": 20},
				},
			},
		},
		{
			Name:        "topic",
			Description: "查询指定议题的所有决策记录",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"topic": map[string]any{"type": "string", "description": "议题名称"},
				},
				"required": []string{"topic"},
			},
		},
		{
			Name:        "decision",
			Description: "获取单个决策的详细信息",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sdr_id": map[string]any{"type": "string", "description": "决策ID"},
				},
				"required": []string{"sdr_id"},
			},
		},
		{
			Name:        "extract_decision",
			Description: "从文本内容中智能提取决策信息",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{"type": "string", "description": "待分析的文本"},
					"topics":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "候选议题"},
				},
				"required": []string{"content"},
			},
		},
		{
			Name:        "classify_topic",
			Description: "将决策智能分类到正确的议题",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"decision": map[string]any{"type": "string", "description": "决策内容"},
					"topics":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "候选议题"},
				},
				"required": []string{"decision", "topics"},
			},
		},
		{
			Name:        "detect_crosstopic",
			Description: "检测决策是否会影响多个议题",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":            map[string]any{"type": "string", "description": "决策标题"},
					"decision":         map[string]any{"type": "string", "description": "决策内容"},
					"candidate_topics": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "候选议题"},
				},
				"required": []string{"title", "decision", "candidate_topics"},
			},
		},
		{
			Name:        "check_conflict",
			Description: "评估两个决策之间是否存在冲突",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"decision_a": map[string]any{"type": "string", "description": "新决策"},
					"decision_b": map[string]any{"type": "string", "description": "已有决策"},
				},
				"required": []string{"decision_a", "decision_b"},
			},
		},
		{
			Name:        "timeline",
			Description: "获取决策历史时间线",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	s.sendResponse(req.ID, map[string]any{"tools": tools})
}

func (s *MCPServer) handleCallTool(req Request) {
	var params map[string]any
	if p, ok := req.Params.(map[string]any); ok {
		params = p
	} else {
		params = make(map[string]any)
	}

	name, _ := params["name"].(string)
	if name == "" {
		s.sendError(req.ID, ErrCodeInvalidParams, "missing required parameter: name")
		return
	}

	args, ok := params["arguments"].(map[string]any)
	if !ok {
		args = make(map[string]any)
	}

	// 校验必需参数
	if err := s.validateToolArgs(name, args); err != "" {
		s.sendError(req.ID, ErrCodeInvalidParams, err)
		return
	}

	var content []Content

	switch name {
	case "search":
		content = s.handleSearch(args)
	case "topic":
		content = s.handleTopic(args)
	case "decision":
		content = s.handleDecision(args)
	case "extract_decision":
		content = s.handleExtract(args)
	case "classify_topic":
		content = s.handleClassify(args)
	case "detect_crosstopic":
		content = s.handleCrossTopic(args)
	case "check_conflict":
		content = s.handleConflict(args)
	case "timeline":
		content = s.handleTimeline(args)
	default:
		s.sendError(req.ID, ErrCodeToolNotFound, "unknown tool: "+name)
		return
	}

	s.sendResponse(req.ID, map[string]any{"content": content})
}

func (s *MCPServer) handleListResources(req Request) {
	resources := []Resource{
		{
			URI:      "docs://design",
			Name:     "系统设计文档",
			MimeType: "text/markdown",
		},
		{
			URI:      "docs://prompts",
			Name:     "LLM提示词模板",
			MimeType: "text/markdown",
		},
	}
	s.sendResponse(req.ID, map[string]any{"resources": resources})
}

func (s *MCPServer) handleReadResource(req Request) {
	var params map[string]any
	if p, ok := req.Params.(map[string]any); ok {
		params = p
	} else {
		params = make(map[string]any)
	}

	uri, _ := params["uri"].(string)
	var content string

	switch uri {
	case "docs://design":
		content = `# Feishu Memory Agent 系统设计

## 核心概念
- **Decision Node**: 决策节点，记录项目决策
- **Topic**: 议题，用于组织决策
- **Signal**: 信号，触发决策提取的事件

## 主要模块
1. **internal/core**: 核心数据结构
2. **internal/decision**: 决策树管理
3. **internal/llm**: LLM 智能处理模块
4. **internal/storage/git**: Git 持久化存储
5. **internal/mcp**: MCP 服务器（本模块）
`
	case "docs://prompts":
		content = `# LLM 提示词模板

## 1. 决策提取 (extraction)
用于从文本中识别和提取决策信息

## 2. 议题分类 (classification)
将决策归类到正确的议题

## 3. 跨议题检测 (crosstopic)
检测决策是否影响多个议题

## 4. 冲突评估 (conflict)
评估两个决策之间的冲突类型和程度
`
	default:
		content = "Resource not found"
	}

	contents := []ResourceContent{{
		URI:      uri,
		MimeType: "text/markdown",
		Text:     content,
	}}
	s.sendResponse(req.ID, map[string]any{"contents": contents})
}

func (s *MCPServer) handleListPrompts(req Request) {
	prompts := []map[string]any{
		{
			"name":        "extract_decision",
			"description": "从文本内容中智能提取决策信息",
			"arguments": []map[string]any{
				{"name": "content", "description": "待分析的文本", "required": true},
				{"name": "topics", "description": "候选议题列表", "required": false},
			},
		},
		{
			"name":        "classify_topic",
			"description": "将决策智能分类到正确的议题",
			"arguments": []map[string]any{
				{"name": "decision", "description": "决策内容", "required": true},
				{"name": "topics", "description": "候选议题列表", "required": true},
			},
		},
		{
			"name":        "detect_crosstopic",
			"description": "检测决策是否会影响多个议题",
			"arguments": []map[string]any{
				{"name": "title", "description": "决策标题", "required": true},
				{"name": "decision", "description": "决策内容", "required": true},
				{"name": "candidate_topics", "description": "候选议题列表", "required": true},
			},
		},
		{
			"name":        "check_conflict",
			"description": "评估两个决策之间是否存在冲突",
			"arguments": []map[string]any{
				{"name": "decision_a", "description": "新决策", "required": true},
				{"name": "decision_b", "description": "已有决策", "required": true},
			},
		},
	}
	s.sendResponse(req.ID, map[string]any{"prompts": prompts})
}

func (s *MCPServer) handleGetPrompt(req Request) {
	var params map[string]any
	if p, ok := req.Params.(map[string]any); ok {
		params = p
	} else {
		params = make(map[string]any)
	}

	name, _ := params["name"].(string)
	prompts := map[string]map[string]any{
		"extract_decision": {
			"name":        "extract_decision",
			"description": "从文本中提取决策信息",
			"prompt":      "从以下文本中提取决策信息，包括决策标题、内容、建议议题、负责人等。",
		},
		"classify_topic": {
			"name":        "classify_topic",
			"description": "将决策归类到正确的议题",
			"prompt":      "将给定的决策内容归类到最匹配的议题分类中。",
		},
		"detect_crosstopic": {
			"name":        "detect_crosstopic",
			"description": "检测跨议题影响",
			"prompt":      "判断给定的决策是否会对其他议题产生影响，并列出受影响的议题。",
		},
		"check_conflict": {
			"name":        "check_conflict",
			"description": "评估决策冲突",
			"prompt":      "评估两个决策之间是否存在语义矛盾，给出矛盾分数和类型。",
		},
	}

	if prompt, ok := prompts[name]; ok {
		s.sendResponse(req.ID, map[string]any{"prompt": prompt})
	} else {
		s.sendError(req.ID, ErrCodeInvalidParams, "unknown prompt: "+name)
	}
}

func (s *MCPServer) handleSearch(args map[string]any) []Content {
	query := getStringArg(args, "query", "")
	topic := getStringArg(args, "topic", "")
	limit := int(getNumberArg(args, "limit", 20))

	var results []SearchResult
	if s.memoryGraph != nil {
		decisions := s.memoryGraph.SearchByKeywords(query, topic)
		for _, d := range decisions {
			results = append(results, SearchResult{
				SDRID:       d.SDRID,
				Title:       d.Title,
				Topic:       d.Topic,
				ImpactLevel: string(d.ImpactLevel),
				Status:      string(d.Status),
			})
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}

	text := formatSearchResults(results)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) handleTopic(args map[string]any) []Content {
	topic := getStringArg(args, "topic", "")

	var decisions []*decision.DecisionNode
	if s.memoryGraph != nil {
		decisions = s.memoryGraph.QueryByTopic("", topic)
	}

	text := formatTopicResults(topic, decisions)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) handleDecision(args map[string]any) []Content {
	sdrID := getStringArg(args, "sdr_id", "")

	var d *decision.DecisionNode
	found := false
	if s.memoryGraph != nil {
		d, found = s.memoryGraph.GetDecision(sdrID)
	}

	text := formatDecisionResult(d, found)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) handleExtract(args map[string]any) []Content {
	content := getStringArg(args, "content", "")
	topics := getStringArrayArg(args, "topics")

	result, err := s.llmAgent.ExtractDecision(content, topics)
	if err != nil {
		return []Content{{Type: "text", Text: fmt.Sprintf("提取失败: %v", err)}}
	}

	text := formatExtractResult(result)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) handleClassify(args map[string]any) []Content {
	decisionStr := getStringArg(args, "decision", "")
	topics := getStringArrayArg(args, "topics")

	result, err := s.llmAgent.ClassifyTopic(decisionStr, topics)
	if err != nil {
		return []Content{{Type: "text", Text: fmt.Sprintf("分类失败: %v", err)}}
	}

	text := formatClassifyResult(result)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) handleCrossTopic(args map[string]any) []Content {
	result, err := s.llmAgent.DetectCrossTopic(args)
	if err != nil {
		return []Content{{Type: "text", Text: fmt.Sprintf("检测失败: %v", err)}}
	}

	text := formatCrossTopicResult(result)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) handleConflict(args map[string]any) []Content {
	decisionA := getStringArg(args, "decision_a", "")
	decisionB := getStringArg(args, "decision_b", "")

	result, err := s.llmAgent.ResolveConflict(decisionA, decisionB)
	if err != nil {
		return []Content{{Type: "text", Text: fmt.Sprintf("冲突评估失败: %v", err)}}
	}

	text := formatConflictResult(result)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) handleTimeline(args map[string]any) []Content {
	items := []TimelineItem{}
	if s.memoryGraph != nil {
		for _, d := range s.memoryGraph.GetAllDecisions() {
			ts := d.CreatedAt
			if d.DecidedAt != nil {
				ts = *d.DecidedAt
			}
			items = append(items, TimelineItem{
				Timestamp: ts,
				Event:     d.Title,
				SDRID:     d.SDRID,
			})
		}
	}
	text := formatTimelineResults(items)
	return []Content{{Type: "text", Text: text}}
}

func (s *MCPServer) sendResponse(id any, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[MCP] sendResponse marshal error: %v\n", err)
		fmt.Fprintf(s.out, `{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`+"\n")
		return
	}
	fmt.Fprintf(s.out, "%s\n", data)
	fmt.Fprintf(os.Stderr, "[MCP] <-- response id=%v size=%d\n", id, len(data))
}

func (s *MCPServer) sendError(id any, code int, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: errMsg},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[MCP] sendError marshal error: %v\n", err)
		fmt.Fprintf(s.out, `{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`+"\n")
		return
	}
	fmt.Fprintf(s.out, "%s\n", data)
	fmt.Fprintf(os.Stderr, "[MCP] <-- error id=%v code=%d message=%q\n", id, code, errMsg)
}

func getStringArg(args map[string]any, key, defaultValue string) string {
	if val, ok := args[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func getNumberArg(args map[string]any, key string, defaultValue float64) float64 {
	if val, ok := args[key]; ok {
		if num, ok := val.(float64); ok {
			return num
		}
	}
	return defaultValue
}

func getStringArrayArg(args map[string]any, key string) []string {
	if val, ok := args[key]; ok {
		if arr, ok := val.([]any); ok {
			var result []string
			for _, item := range arr {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return []string{}
}

func formatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "未找到匹配的决策记录"
	}
	text := "## 搜索结果\n\n"
	for _, r := range results {
		text += fmt.Sprintf("- [%s] %s (%s) - %s\n", r.Status, r.Title, r.SDRID, r.Topic)
	}
	return text
}

func formatTopicResults(topic string, decisions []*decision.DecisionNode) string {
	text := fmt.Sprintf("## 议题: %s\n\n", topic)
	text += fmt.Sprintf("共 %d 个决策\n\n", len(decisions))
	for _, d := range decisions {
		text += fmt.Sprintf("- [%s] %s (%s)\n", d.Status, d.Title, d.SDRID)
	}
	return text
}

func formatDecisionResult(d *decision.DecisionNode, found bool) string {
	if !found || d == nil {
		return "未找到指定的决策"
	}
	text := fmt.Sprintf("## %s\n\n", d.Title)
	text += fmt.Sprintf("- **SDR ID**: %s\n", d.SDRID)
	text += fmt.Sprintf("- **议题**: %s\n", d.Topic)
	text += fmt.Sprintf("- **决策**: %s\n", d.Decision)
	text += fmt.Sprintf("- **依据**: %s\n", d.Rationale)
	return text
}

func formatExtractResult(result *llm.ExtractionResult) string {
	if !result.HasDecision {
		return "未检测到决策信息"
	}
	text := "## 决策提取结果\n\n"
	text += fmt.Sprintf("- **置信度**: %.2f\n", result.Confidence)
	if result.Decision != nil {
		text += fmt.Sprintf("- **标题**: %s\n", result.Decision.Title)
		text += fmt.Sprintf("- **决策**: %s\n", result.Decision.Decision)
		text += fmt.Sprintf("- **建议议题**: %s\n", result.Decision.SuggestedTopic)
	}
	return text
}

func formatClassifyResult(result *llm.ClassificationResult) string {
	text := "## 议题分类结果\n\n"
	text += fmt.Sprintf("- **建议议题**: %s\n", result.Topic)
	text += fmt.Sprintf("- **置信度**: %.2f\n", result.Confidence)
	text += fmt.Sprintf("- **说明**: %s\n", result.Reasoning)
	return text
}

func formatCrossTopicResult(result *llm.CrossTopicResult) string {
	text := "## 跨议题检测结果\n\n"
	if result.IsCrossTopic {
		text += "⚠️ **检测到跨议题影响**\n"
		text += fmt.Sprintf("- **受影响议题**: %v\n", result.CrossTopicRefs)
	} else {
		text += "✅ **无跨议题影响**\n"
	}
	text += fmt.Sprintf("\n- **置信度**: %.2f\n", result.Confidence)
	return text
}

func formatConflictResult(result *llm.ConflictResult) string {
	text := "## 冲突评估结果\n\n"
	text += fmt.Sprintf("- **冲突分数**: %.2f\n", result.ContradictionScore)
	text += fmt.Sprintf("- **冲突类型**: %s\n", result.ContradictionType)
	text += fmt.Sprintf("- **描述**: %s\n", result.Description)
	text += fmt.Sprintf("- **建议**: %s\n", result.Action)
	return text
}

func formatTimelineResults(items []TimelineItem) string {
	text := "## 决策时间线\n\n"
	for _, item := range items {
		text += fmt.Sprintf("- %s: %s (%s)\n",
			item.Timestamp.Format("2006-01-02 15:04"),
			item.Event,
			item.SDRID,
		)
	}
	return text
}

// validateToolArgs 校验工具必需参数
func (s *MCPServer) validateToolArgs(name string, args map[string]any) string {
	switch name {
	case "topic":
		if _, ok := args["topic"]; !ok {
			return "missing required parameter: topic"
		}
	case "decision":
		if _, ok := args["sdr_id"]; !ok {
			return "missing required parameter: sdr_id"
		}
	case "extract_decision":
		if _, ok := args["content"]; !ok {
			return "missing required parameter: content"
		}
	case "classify_topic":
		if _, ok := args["decision"]; !ok {
			return "missing required parameter: decision"
		}
		if _, ok := args["topics"]; !ok {
			return "missing required parameter: topics"
		}
	case "detect_crosstopic":
		if _, ok := args["title"]; !ok {
			return "missing required parameter: title"
		}
		if _, ok := args["decision"]; !ok {
			return "missing required parameter: decision"
		}
		if _, ok := args["candidate_topics"]; !ok {
			return "missing required parameter: candidate_topics"
		}
	case "check_conflict":
		if _, ok := args["decision_a"]; !ok {
			return "missing required parameter: decision_a"
		}
		if _, ok := args["decision_b"]; !ok {
			return "missing required parameter: decision_b"
		}
	}
	return ""
}

// NotifyConflictViaMCP 记录冲突通知
func NotifyConflictViaMCP(sdrID1, sdrID2, reason string) {
	fmt.Fprintf(os.Stderr, "[MCP-Notify] ⚠️ Conflict: %s vs %s - %s\n", sdrID1, sdrID2, reason)
}

// ===== 类型定义 =====

// MCP 标准错误码
const (
	ErrCodeParseError       = -32700
	ErrCodeInvalidRequest   = -32600
	ErrCodeMethodNotFound   = -32601
	ErrCodeInvalidParams    = -32602
	ErrCodeInternalError    = -32603
	ErrCodeToolNotFound     = -32003
	ErrCodeResourceNotFound = -32002
)

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Resource struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MimeType string `json:"mimeType"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type SearchResult struct {
	SDRID       string `json:"sdr_id"`
	Title       string `json:"title"`
	Topic       string `json:"topic"`
	ImpactLevel string `json:"impact_level"`
	Status      string `json:"status"`
}

type TimelineItem struct {
	Timestamp time.Time `json:"timestamp"`
	Event     string    `json:"event"`
	SDRID     string    `json:"sdr_id"`
}
