package llm

import (
	"context"
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

	// 检查 LLM 是否可用
	if !a.llmClient.IsAvailable() {
		log.Println("[Agent] LLM not available, returning fallback")
		log.Println("========== EXTRACT DECISION END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, fmt.Errorf("ARK_API_KEY is not set")
	}

	// 构建提示词
	log.Println("[Agent] Building extraction prompts...")
	systemPrompt, userPrompt, err := a.buildExtractionPrompts(content, topics)
	if err != nil {
		log.Printf("[Agent] Build prompts failed: %v", err)
		log.Println("========== EXTRACT DECISION END ==========")
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
		log.Println("========== EXTRACT DECISION END ==========")
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
		log.Println("========== EXTRACT DECISION END ==========")
		return &ExtractionResult{
			HasDecision:    false,
			Confidence:     0.0,
			ExtractedFrom: content,
		}, err
	}

	log.Printf("[Agent] Parse result: HasDecision=%v, Confidence=%.2f", result.HasDecision, result.Confidence)
	if result.Decision != nil {
		log.Printf("[Agent] Decision title: %s", result.Decision.Title)
		log.Printf("[Agent] Decision content: %s", truncateForLog(result.Decision.Decision, 200))
	}

	result.ExtractedFrom = content
	log.Println("========== EXTRACT DECISION END ==========")
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
