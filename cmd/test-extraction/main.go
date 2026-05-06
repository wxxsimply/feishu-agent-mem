package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"feishu-mem/internal/llm"
	"feishu-mem/internal/llm/prompts"
	"feishu-mem/internal/signal"
)

func main() {
	inputFile := flag.String("f", "", "输入的 markdown 文件路径")
	docType := flag.String("type", "unknown", "文档类型 (design_doc/weekly_report/meeting_notes/spec/decision_log/administrative/unknown)")
	title := flag.String("title", "", "文档标题（不指定则从文件名推断）")
	rawMode := flag.Bool("raw", false, "输出原始 LLM 响应（不解析 JSON）")
	flag.Parse()

	if *inputFile == "" {
		fmt.Println("用法: test-extraction -f <file.md> [-type <doc_type>] [-title <title>] [-raw]")
		fmt.Println()
		fmt.Println("示例:")
		fmt.Println("  test-extraction -f test_doc.md -type design_doc -title \"后端架构方案\"")
		fmt.Println("  test-extraction -f test_doc.md -raw  # 输出原始 LLM 响应")
		os.Exit(1)
	}

	// 读取文件
	content, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("读取文件失败: %v", err)
	}

	// 推断标题
	if *title == "" {
		base := strings.TrimSuffix(*inputFile, ".md")
		parts := strings.Split(base, "/")
		*title = parts[len(parts)-1]
	}

	fmt.Println("========================================")
	fmt.Println("  LLM 决策提取测试")
	fmt.Println("========================================")
	fmt.Printf("文件: %s\n", *inputFile)
	fmt.Printf("标题: %s\n", *title)
	fmt.Printf("类型: %s\n", *docType)
	fmt.Printf("内容长度: %d 字符\n", len(content))
	fmt.Println("========================================")
	fmt.Println()

	// 检查 LLM 配置
	client := llm.NewClient()
	if !client.IsAvailable() {
		log.Fatal("LLM 不可用: 请设置 ARK_API_KEY 环境变量")
	}

	// 构建 prompt
	pm := prompts.NewPromptManager()
	topics := []string{"general", "system", "backend", "frontend", "devops", "architecture", "security", "testing"}

	systemPrompt := prompts.ExtractionDocStaticPrompt
	userPrompt, err := pm.BuildPrompt("extraction_doc", map[string]any{
		"content":  string(content),
		"topics":   topics,
		"doc_type": *docType,
		"title":    *title,
	})
	if err != nil {
		log.Fatalf("构建 prompt 失败: %v", err)
	}

	fmt.Printf("System prompt 长度: %d 字符\n", len(systemPrompt))
	fmt.Printf("User prompt 长度: %d 字符\n", len(userPrompt))
	fmt.Println()

	// 调用 LLM
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("正在调用 LLM...")
	startTime := time.Now()

	rawResponse, err := client.Call(ctx, systemPrompt, userPrompt)
	if err != nil {
		log.Fatalf("LLM 调用失败: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("LLM 响应耗时: %v\n", elapsed)
	fmt.Printf("原始响应长度: %d 字符\n", len(rawResponse))
	fmt.Println()

	if *rawMode {
		fmt.Println("=== 原始 LLM 响应 ===")
		fmt.Println(rawResponse)
		return
	}

	// 解析结果
	result, err := llm.ParseExtractionResult(rawResponse)
	if err != nil {
		fmt.Printf("解析失败: %v\n", err)
		fmt.Println()
		fmt.Println("=== 原始 LLM 响应 ===")
		fmt.Println(rawResponse)
		return
	}

	// 输出结果
	printResult(result)

	// 筛选测试
	decisions := result.Decisions
	if len(decisions) == 0 && result.Decision != nil {
		decisions = []llm.DecisionExtract{*result.Decision}
	}
	if len(decisions) > 1 {
		fmt.Println()
		fmt.Println("========================================")
		fmt.Println("  筛选与合并测试")
		fmt.Println("========================================")
		fmt.Printf("筛选前: %d 条决策\n", len(decisions))
		for i, d := range decisions {
			fmt.Printf("  [%d] %s (impact=%s, phase=%s)\n", i+1, d.Title, d.ImpactLevel, d.ProjectPhase)
		}

		filtered := signal.FilterAndMergeDecisions(decisions)
		fmt.Printf("\n筛选后: %d 条决策\n", len(filtered))
		for i, d := range filtered {
			fmt.Printf("  [%d] %s (impact=%s, phase=%s)\n", i+1, d.Title, d.ImpactLevel, d.ProjectPhase)
		}
	}

	// 同时输出 JSON 格式
	fmt.Println()
	fmt.Println("=== JSON 输出 ===")
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonBytes))
}

