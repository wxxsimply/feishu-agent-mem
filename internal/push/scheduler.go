package push

import (
	"context"
	"log"
	"time"
)

// PushScheduler 推送调度器
// 负责定时触发主动推送（每日摘要、热点提醒、遗忘提醒）
type PushScheduler struct {
	engine        *PushEngine
	chatIDs       []string
	dailySummaryAt time.Duration // 每天几点推送（如 9*time.Hour = 9:00）
	proactiveInterval time.Duration // 主动推送检查间隔
}

// NewPushScheduler 创建推送调度器
func NewPushScheduler(engine *PushEngine, chatIDs []string) *PushScheduler {
	return &PushScheduler{
		engine:             engine,
		chatIDs:            chatIDs,
		dailySummaryAt:     9 * time.Hour, // 默认每天 9:00
		proactiveInterval:  1 * time.Hour,  // 默认每小时检查一次
	}
}

// Start 启动推送调度器
func (ps *PushScheduler) Start(ctx context.Context) {
	log.Printf("[PushScheduler] Starting with %d chats", len(ps.chatIDs))
	log.Printf("[PushScheduler] Daily summary at: %v", ps.dailySummaryAt)
	log.Printf("[PushScheduler] Proactive interval: %v", ps.proactiveInterval)

	// 每分钟检查是否到了推送时间
	minuteTicker := time.NewTicker(1 * time.Minute)
	defer minuteTicker.Stop()

	// 主动推送 ticker
	proactiveTicker := time.NewTicker(ps.proactiveInterval)
	defer proactiveTicker.Stop()

	lastDailyPush := time.Time{}

	for {
		select {
		case <-minuteTicker.C:
			// 检查是否到了每日摘要时间
			now := time.Now()
			todayTarget := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(ps.dailySummaryAt)

			if now.After(todayTarget) && lastDailyPush.Before(todayTarget) {
				log.Printf("[PushScheduler] Triggering daily summary")
				for _, chatID := range ps.chatIDs {
					if _, err := ps.engine.DailySummary(chatID); err != nil {
						log.Printf("[PushScheduler] Daily summary failed for %s: %v", chatID, err)
					}
				}
				lastDailyPush = now
			}

		case <-proactiveTicker.C:
			// 主动推送检查
			log.Printf("[PushScheduler] Running proactive push check")
			for _, chatID := range ps.chatIDs {
				pushed := ps.engine.PushProactive(chatID)
				if pushed > 0 {
					log.Printf("[PushScheduler] Proactive push: %d cards to %s", pushed, chatID)
				}
			}

		case <-ctx.Done():
			log.Printf("[PushScheduler] Stopped")
			return
		}
	}
}
