package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"feishu-mem/internal/llm/prompts"
)

// MemoryAgent 记忆系统专用 Agent
type MemoryAgent struct {
	promptMgr *prompts.PromptManager
	fallback  *Fallback
	llmClient *Client
}

// NewMemoryAgent 创建 MemoryAgent
func NewMemoryAgent() *MemoryAgent {
	return &MemoryAgent{
		promptMgr: prompts.NewPromptManager(),
		fallback:  NewFallback(),
		llmClient: NewClient(),
	}
}

// ========== 核心工作流 ==========

// ExtractDecision 提取决策
func (a *MemoryAgent) ExtractDecision(content string, topics []string) (*ExtractionResult, error) {
	log.Println("========== EXTRACT DECISION START ==========")
	log.Printf("[Agent] Content length: %d", len(content))
	log.Printf("[Agent] Topics: %v", topics)

	// LLM 不可用时使用降级策略
	if !a.llmClient.IsAvailable() {
		log.Println("[Agent] LLM not available, using keyword fallback")
		fallbackResult := a.fallback.ExtractDecision(content)
		fallbackResult.ExtractedFrom = content
		log.Printf("[Agent] Fallback result: HasDecision=%v, Confidence=%.2f",
			fallbackResult.HasDecision, fallbackResult.Confidence)
		log.Println("========== EXTRACT DECISION END ==========")
		return fallbackResult, nil
	}

	// 构建提示词
	log.Println("[Agent] Building extraction prompts...")
	systemPrompt, userPrompt, err := a.buildExtractionPrompts(content, topics)
	if err != nil {
		log.Printf("[Agent] Build prompts failed: %v", err)
		log.Println("========== EXTRACT DECISION END ==========")
		return &ExtractionResult{
			HasDecision:   false,
			Confidence:    0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] System prompt (first 500 chars): %s", truncateForLog(systemPrompt, 500))
	log.Printf("[Agent] User prompt (first 500 chars): %s", truncateForLog(userPrompt, 500))

	// 调用 LLM
	log.Println("[Agent] Calling LLM...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("[Agent] LLM call failed: %v", err)
		log.Println("========== EXTRACT DECISION END ==========")
		return &ExtractionResult{
			HasDecision:   false,
			Confidence:    0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Raw LLM response: %s", llmResponse)

	// 解析 LLM 响应
	log.Println("[Agent] Parsing LLM response...")
	result, err := ParseExtractionResult(llmResponse)
	if err != nil {
		log.Printf("[Agent] Parse failed: %v, using fallback", err)
		// 解析失败时回退到降级策略
		fallbackResult := a.fallback.ExtractDecision(content)
		fallbackResult.ExtractedFrom = content
		log.Println("========== EXTRACT DECISION END ==========")
		return fallbackResult, nil
	}

	log.Printf("[Agent] Parse result: HasDecision=%v, Confidence=%.2f", result.HasDecision, result.Confidence)
	if result.Decision != nil {
		log.Printf("[Agent] Decision title: %s", result.Decision.Title)
		log.Printf("[Agent] Decision content: %s", truncateForLog(result.Decision.Decision, 200))
	}
	if result.HasObjections {
		log.Printf("[Agent] Objections extracted: %d", len(result.Objections))
		for i, obj := range result.Objections {
			log.Printf("[Agent] Objection[%d]: %s (by %s)", i, obj.ObjectionContent, obj.Objector)
		}
	}

	result.ExtractedFrom = content
	log.Println("========== EXTRACT DECISION END ==========")
	return result, nil
}

// ========== 文档决策提取 ==========

// ExtractDecisionFromDoc 从文档中提取决策（使用分阶段分析 prompt）
func (a *MemoryAgent) ExtractDecisionFromDoc(content string, topics []string, docType string, title string) (*ExtractionResult, error) {
	log.Println("========== EXTRACT DECISION FROM DOC START ==========")
	log.Printf("[Agent] Doc content length: %d", len(content))
	log.Printf("[Agent] Doc type: %s, title: %s", docType, title)
	log.Printf("[Agent] Topics: %v", topics)

	if !a.llmClient.IsAvailable() {
		log.Println("[Agent] LLM not available, returning fallback")
		log.Println("========== EXTRACT DECISION FROM DOC END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, fmt.Errorf("ARK_API_KEY is not set")
	}

	log.Println("[Agent] Building doc extraction prompts...")
	systemPrompt, userPrompt, err := a.buildDocExtractionPrompts(content, topics, docType, title)
	if err != nil {
		log.Printf("[Agent] Build prompts failed: %v", err)
		log.Println("========== EXTRACT DECISION FROM DOC END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Doc system prompt (first 500 chars): %s", truncateForLog(systemPrompt, 500))
	log.Printf("[Agent] Doc user prompt (first 500 chars): %s", truncateForLog(userPrompt, 500))

	log.Println("[Agent] Calling LLM for doc extraction...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("[Agent] LLM call failed: %v", err)
		log.Println("========== EXTRACT DECISION FROM DOC END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Raw LLM response: %s", llmResponse)

	log.Println("[Agent] Parsing LLM response...")
	result, err := ParseExtractionResult(llmResponse)
	if err != nil {
		log.Printf("[Agent] Parse failed: %v, using fallback", err)
		log.Println("========== EXTRACT DECISION FROM DOC END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Doc parse result: HasDecision=%v, Confidence=%.2f, ChangeType=%v",
		result.HasDecision, result.Confidence, result.Decision)
	if result.Decision != nil {
		log.Printf("[Agent] Doc decision title: %s", result.Decision.Title)
		log.Printf("[Agent] Doc decision content: %s", truncateForLog(result.Decision.Decision, 200))
	}
	if result.HasObjections {
		log.Printf("[Agent] Doc objections extracted: %d", len(result.Objections))
		for i, obj := range result.Objections {
			log.Printf("[Agent] Doc Objection[%d]: %s (by %s)", i, obj.ObjectionContent, obj.Objector)
		}
	}

	result.ExtractedFrom = content
	log.Println("========== EXTRACT DECISION FROM DOC END ==========")
	return result, nil
}

// ExtractDecisionFromDocWithContext 从文档中提取决策（带相关决策上下文）
func (a *MemoryAgent) ExtractDecisionFromDocWithContext(
	content string, topics []string, docType string, title string, relatedDecisionSummaries []string,
) (*ExtractionResult, error) {
	log.Println("========== EXTRACT DECISION FROM DOC WITH CONTEXT START ==========")
	log.Printf("[Agent] Doc content length: %d", len(content))
	log.Printf("[Agent] Doc type: %s, title: %s", docType, title)
	log.Printf("[Agent] Topics: %v", topics)
	log.Printf("[Agent] Related decisions: %d", len(relatedDecisionSummaries))

	if !a.llmClient.IsAvailable() {
		log.Println("[Agent] LLM not available, returning fallback")
		log.Println("========== EXTRACT DECISION FROM DOC WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, fmt.Errorf("ARK_API_KEY is not set")
	}

	log.Println("[Agent] Building doc extraction prompts with context...")
	systemPrompt, userPrompt, err := a.buildDocExtractionPromptsWithContext(content, topics, docType, title, relatedDecisionSummaries)
	if err != nil {
		log.Printf("[Agent] Build prompts failed: %v", err)
		log.Println("========== EXTRACT DECISION FROM DOC WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Doc system prompt (first 500 chars): %s", truncateForLog(systemPrompt, 500))
	log.Printf("[Agent] Doc user prompt (first 500 chars): %s", truncateForLog(userPrompt, 500))

	log.Println("[Agent] Calling LLM for doc extraction...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("[Agent] LLM call failed: %v", err)
		log.Println("========== EXTRACT DECISION FROM DOC WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Raw LLM response: %s", llmResponse)

	log.Println("[Agent] Parsing LLM response...")
	result, err := ParseExtractionResult(llmResponse)
	if err != nil {
		log.Printf("[Agent] Parse failed: %v, using fallback", err)
		log.Println("========== EXTRACT DECISION FROM DOC WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Doc parse result: HasDecision=%v, Confidence=%.2f, ChangeType=%v",
		result.HasDecision, result.Confidence, result.Decision)
	if result.Decision != nil {
		log.Printf("[Agent] Doc decision title: %s", result.Decision.Title)
		log.Printf("[Agent] Doc decision content: %s", truncateForLog(result.Decision.Decision, 200))
	}
	if result.HasObjections {
		log.Printf("[Agent] Doc objections extracted: %d", len(result.Objections))
		for i, obj := range result.Objections {
			log.Printf("[Agent] Doc Objection[%d]: %s (by %s)", i, obj.ObjectionContent, obj.Objector)
		}
	}

	result.ExtractedFrom = content
	log.Println("========== EXTRACT DECISION FROM DOC WITH CONTEXT END ==========")
	return result, nil
}

// ExtractDecisionWithContext 从内容中提取决策（带相关决策上下文）
func (a *MemoryAgent) ExtractDecisionWithContext(
	content string, topics []string, relatedDecisionSummaries []string,
) (*ExtractionResult, error) {
	log.Println("========== EXTRACT DECISION WITH CONTEXT START ==========")
	log.Printf("[Agent] Content length: %d", len(content))
	log.Printf("[Agent] Topics: %v", topics)
	log.Printf("[Agent] Related decisions: %d", len(relatedDecisionSummaries))

	// 检查 LLM 是否可用
	if !a.llmClient.IsAvailable() {
		log.Println("[Agent] LLM not available, returning fallback")
		log.Println("========== EXTRACT DECISION WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, fmt.Errorf("ARK_API_KEY is not set")
	}

	// 构建提示词（带相关决策）
	log.Println("[Agent] Building extraction prompts with context...")
	systemPrompt, userPrompt, err := a.buildExtractionPromptsWithContext(
		content, topics, relatedDecisionSummaries)
	if err != nil {
		log.Printf("[Agent] Build prompts failed: %v", err)
		log.Println("========== EXTRACT DECISION WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] System prompt (first 500 chars): %s", truncateForLog(systemPrompt, 500))
	log.Printf("[Agent] User prompt (first 500 chars): %s", truncateForLog(userPrompt, 500))

	// 调用 LLM
	log.Println("[Agent] Calling LLM...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("[Agent] LLM call failed: %v", err)
		log.Println("========== EXTRACT DECISION WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Raw LLM response: %s", llmResponse)

	// 解析 LLM 响应
	log.Println("[Agent] Parsing LLM response...")
	result, err := ParseExtractionResult(llmResponse)
	if err != nil {
		log.Printf("[Agent] Parse failed: %v, using fallback", err)
		log.Println("========== EXTRACT DECISION WITH CONTEXT END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Parse result: HasDecision=%v, Confidence=%.2f",
		result.HasDecision, result.Confidence)
	if result.Decision != nil {
		log.Printf("[Agent] Decision title: %s", result.Decision.Title)
		log.Printf("[Agent] Decision content: %s", truncateForLog(result.Decision.Decision, 200))
	}
	if result.HasObjections {
		log.Printf("[Agent] Objections extracted: %d", len(result.Objections))
		for i, obj := range result.Objections {
			log.Printf("[Agent] Objection[%d]: %s (by %s)", i, obj.ObjectionContent, obj.Objector)
		}
	}

	result.ExtractedFrom = content
	log.Println("========== EXTRACT DECISION WITH CONTEXT END ==========")
	return result, nil
}

// ClassifyTopic 分类议题
func (a *MemoryAgent) ClassifyTopic(decision string, topics []string) (*ClassificationResult, error) {
	log.Printf("[Agent] ClassifyTopic called: decision=%s, topics=%v", truncateForLog(decision, 100), topics)

	quickResult := a.fallback.ClassifyTopic(decision, topics)
	if quickResult.Topic != "" {
		log.Printf("[Agent] Fallback result: %s", quickResult.Topic)
		return quickResult, nil
	}

	if !a.llmClient.IsAvailable() {
		log.Println("[Agent] LLM not available, returning fallback")
		return quickResult, nil
	}

	systemPrompt, userPrompt, err := a.buildClassificationPrompts(decision, topics)
	if err != nil {
		log.Printf("[Agent] Build classification prompts failed: %v", err)
		return quickResult, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("[Agent] LLM call failed: %v", err)
		return quickResult, nil
	}

	result, err := ParseClassificationResult(llmResponse)
	if err != nil {
		log.Printf("[Agent] Parse classification failed: %v", err)
		return quickResult, nil
	}

	log.Printf("[Agent] Classification result: %s", result.Topic)
	return result, nil
}

// DetectCrossTopic 检测跨议题
func (a *MemoryAgent) DetectCrossTopic(node any) (*CrossTopicResult, error) {
	quickResult := a.fallback.DetectCrossTopic(node)

	if !a.llmClient.IsAvailable() {
		return quickResult, nil
	}

	systemPrompt, userPrompt, err := a.buildCrossTopicPrompts(node)
	if err != nil {
		return quickResult, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		return quickResult, nil
	}

	result, err := ParseCrossTopicResult(llmResponse)
	if err != nil {
		return quickResult, nil
	}

	return result, nil
}

// ResolveConflict 解决冲突
func (a *MemoryAgent) ResolveConflict(nodeA, nodeB any) (*ConflictResult, error) {
	result := &ConflictResult{
		ContradictionScore: 0.0,
		ContradictionType:  "none",
		Description:        "",
		Action:             "no_conflict",
		NeedsUser:          false,
	}

	if !a.llmClient.IsAvailable() {
		return result, nil
	}

	systemPrompt, userPrompt, err := a.buildConflictPrompts(nodeA, nodeB)
	if err != nil {
		return result, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		return result, nil
	}

	llmResult, err := ParseConflictResult(llmResponse)
	if err != nil {
		return result, nil
	}

	return llmResult, nil
}

// IsAvailable 检查 LLM 是否可用
func (a *MemoryAgent) IsAvailable() bool {
	return a.llmClient.IsAvailable()
}

// GetTools 获取工具列表 (用于测试兼容)
func (a *MemoryAgent) GetTools() []any {
	return []any{}
}

// SearchTools 搜索工具 (用于测试兼容)
func (a *MemoryAgent) SearchTools(query string) []any {
	return []any{}
}

// ========== 内部方法 ==========

func (a *MemoryAgent) buildExtractionPrompts(content string, topics []string) (string, string, error) {
	systemPrompt := prompts.ExtractionStaticPrompt

	userPrompt, err := a.promptMgr.BuildPrompt("extraction", map[string]any{
		"content": content,
		"topics":  topics,
	})
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}

func (a *MemoryAgent) buildExtractionPromptsWithContext(
	content string, topics []string, relatedDecisionSummaries []string,
) (string, string, error) {
	systemPrompt := prompts.ExtractionStaticPrompt

	userPrompt, err := a.promptMgr.BuildPrompt("extraction", map[string]any{
		"content":           content,
		"topics":            topics,
		"related_decisions": relatedDecisionSummaries,
	})
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}

func (a *MemoryAgent) buildDocExtractionPrompts(content string, topics []string, docType string, title string) (string, string, error) {
	systemPrompt := prompts.ExtractionDocStaticPrompt

	userPrompt, err := a.promptMgr.BuildPrompt("extraction_doc", map[string]any{
		"content":  content,
		"topics":   topics,
		"doc_type": docType,
		"title":    title,
	})
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}

func (a *MemoryAgent) buildDocExtractionPromptsWithContext(
	content string, topics []string, docType string, title string, relatedDecisionSummaries []string,
) (string, string, error) {
	systemPrompt := prompts.ExtractionDocStaticPrompt

	userPrompt, err := a.promptMgr.BuildPrompt("extraction_doc", map[string]any{
		"content":           content,
		"topics":            topics,
		"doc_type":          docType,
		"title":             title,
		"related_decisions": relatedDecisionSummaries,
	})
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}

func (a *MemoryAgent) buildClassificationPrompts(decision string, topics []string) (string, string, error) {
	systemPrompt := prompts.ClassificationStaticPrompt

	userPrompt, err := a.promptMgr.BuildPrompt("classification", map[string]any{
		"decision": decision,
		"topics":   topics,
	})
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}

func (a *MemoryAgent) buildCrossTopicPrompts(node any) (string, string, error) {
	systemPrompt := prompts.CrossTopicStaticPrompt

	data := make(map[string]any)
	if mapNode, ok := node.(map[string]any); ok {
		for k, v := range mapNode {
			data[k] = v
		}
	}

	userPrompt, err := a.promptMgr.BuildPrompt("crosstopic", data)
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}

func (a *MemoryAgent) buildConflictPrompts(nodeA, nodeB any) (string, string, error) {
	systemPrompt := prompts.ConflictStaticPrompt

	decisionA := fmt.Sprintf("%v", nodeA)
	decisionB := fmt.Sprintf("%v", nodeB)

	userPrompt, err := a.promptMgr.BuildPrompt("conflict", map[string]any{
		"decisionA": decisionA,
		"decisionB": decisionB,
	})
	if err != nil {
		return "", "", err
	}

	return systemPrompt, userPrompt, nil
}

// DedupResult LLM 去重+冲突联合判断结果
type DedupResult struct {
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// EvaluateDedupAction 使用 LLM + JSON Schema 同时判断去重和冲突
func (a *MemoryAgent) EvaluateDedupAction(newTitle, newDecision, existingTitle, existingDecision string) (*DedupResult, error) {
	log.Println("========== EVALUATE DEDUP ACTION START ==========")
	log.Printf("[Agent] New: title=%s, decision=%s", newTitle, truncateForLog(newDecision, 100))
	log.Printf("[Agent] Old: title=%s, decision=%s", existingTitle, truncateForLog(existingDecision, 100))

	if !a.llmClient.IsAvailable() {
		log.Println("[Agent] LLM not available for dedup")
		return nil, fmt.Errorf("ARK_API_KEY is not set")
	}

	systemPrompt := prompts.DedupStaticPrompt
	userPrompt, err := a.promptMgr.BuildPrompt("dedup", map[string]any{
		"new_title":         newTitle,
		"new_decision":      newDecision,
		"existing_title":    existingTitle,
		"existing_decision": existingDecision,
	})
	if err != nil {
		log.Printf("[Agent] Build dedup prompt failed: %v", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.CallWithJSONSchema(ctx, systemPrompt, userPrompt,
		GenerateSchema[DedupActionType](), "dedup_action", "去重动作：skip/update/conflict")
	if err != nil {
		log.Printf("[Agent] LLM dedup call failed: %v", err)
		// 降级：使用普通 Call + parse
		return a.EvaluateDedupActionFallback(systemPrompt, userPrompt)
	}

	var result DedupActionType
	if err := json.Unmarshal([]byte(llmResponse), &result); err != nil {
		log.Printf("[Agent] Unmarshal dedup failed: %v, falling back", err)
		return a.EvaluateDedupActionFallback(systemPrompt, userPrompt)
	}

	log.Printf("[Agent] Dedup result: action=%s, reason=%s", result.Action, result.Reason)
	log.Println("========== EVALUATE DEDUP ACTION END ==========")
	return &DedupResult{Action: result.Action, Reason: result.Reason}, nil
}

// EvaluateDedupActionFallback JSON Schema 失败时降级为普通 Call + Parse
func (a *MemoryAgent) EvaluateDedupActionFallback(systemPrompt, userPrompt string) (*DedupResult, error) {
	log.Println("[Agent] Dedup fallback: using plain text response")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	result, err := ParseDedupResult(llmResponse)
	if err != nil {
		return &DedupResult{Action: "update", Reason: "fallback"}, nil
	}
	return result, nil
}

// ResolveConflictAction 使用 LLM + JSON Schema 判断冲突能否自动合并
func (a *MemoryAgent) ResolveConflictAction(newTitle, newDecision, existingTitle, existingDecision string) (*ConflictResolveResult, error) {
	log.Println("========== RESOLVE CONFLICT ACTION START ==========")
	log.Printf("[Agent] New: title=%s, decision=%s", newTitle, truncateForLog(newDecision, 100))
	log.Printf("[Agent] Old: title=%s, decision=%s", existingTitle, truncateForLog(existingDecision, 100))

	if !a.llmClient.IsAvailable() {
		return nil, fmt.Errorf("ARK_API_KEY is not set")
	}

	systemPrompt := prompts.ConflictResolveStaticPrompt
	userPrompt, err := a.promptMgr.BuildPrompt("conflict_resolve", map[string]any{
		"new_title":         newTitle,
		"new_decision":      newDecision,
		"existing_title":    existingTitle,
		"existing_decision": existingDecision,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	llmResponse, err := a.llmClient.CallWithJSONSchema(ctx, systemPrompt, userPrompt,
		GenerateSchema[ConflictResolveResult](), "conflict_resolve", "冲突解决动作：merge 或 keep_both")
	if err != nil {
		log.Printf("[Agent] Conflict resolve call failed: %v, defaulting to keep_both", err)
		return &ConflictResolveResult{Action: "keep_both", Reason: "LLM unavailable"}, nil
	}

	var result ConflictResolveResult
	if err := json.Unmarshal([]byte(llmResponse), &result); err != nil {
		log.Printf("[Agent] Unmarshal conflict resolve failed: %v", err)
		return &ConflictResolveResult{Action: "keep_both", Reason: "parse error"}, nil
	}

	switch result.Action {
	case "merge", "keep_both":
	default:
		result.Action = "keep_both"
		result.Reason = "LLM returned unknown action, defaulting to keep_both"
	}

	log.Printf("[Agent] Conflict resolve: action=%s, reason=%s", result.Action, result.Reason)
	log.Println("========== RESOLVE CONFLICT ACTION END ==========")
	return &result, nil
}
