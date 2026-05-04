package larkadapter

import (
	"log"
	"sort"
	"sync"
	"time"
)

// MessageBatcher 消息分组与上下文构建
type MessageBatcher struct {
	mu           sync.Mutex
	chatCache    map[string][]MessageRecord // chat_id → 有序消息列表
	contextLimit int                        // 上下文窗口大小
	cacheExpiry  time.Duration
	lastCleanup  time.Time
}

// NewMessageBatcher 创建消息批处理器
func NewMessageBatcher() *MessageBatcher {
	return &MessageBatcher{
		chatCache:    make(map[string][]MessageRecord),
		contextLimit: 10,
		cacheExpiry:  5 * time.Minute,
		lastCleanup:  time.Now(),
	}
}

// UpdateCache 更新指定 chat 的消息缓存
func (b *MessageBatcher) UpdateCache(chatID string, records []MessageRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(records) == 0 {
		return
	}

	// 合并到缓存
	existing := b.chatCache[chatID]
	existing = mergeRecords(existing, records)

	// 按时间排序
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].CreateTime < existing[j].CreateTime
	})

	// 裁剪到限制大小
	if len(existing) > b.contextLimit*2 {
		existing = existing[len(existing)-b.contextLimit*2:]
	}

	b.chatCache[chatID] = existing
	b.cleanup()
}

// BuildContext 为单条消息构建上下文
func (b *MessageBatcher) BuildContext(record MessageRecord, index int, allRecords []MessageRecord) *ContextMessage {
	ctx := &ContextMessage{
		MessageIndex: index + 1,
	}

	// 1. 查找同 thread 的历史消息
	if record.ThreadID != "" {
		ctx.ThreadHistory = b.findThreadHistory(record.ThreadID, record.CreateTime, record.ChatID)
	}

	// 2. 查找同 chat 的最近消息（排除 thread 内的重复）
	ctx.ChatHistory = b.findChatHistory(record.ChatID, record.CreateTime, record.ThreadID, record.MessageID)

	// 3. 提取关键词
	combined := append(ctx.ThreadHistory, ctx.ChatHistory...)
	ctx.Keywords = ExtractKeywords(combined, 5)

	// 4. 构建 Change
	changeType := "new"
	if record.MsgType == "text" || record.MsgType == "post" {
		changeType = "new_text"
	}

	entityType := "group_message"
	summary := record.Content
	if len([]rune(summary)) > 60 {
		summary = string([]rune(summary)[:60]) + "..."
	}

	ctx.Change = Change{
		Type:       changeType,
		EntityType: entityType,
		EntityID:   record.MessageID,
		Summary:    summary,
		Timestamp:  record.CreateTime,
		ChatID:     record.ChatID,
		ThreadID:   record.ThreadID,
		SenderID:   record.SenderID,
		SenderName: record.SenderName,
		MentionIDs: record.Mentions,
		RawContent: record.Content,
	}

	return ctx
}

// findThreadHistory 查找同一 thread 的历史消息
func (b *MessageBatcher) findThreadHistory(threadID string, beforeTime int64, chatID string) []MessageRecord {
	b.mu.Lock()
	defer b.mu.Unlock()

	records, ok := b.chatCache[chatID]
	if !ok {
		return nil
	}

	var history []MessageRecord
	for _, r := range records {
		if r.ThreadID == threadID && r.CreateTime < beforeTime {
			history = append(history, r)
		}
	}

	// 取最近的 N 条
	if len(history) > b.contextLimit {
		history = history[len(history)-b.contextLimit:]
	}
	return history
}

// findChatHistory 查找同 chat 的最近消息
func (b *MessageBatcher) findChatHistory(chatID string, beforeTime int64, excludeThreadID string, excludeMsgID string) []MessageRecord {
	b.mu.Lock()
	defer b.mu.Unlock()

	records, ok := b.chatCache[chatID]
	if !ok {
		return nil
	}

	var history []MessageRecord
	for _, r := range records {
		if r.CreateTime >= beforeTime {
			continue
		}
		if r.MessageID == excludeMsgID {
			continue
		}
		// 如果消息属于某 thread，且与当前消息 thread 不同，跳过（thread 消息已归类）
		if r.ThreadID != "" && r.ThreadID != excludeThreadID {
			continue
		}
		history = append(history, r)
	}

	// 取最近的 N 条
	if len(history) > b.contextLimit {
		history = history[len(history)-b.contextLimit:]
	}
	return history
}

// cleanup 定期清理过期缓存
func (b *MessageBatcher) cleanup() {
	if time.Since(b.lastCleanup) < b.cacheExpiry {
		return
	}
	b.lastCleanup = time.Now()

	for chatID, records := range b.chatCache {
		cutoff := time.Now().Add(-b.cacheExpiry).Unix()
		var kept []MessageRecord
		for _, r := range records {
			if r.CreateTime >= cutoff {
				kept = append(kept, r)
			}
		}
		if len(kept) == 0 {
			delete(b.chatCache, chatID)
			log.Printf("[MessageBatcher] Cleared expired cache for chat %s", chatID)
		} else {
			b.chatCache[chatID] = kept
		}
	}
}

// mergeRecords 合并两条消息列表（去重）
func mergeRecords(existing, incoming []MessageRecord) []MessageRecord {
	seen := make(map[string]bool)
	for _, r := range existing {
		seen[r.MessageID] = true
	}

	merged := existing
	for _, r := range incoming {
		if !seen[r.MessageID] {
			merged = append(merged, r)
			seen[r.MessageID] = true
		}
	}
	return merged
}
