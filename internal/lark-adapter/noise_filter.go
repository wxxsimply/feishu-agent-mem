package larkadapter

import (
	"strings"
	"unicode"
)

// NoiseFilter IM 消息噪声过滤器
type NoiseFilter struct {
	noiseWords []string // 噪声词（问候、附和等）
	noiseTopics []string // 噪声话题（闲聊模式）
	minLength  int      // 最短有效消息长度（字符数）
}

// NewNoiseFilter 创建噪声过滤器
func NewNoiseFilter() *NoiseFilter {
	return &NoiseFilter{
		noiseWords: []string{
			// 问候/附和
			"好的", "收到", "嗯", "哦", "OK", "ok", "Ok",
			"好", "行", "是", "对", "嗯嗯", "哈哈", "嘿嘿",
			"谢谢", "感谢", "辛苦了", "赞", "+1", "1",
			// 英文
			"yes", "no", "thanks", "thx", "got it", "sure",
			"lol", "haha", "nice", "good", "well",
		},
		noiseTopics: []string{
			// 天气/闲聊
			"天气", "下雨", "太阳", "温度", "热", "冷",
			"吃饭", "午餐", "晚餐", "早餐", "外卖", "食堂",
			"周末", "假期", "旅游", "电影", "游戏", "综艺",
			"地铁", "公交", "堵车", "迟到",
		},
		minLength: 5,
	}
}

// IsNoise 判断消息是否为噪声
func (f *NoiseFilter) IsNoise(record MessageRecord) bool {
	content := strings.TrimSpace(record.Content)
	if content == "" {
		return true
	}

	// 规则 1: 纯表情/emoji
	if isOnlyEmoji(content) {
		return true
	}

	// 规则 2: 噪声词精确匹配
	contentLower := strings.ToLower(content)
	for _, w := range f.noiseWords {
		if contentLower == strings.ToLower(w) {
			return true
		}
	}

	// 规则 3: 短消息（< minLength）且无决策关键词
	runes := []rune(content)
	if len(runes) < f.minLength && !containsDecisionSignal(content) {
		return true
	}

	// 规则 4: 噪声话题（闲聊）
	if containsNoiseTopic(contentLower, f.noiseTopics) && !containsDecisionSignal(content) {
		return true
	}

	// 规则 4: 纯链接分享（无附加评论）
	if isLinkOnly(content) {
		return true
	}

	return false
}

// isOnlyEmoji 判断是否纯表情
func isOnlyEmoji(s string) bool {
	// 去除空格和常见标点后，检查是否只剩 emoji
	cleaned := strings.NewReplacer(
		" ", "", "\n", "", "\t", "",
		".", "", ",", "", "!", "", "?", "",
	).Replace(s)

	if cleaned == "" {
		return false
	}

	emojiCount := 0
	nonEmojiCount := 0
	for _, r := range cleaned {
		if isEmoji(r) {
			emojiCount++
		} else {
			nonEmojiCount++
		}
	}

	// 全是 emoji，或 emoji 占比超过 80%
	return nonEmojiCount == 0 || (emojiCount > 0 && float64(emojiCount)/float64(emojiCount+nonEmojiCount) > 0.8)
}

// isEmoji 判断 rune 是否为 emoji
func isEmoji(r rune) bool {
	// 常见 emoji Unicode 范围
	return (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
		(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols and Pictographs
		(r >= 0x1F680 && r <= 0x1F6FF) || // Transport and Map
		(r >= 0x1F1E0 && r <= 0x1F1FF) || // Flags
		(r >= 0x2600 && r <= 0x26FF) || // Misc symbols
		(r >= 0x2700 && r <= 0x27BF) || // Dingbats
		(r >= 0xFE00 && r <= 0xFE0F) || // Variation Selectors
		(r >= 0x200D && r <= 0x200D) || // Zero Width Joiner
		(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols
		(r >= 0x1FA00 && r <= 0x1FA6F) || // Chess Symbols
		(r >= 0x1FA70 && r <= 0x1FAFF) // Symbols and Pictographs Extended
}

// containsDecisionSignal 判断是否包含决策信号词
func containsDecisionSignal(text string) bool {
	signals := []string{
		"决定", "确认", "采用", "选择", "结论", "通过",
		"方案", "选型", "确认", "批准", "同意", "拒绝",
		"decide", "confirm", "adopt", "select", "choose",
		"approve", "agree", "reject", "conclusion",
	}
	lower := strings.ToLower(text)
	for _, s := range signals {
		if strings.Contains(lower, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// isLinkOnly 判断是否纯链接分享
func isLinkOnly(content string) bool {
	// 如果包含 http/https 且去除链接后内容很短
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "http") {
		return false
	}

	// 简单判断：如果内容以 http 开头且没有其他实质文字
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		// 检查链接后面是否有附加评论
		parts := strings.SplitN(trimmed, " ", 2)
		if len(parts) == 1 {
			return true // 纯链接
		}
		// 链接后的评论是否太短
		comment := strings.TrimSpace(parts[1])
		if len([]rune(comment)) < 3 {
			return true
		}
	}

	return false
}

// IsNoiseForDoc 文档评论的噪声判断（更宽松）
func (f *NoiseFilter) IsNoiseForDoc(commentText string) bool {
	text := strings.TrimSpace(commentText)
	if text == "" {
		return true
	}

	// 文档评论的噪声词（更严格）
	docNoise := []string{"LGTM", "lgtm", "+1", "👍", "好的", "收到"}
	textLower := strings.ToLower(text)
	for _, w := range docNoise {
		if textLower == strings.ToLower(w) {
			return true
		}
	}

	return false
}

// hasCJK 判断是否包含中日韩字符
func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) {
			return true
		}
	}
	return false
}

// containsNoiseTopic 判断是否包含噪声话题
func containsNoiseTopic(text string, topics []string) bool {
	for _, t := range topics {
		if strings.Contains(text, t) {
			return true
		}
	}
	return false
}