func printResult(result *llm.ExtractionResult) {
	fmt.Println("========================================")
	fmt.Println("  提取结果")
	fmt.Println("========================================")

	if !result.HasDecision {
		fmt.Println("结论: 未检测到决策")
		fmt.Printf("置信度: %.2f\n", result.Confidence)
		if result.ChangeType != "" {
			fmt.Printf("变更类型: %s\n", result.ChangeType)
		}
		if result.Analysis != "" {
			fmt.Printf("分析: %s\n", result.Analysis)
		}
		return
	}

	fmt.Printf("结论: 检测到决策!\n")
	fmt.Printf("置信度: %.2f\n", result.Confidence)
	if result.ChangeType != "" {
		fmt.Printf("变更类型: %s\n", result.ChangeType)
	}
	if result.Analysis != "" {
		fmt.Printf("分析: %s\n", result.Analysis)
	}
	fmt.Println()

	// 获取决策列表（兼容新旧格式）
	decisions := result.Decisions
	if len(decisions) == 0 && result.Decision != nil {
		decisions = []llm.DecisionExtract{*result.Decision}
	}

	fmt.Printf("=== 共提取 %d 条决策 ===\n\n", len(decisions))
	for i, d := range decisions {
		fmt.Printf("--- 决策 [%d] ---\n", i+1)
		fmt.Printf("标题: %s\n", d.Title)
		fmt.Printf("决策: %s\n", d.Decision)
		fmt.Printf("依据: %s\n", d.Rationale)
		fmt.Printf("建议议题: %s\n", d.SuggestedTopic)
		fmt.Printf("影响级别: %s\n", d.ImpactLevel)
		fmt.Printf("决策类型: %s\n", d.DecisionType)
		if d.Proposer != "" {
			fmt.Printf("提出人: %s\n", d.Proposer)
		}
		if d.Executor != "" {
			fmt.Printf("执行者: %s\n", d.Executor)
		}
		// 时间信息
		if d.DecisionTime != "" {
			fmt.Printf("决策时间: %s\n", d.DecisionTime)
		}
		if d.EffectiveTime != "" {
			fmt.Printf("生效时间: %s\n", d.EffectiveTime)
		}
		if d.Deadline != "" {
			fmt.Printf("截止时间: %s\n", d.Deadline)
		}
		if d.ProjectPhase != "" {
			fmt.Printf("项目阶段: %s\n", d.ProjectPhase)
		}
		fmt.Println()
	}

	if result.HasObjections && len(result.Objections) > 0 {
		fmt.Println()
		fmt.Printf("--- 反对意见 (%d 条) ---\n", len(result.Objections))
		for i, obj := range result.Objections {
			fmt.Printf("  [%d] 内容: %s\n", i+1, obj.ObjectionContent)
			if obj.Rationale != "" {
				fmt.Printf("      理由: %s\n", obj.Rationale)
			}
			if obj.Alternative != "" {
				fmt.Printf("      替代方案: %s\n", obj.Alternative)
			}
			if obj.Objector != "" {
				fmt.Printf("      反对人: %s\n", obj.Objector)
			}
		}
	}

	if result.HasDeletions && len(result.Deletions) > 0 {
		fmt.Println()
		fmt.Printf("--- 被删除/废弃的决策 (%d 条) ---\n", len(result.Deletions))
		for i, del := range result.Deletions {
			fmt.Printf("  [%d] 原决策: %s\n", i+1, del.OriginalDecision)
			fmt.Printf("      动作: %s\n", del.Action)
			if del.ReplacedBy != "" {
				fmt.Printf("      被取代为: %s\n", del.ReplacedBy)
			}
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
