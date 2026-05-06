package main

import (
	"log"
	"time"

	"feishu-mem/internal/core"
	"feishu-mem/internal/decision"
	"feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/push"
)

func main() {
	larkadapter.LoadEnv()

	// 1. 加载配置
	larkCfg := larkadapter.LoadConfig()
	log.Printf("测试推送，ChatIDs: %v", larkCfg.ChatIDs)

	if len(larkCfg.ChatIDs) == 0 {
		log.Fatal("没有配置 ChatID")
	}

	// 2. 创建内存图和测试数据
	memoryGraph := core.NewMemoryGraph()

	// 插入测试决策
	node1 := decision.NewDecisionNode("test-001", "测试决策卡片推送", "yatsenos", "lab0")
	node1.Decision = "这是一个测试决策，验证推送功能是否正常工作。"
	node1.Rationale = "测试卡片推送流程和飞书 API 集成"
	node1.ImpactLevel = decision.ImpactMajor
	node1.Status = decision.StatusDecided
	node1.AccessStats.HotScore = 90.0
	memoryGraph.AddDecision(node1)

	node2 := decision.NewDecisionNode("test-002", "另一个测试决策", "yatsenos", "lab0")
	node2.Decision = "第二个测试决策，测试每日摘要功能。"
	node2.ImpactLevel = decision.ImpactMinor
	node2.Status = decision.StatusCompleted
	node2.AccessStats.HotScore = 75.0
	memoryGraph.AddDecision(node2)

	// 3. 测试推送
	pushEngine := push.NewPushEngine(memoryGraph)

	log.Println("\n========= 测试1: 主动推送 ==========")
	for _, chatID := range larkCfg.ChatIDs {
		log.Printf("推送到: %s", chatID)
		pushed := pushEngine.PushProactive(chatID)
		log.Printf("  发送了 %d 条", pushed)
	}

	log.Println("\n========= 测试2: 每日摘要 ==========")
	for _, chatID := range larkCfg.ChatIDs {
		log.Printf("推送到: %s", chatID)
		card, err := pushEngine.DailySummary(chatID)
		if err != nil {
			log.Printf("  失败: %v", err)
		} else {
			log.Printf("  成功: %s", card[:60])
		}
	}

	log.Println("\n========= 测试3: 单条决策卡片 ==========")
	for _, chatID := range larkCfg.ChatIDs {
		log.Printf("推送到: %s", chatID)
		card, err := pushEngine.PushDecisionCard(chatID, "test-001")
		if err != nil {
			log.Printf("  失败: %v", err)
		} else {
			log.Printf("  成功")
		}
	}

	log.Println("\n测试完成！")
}
