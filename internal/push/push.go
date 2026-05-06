package push

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"feishu-mem/internal/card"
	"feishu-mem/internal/core"
	"feishu-mem/internal/decision"
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/recall"
)

// PushEngine 推送引擎
type PushEngine struct {
	memory     *core.MemoryGraph
	recall     *recall.RecallEngine
	cardRender *card.Renderer
	cli        *larkadapter.LarkCLI

	// 推送防重
	pushedMu         sync.Mutex
	pushedDecisions  map[string]time.Time // sdr_id → 最后推送时间
	pushCooldown     time.Duration        // 同一决策的推送冷却期
}

// NewPushEngine 创建推送引擎
func NewPushEngine(memory *core.MemoryGraph) *PushEngine {
	return &PushEngine{
		memory:          memory,
		recall:          recall.NewRecallEngine(memory),
		cardRender:      card.NewRenderer(),
		cli:             larkadapter.NewLarkCLI(),
		pushedDecisions: make(map[string]time.Time),
		pushCooldown:    24 * time.Hour,
	}
}

// PushDecisionCard 推送单个决策卡片到飞书群聊
func (pe *PushEngine) PushDecisionCard(chatID string, sdrID string) (string, error) {
	// 检查推送防重
	if !pe.canPush(sdrID) {
		log.Printf("[Push] Skipping %s: recently pushed", sdrID)
		return "", nil
	}

	d, ok := pe.memory.GetDecision(sdrID)
	if !ok {
		return "", fmt.Errorf("decision not found: %s", sdrID)
	}

	hotScore := pe.recall.CalculateHotScore(d)
	cardContent, err := pe.cardRender.RenderLarkCardFromNode(d, hotScore)
	if err != nil {
		return "", fmt.Errorf("render card failed: %w", err)
	}

	// 实际发送到飞书
	if err := pe.sendToFeishu(chatID, cardContent); err != nil {
		return "", fmt.Errorf("send failed: %w", err)
	}

	pe.markPushed(sdrID)
	log.Printf("[Push] Sent decision card %s to chat %s", sdrID, chatID)
	return cardContent, nil
}

// PushProactive 主动推送：基于 hot_score + 状态变化
func (pe *PushEngine) PushProactive(chatID string) int {
	pushed := 0

	// 1. 高热点决策（hot_score >= 80）
	hotDecisions := pe.memory.GetDecisionsByHotScore(80)
	for _, d := range hotDecisions {
		if pe.canPush(d.SDRID) {
			if err := pe.pushDecisionSummary(chatID, d, "🔥 高热点决策提醒"); err == nil {
				pushed++
			}
		}
	}

	// 2. 被遗忘的决策（hot_score < 20 且 status=decided）
	allDecisions := pe.memory.GetAllDecisions()
	for _, d := range allDecisions {
		if d.Status == decision.StatusDecided && d.AccessStats.HotScore < 20 {
			if pe.canPush(d.SDRID) {
				if err := pe.pushDecisionSummary(chatID, d, "⚠️ 以下决策可能已被遗忘"); err == nil {
					pushed++
				}
			}
		}
	}

	// 3. 最近完成的决策
	completedRecent := pe.getRecentlyCompleted(24 * time.Hour)
	for _, d := range completedRecent {
		if pe.canPush(d.SDRID) {
			if err := pe.pushDecisionSummary(chatID, d, "✅ 决策已完成"); err == nil {
				pushed++
			}
		}
	}

	if pushed > 0 {
		log.Printf("[Push] Proactive push: %d cards sent to %s", pushed, chatID)
	}
	return pushed
}

// HandleQuery 处理用户查询并返回决策卡片
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

	// 实际发送到飞书
	if err := pe.sendToFeishu(chatID, summaryCard); err != nil {
		return "", fmt.Errorf("send daily summary failed: %w", err)
	}

	log.Printf("[Push] Daily summary sent to %s", chatID)
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

// sendToFeishu 发送消息到飞书群聊
func (pe *PushEngine) sendToFeishu(chatID string, cardJSON string) error {
	if chatID == "" {
		return fmt.Errorf("chatID is empty")
	}

	// 使用 lark-cli 发送卡片消息
	output, err := pe.cli.RunCommand("im", "+messages-send",
		"--chat-id", chatID,
		"--msg-type", "interactive",
		"--content", cardJSON,
		"--as", "bot",
	)
	if err != nil {
		return fmt.Errorf("lark-cli send failed: %w", err)
	}

	log.Printf("[Push] Feishu response: %s", string(output))
	return nil
}

// sendTextToFeishu 发送文本消息到飞书群聊
func (pe *PushEngine) sendTextToFeishu(chatID string, text string) error {
	if chatID == "" {
		return fmt.Errorf("chatID is empty")
	}

	output, err := pe.cli.RunCommand("im", "+messages-send",
		"--chat-id", chatID,
		"--text", text,
		"--as", "bot",
	)
	if err != nil {
		return fmt.Errorf("lark-cli send failed: %w", err)
	}

	log.Printf("[Push] Feishu text response: %s", string(output))
	return nil
}

// pushDecisionSummary 推送决策摘要（简化卡片）
func (pe *PushEngine) pushDecisionSummary(chatID string, d *decision.DecisionNode, header string) error {
	text := fmt.Sprintf("%s\n\n📌 %s\n决策: %s\n状态: %s | 热点值: %.0f",
		header, d.Title, truncateStr(d.Decision, 100), d.Status, d.AccessStats.HotScore)

	if err := pe.sendTextToFeishu(chatID, text); err != nil {
		return err
	}

	pe.markPushed(d.SDRID)
	return nil
}

// canPush 检查是否可以推送（防重）
func (pe *PushEngine) canPush(sdrID string) bool {
	pe.pushedMu.Lock()
	defer pe.pushedMu.Unlock()

	lastPush, exists := pe.pushedDecisions[sdrID]
	if !exists {
		return true
	}
	return time.Since(lastPush) > pe.pushCooldown
}

// markPushed 标记已推送
func (pe *PushEngine) markPushed(sdrID string) {
	pe.pushedMu.Lock()
	defer pe.pushedMu.Unlock()
	pe.pushedDecisions[sdrID] = time.Now()
}

// getRecentlyCompleted 获取最近完成的决策
func (pe *PushEngine) getRecentlyCompleted(within time.Duration) []*decision.DecisionNode {
	var result []*decision.DecisionNode
	cutoff := time.Now().Add(-within)

	for _, d := range pe.memory.GetAllDecisions() {
		if d.Status == decision.StatusCompleted && d.CreatedAt.After(cutoff) {
			result = append(result, d)
		}
	}
	return result
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// RenderCardJSON 渲染决策为飞书卡片 JSON（供外部调用）
func (pe *PushEngine) RenderCardJSON(d *decision.DecisionNode) (string, error) {
	hotScore := pe.recall.CalculateHotScore(d)
	cardContent, err := pe.cardRender.RenderLarkCardFromNode(d, hotScore)
	if err != nil {
		return "", err
	}

	// 验证 JSON 格式
	var test map[string]interface{}
	if err := json.Unmarshal([]byte(cardContent), &test); err != nil {
		return "", fmt.Errorf("invalid card JSON: %w", err)
	}

	return cardContent, nil
}
