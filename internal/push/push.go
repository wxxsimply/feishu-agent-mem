package push

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
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

	// 推送防重（基于 SDRID，24h 冷却）
	pushedMu        sync.Mutex
	pushedDecisions map[string]time.Time // sdr_id → 最后推送时间
	pushCooldown    time.Duration        // 同一决策的推送冷却期

	// 内容去重（基于决策内容 hash，5min 冷却）
	contentMu        sync.Mutex
	recentPushedHash map[string]time.Time // content_hash → 最后推送时间
	contentCooldown  time.Duration
}

// NewPushEngine 创建推送引擎
func NewPushEngine(memory *core.MemoryGraph) *PushEngine {
	return &PushEngine{
		memory:           memory,
		recall:           recall.NewRecallEngine(memory),
		cardRender:       card.NewRenderer(),
		cli:              larkadapter.NewLarkCLI(),
		pushedDecisions:  make(map[string]time.Time),
		pushCooldown:     24 * time.Hour,
		recentPushedHash: make(map[string]time.Time),
		contentCooldown:  5 * time.Minute,
	}
}

// PushDecisionCard 推送单个决策卡片到飞书群聊
func (pe *PushEngine) PushDecisionCard(chatID string, sdrID string) (string, error) {
	d, ok := pe.memory.GetDecision(sdrID)
	if !ok {
		return "", fmt.Errorf("decision not found: %s", sdrID)
	}

	// 基于 SDRID 的推送防重（24h 冷却）
	if !pe.canPush(sdrID) {
		log.Printf("[Push] Skipping %s: recently pushed (SDRID cooldown)", sdrID)
		return "", nil
	}

	// 基于决策内容的推送去重（5min 冷却）
	contentHash := pe.decisionContentHash(d)
	if !pe.canPushContent(contentHash) {
		log.Printf("[Push] Skipping %s: similar content recently pushed", sdrID)
		return "", nil
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
	pe.markPushedContent(contentHash)
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

// DailySummary 生成并推送每日摘要（只在有内容时发送）
func (pe *PushEngine) DailySummary(chatID string) (string, error) {
	now := time.Now()
	since := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	newDecisions := pe.memory.GetRecentDecisions(since)
	forgettingDecisions := pe.recall.GetForgottenDecisions(20)

	// 如果没有内容就不发送
	if len(newDecisions) == 0 && len(forgettingDecisions) == 0 {
		log.Printf("[Push] No decisions for daily summary, skipping")
		return "", nil
	}

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

// NotifyConflict 实现 core.ConflictNotifier 接口
// 当决策冲突需要用户确认时，推送冲突解决卡片到群聊
func (pe *PushEngine) NotifyConflict(nodeA, nodeB *decision.DecisionNode, reason string) {
	chatIDs := pe.getChatIDs()
	if len(chatIDs) == 0 {
		log.Printf("[Push] No chat IDs configured for conflict notification")
		return
	}
	for _, chatID := range chatIDs {
		if _, err := pe.PushConflictResolutionCard(chatID, nodeA, nodeB, reason); err != nil {
			log.Printf("[Push] Failed to push conflict card to %s: %v", chatID, err)
		}
	}
}

// getChatIDs 获取推送目标群聊列表
func (pe *PushEngine) getChatIDs() []string {
	// 从配置加载
	cfg := larkadapter.LoadConfig()
	if len(cfg.ChatIDs) > 0 {
		return cfg.ChatIDs
	}
	return nil
}

// NotifyDecisionUpdate 通知有决策更新，触发相关推送
func (pe *PushEngine) NotifyDecisionUpdate(chatIDs []string, node *decision.DecisionNode) {
	log.Printf("[Push] Notified of decision update: %s (%s)", node.SDRID, node.Title)

	for _, chatID := range chatIDs {
		// 推送新/更新的决策卡片
		if _, err := pe.PushDecisionCard(chatID, node.SDRID); err != nil {
			log.Printf("[Push] Failed to push decision card %s to %s: %v", node.SDRID, chatID, err)
		}
	}
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

	// 写卡片 JSON 到临时文件，避免 bash 命令行转义问题
	tmpFile, err := os.CreateTemp("", "feishu-card-*.json")
	if err != nil {
		return fmt.Errorf("create temp file failed: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(cardJSON); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file failed: %w", err)
	}
	tmpFile.Close()

	// 使用 bash 读取文件内容发送（避免 Windows 编码和 bash 转义问题）
	cmdLine := fmt.Sprintf(`lark-cli im +messages-send --chat-id '%s' --msg-type interactive --content "$(cat '%s')" --as bot`,
		chatID, tmpPath)
	output, err := pe.cli.RunBashCommand(cmdLine)
	if err != nil {
		return fmt.Errorf("lark-cli send failed: %w", err)
	}
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

// markPushed 标记已推送（基于 SDRID）
func (pe *PushEngine) markPushed(sdrID string) {
	pe.pushedMu.Lock()
	defer pe.pushedMu.Unlock()
	pe.pushedDecisions[sdrID] = time.Now()
}

// decisionContentHash 基于决策内容生成 hash 用于内容去重
func (pe *PushEngine) decisionContentHash(d *decision.DecisionNode) string {
	// 用 Title + Decision 的主体内容生成 hash
	content := strings.TrimSpace(d.Title) + "|" + strings.TrimSpace(d.Decision)
	// 取前 200 字即可，足够区分不同决策
	runes := []rune(content)
	if len(runes) > 200 {
		content = string(runes[:200])
	}
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:16]) // 取前 16 字节，足够区分
}

// canPushContent 检查基于内容的推送去重
func (pe *PushEngine) canPushContent(contentHash string) bool {
	pe.contentMu.Lock()
	defer pe.contentMu.Unlock()

	lastPush, exists := pe.recentPushedHash[contentHash]
	if !exists {
		return true
	}
	return time.Since(lastPush) > pe.contentCooldown
}

// markPushedContent 标记内容已推送
func (pe *PushEngine) markPushedContent(contentHash string) {
	pe.contentMu.Lock()
	defer pe.contentMu.Unlock()
	pe.recentPushedHash[contentHash] = time.Now()
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

// PushConflictResolutionCard 推送冲突解决卡片到飞书群聊
// 展示两个冲突决策，用户可选择保留哪一个
func (pe *PushEngine) PushConflictResolutionCard(chatID string, nodeA, nodeB *decision.DecisionNode, reason string) (string, error) {
	if chatID == "" {
		return "", fmt.Errorf("chatID is empty")
	}

	cardContent, err := pe.cardRender.RenderConflictResolutionCard(nodeA, nodeB, reason)
	if err != nil {
		return "", fmt.Errorf("render conflict card failed: %w", err)
	}

	if err := pe.sendToFeishu(chatID, cardContent); err != nil {
		return "", fmt.Errorf("send conflict card failed: %w", err)
	}

	log.Printf("[Push] Sent conflict resolution card to %s: %s vs %s", chatID, nodeA.SDRID, nodeB.SDRID)
	return cardContent, nil
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
