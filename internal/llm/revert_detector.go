package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/llm/prompts"
)

// ========== 回溯检测类型定义 ==========

// RevertDecision LLM 回溯判断结果
type RevertDecision struct {
	ShouldRevert bool    `json:"should_revert"`
	TargetCommit string  `json:"target_commit,omitempty"`
	Reason       string  `json:"reason"`
	Confidence   float64 `json:"confidence"`
}

// RevertContext 回溯判断的上下文
type RevertContext struct {
	NewDecision       *RevertDecisionInfo
	ExistingDecisions []*RevertDecisionInfo
	DocDiff           string
	DocTitle          string
}

// RevertDecisionInfo 决策信息（用于回溯检测）
type RevertDecisionInfo struct {
	ID         string
	Title      string
	Decision   string
	Rationale  string
	Status     decision.DecisionStatus
	CommitHash string
	CreateTime time.Time
}

// ========== 回溯检测 Prompt ==========

var revertDetectionStaticPrompt = `# 系统提示词：决策回溯检测器

## 角色
你是一个项目决策版本管理专家。你需要分析文档变更，判断新提取的决策是否应该回溯到历史版本。

## 什么是决策回溯？
决策回溯是指：当前文档中的新决策实际上是对某个历史决策的恢复/撤销。

典型场景：
1. **撤销近期变更**：文档编辑者发现之前的修改有问题，恢复到更早的版本
   - 例：上次把"采用MySQL"改成了"采用TiDB"，现在又改回"采用MySQL"
   - 这种情况应该回溯到"采用MySQL"的版本

2. **纠正错误决策**：新决策与历史某个决策完全一致，说明是在纠正
   - 例：历史版本是"使用PostgreSQL"，后来改成了"使用MySQL"，现在文档又说"使用PostgreSQL"
   - 这种情况应该回溯到最初的"使用PostgreSQL"版本

3. **版本回退**：文档中出现了明显的版本回退信号
   - 例：使用了"恢复到"、"改回"、"回退到"、"撤销"等词语
   - 例：diff 中显示删除了新内容，恢复了旧内容

## 判断标准

### 必须回溯的情况（should_revert: true）
- 新决策与历史某个决策完全相同或高度相似（相似度 > 90%）
- 文档中出现明确的回退/恢复词语（"恢复"、"改回"、"回退"、"撤销"）
- diff 显示删除了近期新增内容，恢复了旧内容
- 新决策是历史决策的精确引用

### 不需要回溯的情况（should_revert: false）
- 新决策是全新决策，与历史无重复
- 新决策是对历史决策的补充或细化（不是完全相同）
- 新决策是历史决策的演进版本（有新增信息）

## 输入格式

你会收到：
1. **新提取的决策**：当前从文档中提取的决策信息
2. **历史决策列表**：该项目已有的决策，按时间倒序排列
3. **文档变更内容**：文档的 diff 或变更摘要

## 输出格式（严格遵守）

{
  "should_revert": true/false,
  "target_commit": "如果需要回溯，指定目标版本的 commit hash（从历史决策中选择）",
  "reason": "一句话说明判断理由",
  "confidence": 0.0-1.0
}

## 规则
- 只有明确是回退/恢复操作时才 should_revert: true
- 如果不确定，should_revert: false，宁可多创建一个决策也不要误回溯
- target_commit 必须是历史决策中存在的 commit hash
- confidence < 0.6 时不建议回溯
`

// revertDetectionDynamicBuilder 动态构建回溯检测 prompt
func revertDetectionDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder

	// 新提取的决策
	if newDecision, ok := ctx["new_decision"].(*RevertDecisionInfo); ok {
		sb.WriteString("\n## 新提取的决策\n")
		sb.WriteString(fmt.Sprintf("- 标题: %s\n", newDecision.Title))
		sb.WriteString(fmt.Sprintf("- 决策: %s\n", newDecision.Decision))
		sb.WriteString(fmt.Sprintf("- 依据: %s\n", newDecision.Rationale))
		sb.WriteString(fmt.Sprintf("- 时间: %s\n", newDecision.CreateTime.Format("2006-01-02 15:04:05")))
	}

	// 历史决策列表
	if existingDecisions, ok := ctx["existing_decisions"].([]*RevertDecisionInfo); ok && len(existingDecisions) > 0 {
		sb.WriteString("\n## 历史决策列表（按时间倒序）\n")
		for i, d := range existingDecisions {
			commitHash := d.CommitHash
			if len(commitHash) > 7 {
				commitHash = commitHash[:7]
			}
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, commitHash, d.Title))
			sb.WriteString(fmt.Sprintf("   决策: %s\n", d.Decision))
			sb.WriteString(fmt.Sprintf("   状态: %s, 时间: %s\n", d.Status, d.CreateTime.Format("2006-01-02 15:04:05")))
		}
	}

	// 文档变更内容
	if docDiff, ok := ctx["doc_diff"].(string); ok && docDiff != "" {
		sb.WriteString(fmt.Sprintf("\n## 文档变更内容（diff）\n%s\n", docDiff))
	}

	// 文档标题
	if docTitle, ok := ctx["doc_title"].(string); ok && docTitle != "" {
		sb.WriteString(fmt.Sprintf("\n## 文档标题\n%s\n", docTitle))
	}

	return sb.String()
}

