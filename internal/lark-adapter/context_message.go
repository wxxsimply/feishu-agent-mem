package larkadapter

import (
	"fmt"
	"strings"
)

// MessageRecord 完整消息记录
type MessageRecord struct {
	MessageID  string
	ChatID     string
	ThreadID   string
	SenderID   string
	SenderName string
	Content    string
	MsgType    string
	CreateTime int64
	Mentions   []string
}

// ContextMessage 携带上下文的检测消息
type ContextMessage struct {
	Change        Change
	ChatHistory   []MessageRecord // 同 chat 的最近消息（不含 thread 重复）
	ThreadHistory []MessageRecord // 同 thread 的历史消息（如果有）
	MessageIndex  int             // 在当前 chat/thread 中的序号（从1开始）
	Keywords      []string        // 从历史消息中提取的关键词
}

// BuildLLMInput 拼接上下文文本供 LLM 使用
func (cm *ContextMessage) BuildLLMInput() string {
	var sb strings.Builder

	// 1. 线程历史（最相关）
	if len(cm.ThreadHistory) > 0 {
		sb.WriteString("【同一话题的历史讨论】\n")
		for _, msg := range cm.ThreadHistory {
			sb.WriteString(fmt.Sprintf("%s: %s\n", msg.SenderName, truncateContent(msg.Content, 200)))
		}
		sb.WriteString("\n")
	}

	// 2. 最近聊天历史（背景信息）
	if len(cm.ChatHistory) > 0 {
		sb.WriteString("【最近聊天记录】\n")
		for _, msg := range cm.ChatHistory {
			sb.WriteString(fmt.Sprintf("%s: %s\n", msg.SenderName, truncateContent(msg.Content, 200)))
		}
		sb.WriteString("\n")
	}

	// 3. 当前消息
	sb.WriteString("【当前消息】\n")
	sender := cm.Change.SenderName
	if sender == "" {
		sender = "unknown"
	}
	content := cm.Change.RawContent
	if content == "" {
		content = cm.Change.Summary
	}
	sb.WriteString(fmt.Sprintf("%s: %s\n", sender, content))

	return sb.String()
}

// ExtractKeywords 从历史消息中提取关键词
func ExtractKeywords(records []MessageRecord, maxKeywords int) []string {
	if maxKeywords <= 0 {
		maxKeywords = 5
	}
	seen := make(map[string]int)
	for _, r := range records {
		if r.MsgType != "text" {
			continue
		}
		words := strings.Fields(r.Content)
		for _, w := range words {
			w = strings.TrimSpace(w)
			if len([]rune(w)) < 2 {
				continue
			}
			seen[w]++
		}
	}

	type kw struct {
		word  string
		count int
	}
	var sorted []kw
	for word, count := range seen {
		sorted = append(sorted, kw{word, count})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var result []string
	for i := 0; i < len(sorted) && i < maxKeywords; i++ {
		result = append(result, sorted[i].word)
	}
	return result
}
