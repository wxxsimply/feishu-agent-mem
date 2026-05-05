package larkadapter

import (
	"fmt"
	"log"
	"sort"
	"strings"
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
	noiseFilter  *NoiseFilter // 噪声过滤器
}

// NewMessageBatcher 创建消息批处理器
func NewMessageBatcher() *MessageBatcher {
	return &MessageBatcher{
		chatCache:    make(map[string][]MessageRecord),
		contextLimit: 30, // 从 10 扩展到 30
		cacheExpiry:  30 * time.Minute, // 从 5min 扩展到 30min
		lastCleanup:  time.Now(),
		noiseFilter:  NewNoiseFilter(),
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

// GroupMessages 将消息列表聚合为讨论组
// Layer 1: Thread 精确聚合
// Layer 2: 时间窗口 + 关键词聚合
// Layer 3: 噪声过滤（在聚合前执行）
func (b *MessageBatcher) GroupMessages(records []MessageRecord) [][]MessageRecord {
	if len(records) == 0 {
		return nil
	}

	// Layer 3: 噪声过滤
	var valid []MessageRecord
	for _, r := range records {
		if !b.noiseFilter.IsNoise(r) {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		log.Printf("[MessageBatcher] All %d messages filtered as noise", len(records))
		return nil
	}
	log.Printf("[MessageBatcher] Noise filter: %d → %d messages", len(records), len(valid))

	// Layer 1: Thread 精确聚合
	threadGroups := make(map[string][]MessageRecord)
	var nonThread []MessageRecord
	for _, r := range valid {
		if r.ThreadID != "" {
			threadGroups[r.ThreadID] = append(threadGroups[r.ThreadID], r)
		} else {
			nonThread = append(nonThread, r)
		}
	}

	// Layer 2: 非 Thread 消息的时间窗口 + 关键词聚合
	timeGroups := groupByTimeAndKeywords(nonThread, 5*time.Minute)

	// 合并结果
	var result [][]MessageRecord
	for _, g := range threadGroups {
		result = append(result, g)
	}
	result = append(result, timeGroups...)

	log.Printf("[MessageBatcher] Grouped %d valid messages into %d discussion groups", len(valid), len(result))
	return result
}

// BuildGroupContext 为一组消息构建上下文
func (b *MessageBatcher) BuildGroupContext(group []MessageRecord) *ContextMessage {
	if len(group) == 0 {
		return nil
	}

	// 取组内最后一条消息作为主消息
	lastMsg := group[len(group)-1]

	// 构建上下文文本：组内所有消息拼接
	var sb strings.Builder
	sb.WriteString("【群聊讨论内容】\n")
	for _, msg := range group {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.SenderName, truncateContent(msg.Content, 300)))
	}

	// 构建 Change
	changeType := "new_text"
	entityType := "group_message"
	summary := fmt.Sprintf("[群聊] %s: %s", lastMsg.SenderName, truncateContent(lastMsg.Content, 60))

	ctx := &ContextMessage{
		MessageIndex: len(group),
		Change: Change{
			Type:        changeType,
			EntityType:  entityType,
			EntityID:    lastMsg.MessageID, // 使用最后一条消息的 ID
			Summary:     summary,
			Timestamp:   lastMsg.CreateTime,
			ChatID:      lastMsg.ChatID,
			ThreadID:    lastMsg.ThreadID,
			SenderID:    lastMsg.SenderID,
			SenderName:  lastMsg.SenderName,
			MentionIDs:  lastMsg.Mentions,
			RawContent:  sb.String(), // 整个讨论组的内容
			ContextText: sb.String(),
		},
	}

	// 提取关键词
	ctx.Keywords = ExtractKeywords(group, 5)

	return ctx
}

// BuildContext 为单条消息构建上下文（保留兼容性）
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

// groupByTimeAndKeywords 按时间窗口 + 关键词聚合消息
func groupByTimeAndKeywords(records []MessageRecord, window time.Duration) [][]MessageRecord {
	if len(records) == 0 {
		return nil
	}

	var groups [][]MessageRecord
	currentGroup := []MessageRecord{records[0]}

	for i := 1; i < len(records); i++ {
		timeDiff := records[i].CreateTime - records[i-1].CreateTime
		if timeDiff > int64(window.Seconds()) {
			// 超出时间窗口，开始新组
			groups = append(groups, currentGroup)
			currentGroup = []MessageRecord{records[i]}
		} else {
			// 检查关键词重叠
			if hasKeywordOverlap(currentGroup, records[i]) {
				currentGroup = append(currentGroup, records[i])
			} else {
				// 无关键词重叠，各自独立
				groups = append(groups, currentGroup)
				currentGroup = []MessageRecord{records[i]}
			}
		}
	}
	groups = append(groups, currentGroup)
	return groups
}

// hasKeywordOverlap 检查新消息是否与组内消息共享关键词
func hasKeywordOverlap(group []MessageRecord, newMsg MessageRecord) bool {
	// 提取组内关键词
	groupKeywords := make(map[string]bool)
	for _, msg := range group {
		words := extractWords(msg.Content)
		for _, w := range words {
			groupKeywords[w] = true
		}
	}

	// 检查新消息的关键词是否与组内重叠
	newWords := extractWords(newMsg.Content)
	overlapCount := 0
	for _, w := range newWords {
		if groupKeywords[w] {
			overlapCount++
		}
	}

	// 至少 1 个关键词重叠
	return overlapCount > 0
}

// extractWords 从文本中提取关键词（简单分词）
func extractWords(text string) []string {
	// 中文按字符拆分，英文按空格拆分
	var words []string

	// 英文单词
	for _, w := range strings.Fields(text) {
		w = strings.ToLower(strings.Trim(w, ".,!?;:\"'()[]{}"))
		if len(w) >= 2 {
			words = append(words, w)
		}
	}

	// 中文：提取 2-4 字的连续中文字符
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if isChinese(runes[i]) {
			// 尝试提取 2-4 字的中文词
			for length := 4; length >= 2; length-- {
				if i+length <= len(runes) {
					word := string(runes[i : i+length])
					if allChinese(word) {
						words = append(words, word)
					}
				}
			}
		}
	}

	return words
}

// isChinese 判断 rune 是否为中文字符
func isChinese(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

// allChinese 判断字符串是否全部为中文字符
func allChinese(s string) bool {
	for _, r := range s {
		if !isChinese(r) {
			return false
		}
	}
	return true
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

