package push

import (
	"fmt"
	"time"

	"feishu-mem/internal/card"
	"feishu-mem/internal/core"
	"feishu-mem/internal/decision"
	"feishu-mem/internal/recall"
)

// PushEngine 推送引擎
type PushEngine struct {
	memory     *core.MemoryGraph
	recall     *recall.RecallEngine
	cardRender *card.Renderer
}

// NewPushEngine 创建推送引擎
func NewPushEngine(memory *core.MemoryGraph) *PushEngine {
	return &PushEngine{
		memory:     memory,
		recall:     recall.NewRecallEngine(memory),
		cardRender: card.NewRenderer(),
	}
}

// PushDecisionCard 推送单个决策卡片
func (pe *PushEngine) PushDecisionCard(chatID string, sdrID string) (string, error) {
	d, ok := pe.memory.GetDecision(sdrID)
	if !ok {
		return "", fmt.Errorf("decision not found: %s", sdrID)
	}

	hotScore := pe.recall.CalculateHotScore(d)
	cardContent, err := pe.cardRender.RenderLarkCardFromNode(d, hotScore)
	if err != nil {
		return "", fmt.Errorf("render card failed: %w", err)
	}

	// TODO: 实际发送到飞书的逻辑
	fmt.Printf("Sending card to chat %s: %s\n", chatID, cardContent)
	return cardContent, nil
}

// HandleQuery 处理用户查询并推送相关决策
func (pe *PushEngine) HandleQuery(chatID string, query string) ([]string, error) {
	decisions := pe.recall.SearchRecall(query, 5)

	var results []string
	for _, dCard := range decisions {
		cardContent, err := pe.cardRender.RenderLarkCard(dCard)
		if err != nil {
			return nil, fmt.Errorf("render card failed: %w", err)
		}
		results = append(results, cardContent)
	}

	return results, nil
}

// DailySummary 生成并推送每日摘要
func (pe *PushEngine) DailySummary(chatID string) (string, error) {
	now := time.Now()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	newDecisions := pe.memory.GetRecentDecisions(since)
	forgettingDecisions := pe.recall.GetForgottenDecisions(20)

	// 包装决策卡片
	var newCards []*recall.DecisionCard
	var forgotCards []*recall.DecisionCard

	for _, d := range newDecisions {
		hotScore := pe.recall.CalculateHotScore(d)
		newCards = append(newCards, &recall.DecisionCard{
			Decision: d,
			HotScore: hotScore,
		})
	}

	for _, d := range forgettingDecisions {
		hotScore := pe.recall.CalculateHotScore(d.Decision)
		forgotCards = append(forgotCards, &recall.DecisionCard{
			Decision: d.Decision,
			HotScore: hotScore,
		})
	}

	summaryCard, err := pe.cardRender.RenderDailySummary(now, newCards, forgotCards)
	if err != nil {
		return "", fmt.Errorf("render daily summary failed: %w", err)
	}

	// TODO: 实际发送到飞书的逻辑
	fmt.Printf("Sending daily summary to chat %s\n", chatID)
	return summaryCard, nil
}

// GetHotDecisions 获取高热点值决策列表
func (pe *PushEngine) GetHotDecisions(minScore float64, limit int) ([]*decision.DecisionNode, []float64) {
	decisions := pe.memory.GetDecisionsByHotScore(minScore)
	var hotScores []float64

	for _, d := range decisions {
		hotScores = append(hotScores, d.AccessStats.HotScore)
	}

	if limit > 0 && len(decisions) > limit {
		return decisions[:limit], hotScores[:limit]
	}
	return decisions, hotScores
}
