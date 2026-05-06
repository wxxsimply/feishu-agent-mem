package main

import (
	"encoding/json"
	"log"
	"strings"

	"feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/llm"
	"feishu-mem/internal/llm/prompts"
)

func main() {
	larkadapter.LoadEnv()

	// 完整文档内容用于测试
	testMarkdown := `# 飞书决策信息提取方案

基于 lark-cli 从飞书群聊、日程、云文档、知识库、妙记/会议中自动识别和提取决策性信息，建立决策议题-负责人-关联文档-关联群聊-关联日程的完整关联图谱。

## 测试决策更新 — 明确决策1

经过团队讨论和性能比较测试，决定使用 deepseek-v3 替代 doubao 作为主力 LLM！

决策依据：
- deepseek-v3 在结构化提取任务上准确率高 12%
- 响应速度快 30%
- 成本更低

生效时间：立即
负责人：莫文豪

## 测试决策更新 — 明确决策2

决定：测试结束后恢复 10 秒检测间隔和 30 秒防抖窗口。

决策依据：
- 测试期间 5 秒间隔导致 API 调用过多
- 10 秒间隔适合生产环境
- 30 秒防抖窗口能平衡响应速度和去重效果

生效时间：测试结束后

## 测试决策更新 — 明确决策3

决定采用 MySQL 8.0 作为持久化存储，替代纯 Git 方案。

决策理由：
- Git 不支持高效的条件查询和索引
- MySQL 支持复杂的关联查询
- 数据量增大后，Git 的读取性能会下降明显
- Git 依然保留用于变更历史记录，但主存储采用 MySQL

生效时间：下一版发布（2026-05-07）
负责人：张雨婷

## 测试决策更新 — 不明显决策1

为规范版本管理和清晰追踪变更记录，我们统一 Git 提交格式为 "Decision: [SDRID] - [Title]"，替代原有的自由格式 "Update decision..."。

这将使变更历史更易于通过 grep 快速定位特定决策，提升项目可维护性。

## 测试决策更新 — 不明显决策2

为统一超时处理策略，所有飞书 API 调用统一使用 30 秒超时，不再根据不同接口进行区分设置，以简化配置和降低维护复杂度。
`

	log.Println("=== Testing LLM Extraction ===")

	// 测试提取文档决策
	ctx := &prompts.ExtractionPromptContext{
		DocType:          "doc",
		Title:            "测试决策文档",
		Content:          testMarkdown,
		RelatedDecisions: []llm.ExtractedDecisionRef{},
		AvailableTopics:  []string{"general", "测试"},
	}

	prompt, err := prompts.BuildExtractionDocPrompt(ctx)
	if err != nil {
		log.Fatalf("Build prompt failed: %v", err)
	}

	llmClient, _ := llm.NewLLMClient()
	response, err := llmClient.Call(llm.ModelRequest{
		Prompt:      prompt,
		MaxTokens:   4000,
		Temperature: 0.3,
	})
	if err != nil {
		log.Fatalf("LLM Call failed: %v", err)
	}

	log.Printf("=== LLM Response ===\n%s\n\n", response.Completion)

	// 解析响应
	result, err := llm.ParseExtractionResult(response.Completion)
	if err != nil {
		log.Fatalf("Parse failed: %v", err)
	}

	log.Printf("=== Parse Result ===")
	log.Printf("HasDecision: %v", result.HasDecision)
	log.Printf("Confidence: %.2f", result.Confidence)
	log.Printf("ChangeType: %v", result.ChangeType)

	// 处理单决策向后兼容
	if result.Decision != nil && len(result.Decisions) == 0 {
		result.Decisions = []llm.DecisionExtract{*result.Decision}
	}

	log.Printf("Decisions extracted: %d", len(result.Decisions))
	for i, dec := range result.Decisions {
		log.Printf("\n  [%d] %s", i+1, dec.Title)
		log.Printf("  Impact: %v", dec.ImpactLevel)
		log.Printf("  Phase: %v", dec.ProjectPhase)
		log.Printf("  Decision: %s", summarize(dec.Decision, 100))
		log.Printf("  Rationale: %s", summarize(dec.Rationale, 80))
	}

	log.Printf("\n=== Done ===")
}

func summarize(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
