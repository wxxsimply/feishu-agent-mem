package main

import (
	"log"
	"time"

	"feishu-mem/internal/card"
	"feishu-mem/internal/decision"
	"feishu-mem/internal/larkadapter"
	"feishu-mem/internal/recall"
)

func main() {
	larkadapter.LoadEnv()

	// 1. 测试纯文本消息
	log.Println("测试1: 发送纯文本消息")
	cli := larkadapter.NewLarkCLI()
	chatID := larkadapter.LoadConfig().ChatIDs[0]

	output, err := cli.RunCommand("im", "+messages-send",
		"--chat-id", chatID,
		"--text", "测试卡片推送修复 - 这是一条纯文本消息",
		"--as", "bot",
	)
	if err != nil {
		log.Printf("发送失败: %v, 输出: %s", err, output)
	} else {
		log.Printf("发送成功: %s", output)
	}

	// 2. 创建测试决策
	log.Println("\n测试2: 渲染并发送决策卡片")
	node := decision.NewDecisionNode("test-001", "测试决策", "yatsenos", "lab0")
	node.Decision = "这是一个测试决策，验证卡片渲染功能是否正常工作。"
	node.Rationale = "验证卡片渲染和飞书推送流程"
	node.ImpactLevel = decision.ImpactMajor
	node.Status = decision.StatusDecided

	renderer := card.NewRenderer()
	hotScore := 85.0

	cardJSON, err := renderer.RenderLarkCardFromNode(node, hotScore)
	if err != nil {
		log.Fatalf("渲染卡片失败: %v", err)
	}
	log.Printf("卡片内容: %s", cardJSON)

	// 发送交互式卡片
	output, err = cli.RunCommand("im", "+messages-send",
		"--chat-id", chatID,
		"--msg-type", "interactive",
		"--content", cardJSON,
		"--as", "bot",
	)
	if err != nil {
		log.Printf("发送卡片失败: %v, 输出: %s", err, output)
	} else {
		log.Printf("发送卡片成功: %s", output)
	}

	// 3. 测试每日摘要卡片
	log.Println("\n测试3: 渲染并发送每日摘要")
	testCard := &recall.DecisionCard{
		Decision:    node,
		HotScore:    hotScore,
		RecallType: recall.RecallExact,
	}

	summaryJSON, err := renderer.RenderDailySummary(time.Now(), []*recall.DecisionCard{testCard}, []*recall.DecisionCard{})
	if err != nil {
		log.Fatalf("渲染每日摘要失败: %v", err)
	}
	log.Printf("每日摘要内容: %s", summaryJSON)

	output, err = cli.RunCommand("im", "+messages-send",
		"--chat-id", chatID,
		"--msg-type", "interactive",
		"--content", summaryJSON,
		"--as", "bot",
	)
	if err != nil {
		log.Printf("发送每日摘要失败: %v, 输出: %s", err, output)
	} else {
		log.Printf("发送每日摘要成功: %s", output)
	}
}
