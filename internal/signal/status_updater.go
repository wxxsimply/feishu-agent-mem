package signal

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"feishu-mem/internal/decision"
)

// StatusUpdater 决策状态自动更新器
// 定时扫描 active 决策，根据 Deadline/EffectiveTime/讨论活跃度自动推进状态
type StatusUpdater struct {
	memory   MemoryGraphInterface
	pipeline PipelineInterface
	interval time.Duration
}

// NewStatusUpdater 创建状态更新器
func NewStatusUpdater(memory MemoryGraphInterface, pipeline PipelineInterface, interval time.Duration) *StatusUpdater {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &StatusUpdater{
		memory:   memory,
		pipeline: pipeline,
		interval: interval,
	}
}

// Start 启动状态更新器
func (su *StatusUpdater) Start(ctx context.Context) {
	log.Printf("[StatusUpdater] Starting with interval: %v", su.interval)
	ticker := time.NewTicker(su.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			su.runCheck()
		case <-ctx.Done():
			log.Printf("[StatusUpdater] Stopped")
			return
		}
	}
}

// runCheck 执行一次状态检查
func (su *StatusUpdater) runCheck() {
	allDecisions := su.memory.GetAllDecisions()
	now := time.Now()

	updated := 0
	for _, d := range allDecisions {
		if !d.IsActive() {
			continue
		}

		newStatus := su.evaluateStatus(d, now)
		if newStatus != "" && newStatus != d.Status {
			log.Printf("[StatusUpdater] %s: %s → %s", d.SDRID, d.Status, newStatus)
			mut := &DecisionMutation{
				Type:      MutationStatusChange,
				SDRID:     d.SDRID,
				NewStatus: newStatus,
			}
			if err := su.pipeline.ApplyMutation(mut); err != nil {
				log.Printf("[StatusUpdater] Failed to update %s: %v", d.SDRID, err)
			} else {
				updated++
			}
		}
	}

	if updated > 0 {
		log.Printf("[StatusUpdater] Updated %d decisions", updated)
	}
}

// evaluateStatus 评估决策应该处于什么状态
func (su *StatusUpdater) evaluateStatus(d *decision.DecisionNode, now time.Time) decision.DecisionStatus {
	// 规则 1: 过期检测 — Deadline 已过 → completed
	if d.Deadline != "" {
		deadline, err := parseFlexibleTime(d.Deadline)
		if err == nil && now.After(deadline) {
			// 只有 decided/executing 状态的决策才自动完成
			if d.Status == decision.StatusDecided || d.Status == decision.StatusExecuting {
				return decision.StatusCompleted
			}
		}
	}

	// 规则 2: 生效检测 — EffectiveTime 已到 → executing
	if d.EffectiveTime != "" && d.Status == decision.StatusPending {
		effective, err := parseFlexibleTime(d.EffectiveTime)
		if err == nil && now.After(effective) {
			return decision.StatusExecuting
		}
	}

	// 规则 3: 讨论超时 — in_discussion 超过 7 天无更新 → shelved
	if d.Status == decision.StatusInDiscussion {
		daysSinceCreation := now.Sub(d.CreatedAt).Hours() / 24
		if daysSinceCreation > 7 {
			return decision.StatusShelved
		}
	}

	// 规则 4: 老化检测 — pending 超过 30 天 → deprecated
	if d.Status == decision.StatusPending {
		daysSinceCreation := now.Sub(d.CreatedAt).Hours() / 24
		if daysSinceCreation > 30 {
			return decision.StatusDeprecated
		}
	}

	return "" // 不需要状态变更
}

// parseFlexibleTime 解析灵活的时间格式
// 支持: "2026-05-06", "2026-05-06 15:04", "下周三", "3天后" 等
func parseFlexibleTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}

	// ISO 格式
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	// 相对时间解析
	now := time.Now()
	lower := strings.ToLower(s)

	if strings.Contains(lower, "今天") || lower == "today" {
		return now, nil
	}
	if strings.Contains(lower, "明天") || lower == "tomorrow" {
		return now.Add(24 * time.Hour), nil
	}
	if strings.Contains(lower, "后天") {
		return now.Add(48 * time.Hour), nil
	}
	if strings.Contains(lower, "下周") || strings.Contains(lower, "下周一") {
		// 计算到下周一的天数
		daysUntilMonday := (8 - int(now.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		return now.Add(time.Duration(daysUntilMonday) * 24 * time.Hour), nil
	}
	if strings.Contains(lower, "下周三") {
		daysUntilWednesday := (3 - int(now.Weekday()) + 7) % 7
		if daysUntilWednesday == 0 {
			daysUntilWednesday = 7
		}
		return now.Add(time.Duration(daysUntilWednesday) * 24 * time.Hour), nil
	}
	if strings.Contains(lower, "下周五") {
		daysUntilFriday := (5 - int(now.Weekday()) + 7) % 7
		if daysUntilFriday == 0 {
			daysUntilFriday = 7
		}
		return now.Add(time.Duration(daysUntilFriday) * 24 * time.Hour), nil
	}

	// "N天后" 模式
	if strings.Contains(lower, "天后") {
		var days int
		fmt.Sscanf(lower, "%d天后", &days)
		if days > 0 {
			return now.Add(time.Duration(days) * 24 * time.Hour), nil
		}
	}
	if strings.Contains(lower, "days") {
		var days int
		fmt.Sscanf(lower, "%d days", &days)
		if days > 0 {
			return now.Add(time.Duration(days) * 24 * time.Hour), nil
		}
	}

	// "本周五" 模式
	if strings.Contains(lower, "本周五") {
		daysUntilFriday := (5 - int(now.Weekday()) + 7) % 7
		return now.Add(time.Duration(daysUntilFriday) * 24 * time.Hour), nil
	}
	if strings.Contains(lower, "本周三") {
		daysUntilWednesday := (3 - int(now.Weekday()) + 7) % 7
		return now.Add(time.Duration(daysUntilWednesday) * 24 * time.Hour), nil
	}

	return time.Time{}, fmt.Errorf("unparseable time: %s", s)
}