// ========== 回溯检测器 ==========

// RevertDetector 回溯检测器
type RevertDetector struct {
	llmClient *Client
}

// NewRevertDetector 创建回溯检测器
func NewRevertDetector() *RevertDetector {
	return &RevertDetector{
		llmClient: NewClient(),
	}
}

// DetectRevert 检测是否需要回溯
func (d *RevertDetector) DetectRevert(ctx *RevertContext) (*RevertDecision, error) {
	log.Println("========== REVERT DETECTION START ==========")
	log.Printf("[RevertDetector] New decision: %s", ctx.NewDecision.Title)
	log.Printf("[RevertDetector] Existing decisions: %d", len(ctx.ExistingDecisions))
	log.Printf("[RevertDetector] Doc diff length: %d", len(ctx.DocDiff))

	// 检查 LLM 是否可用
	if !d.llmClient.IsAvailable() {
		log.Println("[RevertDetector] LLM not available, skipping revert detection")
		log.Println("========== REVERT DETECTION END ==========")
		return &RevertDecision{
			ShouldRevert: false,
			Confidence:   0,
			Reason:       "LLM not available",
		}, nil
	}

	// 构建 prompt
	pm := prompts.NewPromptManager()
	pm.RegisterTemplate(&prompts.PromptTemplate{
		Name:        "revert_detection",
		Static:      revertDetectionStaticPrompt,
		Dynamic:     revertDetectionDynamicBuilder,
		MaxTokens:   2000,
		Temperature: 0.1,
	})

	dynamicData := map[string]any{
		"new_decision":       ctx.NewDecision,
		"existing_decisions": ctx.ExistingDecisions,
		"doc_diff":           ctx.DocDiff,
		"doc_title":          ctx.DocTitle,
	}

	prompt, err := pm.BuildPrompt("revert_detection", dynamicData)
	if err != nil {
		log.Printf("[RevertDetector] Build prompt failed: %v", err)
		log.Println("========== REVERT DETECTION END ==========")
		return nil, fmt.Errorf("build prompt failed: %w", err)
	}

	log.Printf("[RevertDetector] Prompt length: %d chars", len(prompt))

	// 调用 LLM
	log.Println("[RevertDetector] Calling LLM...")
	callCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := d.llmClient.Call(callCtx, prompt, "")
	if err != nil {
		log.Printf("[RevertDetector] LLM call failed: %v", err)
		log.Println("========== REVERT DETECTION END ==========")
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	log.Printf("[RevertDetector] LLM response length: %d chars", len(response))

	// 解析结果
	revertDecision := &RevertDecision{
		ShouldRevert: false,
		Confidence:   0,
	}

	if err := json.Unmarshal([]byte(response), revertDecision); err != nil {
		// 尝试提取 JSON 部分
		start := strings.Index(response, "{")
		end := strings.LastIndex(response, "}")
		if start >= 0 && end > start {
			jsonStr := response[start : end+1]
			if err := json.Unmarshal([]byte(jsonStr), revertDecision); err != nil {
				log.Printf("[RevertDetector] Parse response failed: %v", err)
				log.Println("========== REVERT DETECTION END ==========")
				return nil, fmt.Errorf("parse response failed: %w", err)
			}
		} else {
			log.Printf("[RevertDetector] No valid JSON found in response")
			log.Println("========== REVERT DETECTION END ==========")
			return nil, fmt.Errorf("no valid JSON found in response")
		}
	}

	log.Printf("[RevertDetector] Result: ShouldRevert=%v, Confidence=%.2f", revertDecision.ShouldRevert, revertDecision.Confidence)
	log.Printf("[RevertDetector] Reason: %s", revertDecision.Reason)
	if revertDecision.ShouldRevert && revertDecision.TargetCommit != "" {
		log.Printf("[RevertDetector] Target commit: %s", revertDecision.TargetCommit)
	}

	log.Println("========== REVERT DETECTION END ==========")
	return revertDecision, nil
}
