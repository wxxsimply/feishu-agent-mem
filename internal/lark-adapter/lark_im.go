package larkadapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// IMExtractor 群聊消息提取器
type IMExtractor struct {
	config  *Config
	cli     *LarkCLI
	batcher *MessageBatcher
}

// LarkEvent 飞书事件结构
type LarkEvent struct {
	UUID      string         `json:"uuid"`
	Timestamp string         `json:"timestamp"`
	EventType string         `json:"event_type"`
	Event     map[string]any `json:"event"`
	Header    map[string]any `json:"header,omitempty"`
}

// IMMessageReceiveV1 接收消息事件
type IMMessageReceiveV1 struct {
	Header struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		CreateTime string `json:"create_time"`
		Token      string `json:"token"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
	Event struct {
		Sender struct {
			SenderID struct {
				OpenID  string `json:"open_id"`
				UserID  string `json:"user_id"`
				UnionID string `json:"union_id"`
			} `json:"sender_id"`
			SenderType string `json:"sender_type"`
			TenantKey  string `json:"tenant_key"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			RootID      string `json:"root_id"`
			ParentID    string `json:"parent_id"`
			CreateTime  string `json:"create_time"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
			Mentions    []struct {
				Key       string `json:"key"`
				ID        struct {
					OpenID  string `json:"open_id"`
					UserID  string `json:"user_id"`
					UnionID string `json:"union_id"`
				} `json:"id"`
				Name string `json:"name"`
			} `json:"mentions"`
		} `json:"event"`
	}
}

// NewIMExtractor 创建群聊提取器
func NewIMExtractor(cfg *Config) *IMExtractor {
	return &IMExtractor{
		config:  cfg,
		cli:     NewLarkCLI(),
		batcher: NewMessageBatcher(),
	}
}

// IMSender 群聊消息发送器
type IMSender struct {
	config *Config
	cli    *LarkCLI
}

// NewIMSender 创建消息发送器
func NewIMSender(cfg *Config) *IMSender {
	return &IMSender{
		config: cfg,
		cli:    NewLarkCLI(),
	}
}

// SendTextMessage 发送文本消息到指定 chat_id
func (s *IMSender) SendTextMessage(chatID, text string) (string, error) {
	args := []string{"im", "+messages-send", "--chat-id", chatID, "--text", text, "--as", "bot"}
	output, err := s.cli.RunCommand(args...)
	if err != nil {
		return "", fmt.Errorf("send text message failed: %w", err)
	}
	return string(output), nil
}

// SendTextMessageToAll 发送文本消息到所有配置的 chat_id
func (s *IMSender) SendTextMessageToAll(text string) ([]string, error) {
	var results []string
	for _, chatID := range s.config.ChatIDs {
		result, err := s.SendTextMessage(chatID, text)
		if err != nil {
			results = append(results, fmt.Sprintf("chat %s: error: %v", chatID, err))
		} else {
			results = append(results, fmt.Sprintf("chat %s: success: %s", chatID, result))
		}
	}
	return results, nil
}

// Name 实现接口
func (e *IMExtractor) Name() string {
	return "lark_im"
}

// Detect 检测消息变化：群聊 + P2P（带上下文拼接）
func (e *IMExtractor) Detect(lastCheck time.Time) (*DetectResult, error) {
	var changes []Change

	if lastCheck.IsZero() {
		lastCheck = time.Now().Add(-1 * time.Hour)
	}

	cutoff := lastCheck.Unix()

	// 1. 检测群聊消息
	for _, rawChatID := range e.config.ChatIDs {
		chatID := rawChatID
		if strings.Contains(chatID, ",") {
			chatID = strings.Split(chatID, ",")[0]
		}

		log.Printf("[IM] Polling chat %s (since %d)", chatID, cutoff)
		items, err := e.getGroupMessageItems(chatID, lastCheck)
		if err != nil {
			log.Printf("[IM] Error polling chat %s: %v", chatID, err)
			continue
		}
		log.Printf("[IM] Chat %s returned %d messages", chatID, len(items))

		var records []MessageRecord
		for _, item := range items {
			ts := extractTimestamp(item)
			if ts <= cutoff {
				continue
			}
			records = append(records, extractMessageRecord(item, chatID))
		}

		e.batcher.UpdateCache(chatID, records)

		// 使用消息聚合：将相关消息分组，每个组作为一个整体送 LLM
		groups := e.batcher.GroupMessages(records)
		for _, group := range groups {
			ctxMsg := e.batcher.BuildGroupContext(group)
			if ctxMsg != nil {
				changes = append(changes, ctxMsg.Change)
			}
		}
	}

	// 2. P2P 双人会话
	if e.config.UserID != "" {
		p2pItems, err := e.getP2PMessageItems(lastCheck)
		if err == nil {
			for _, item := range p2pItems {
				ts := extractTimestamp(item)
				if ts <= cutoff {
					continue
				}
				mid, _ := item["message_id"].(string)
				body := extractBody(item)
				senderName := extractSender(item)
				msgType, _ := item["msg_type"].(string)
				changes = append(changes, e.classifyMessageChange("p2p_message", mid, msgType, senderName, "", "", body, ts))
			}
		}
	}

	result := &DetectResult{
		Source:     e.Name(),
		HasChanges: len(changes) > 0,
		DetectedAt: time.Now(),
		LastCheck:  lastCheck,
		Changes:    changes,
	}
	log.Printf("[IM] Detect complete: %d changes", len(changes))
	_ = SaveDetectResult(result)
	return result, nil
}

func extractMessageRecord(item map[string]any, chatID string) MessageRecord {
	record := MessageRecord{
		MessageID:  getStringField(item, "message_id"),
		ChatID:     chatID,
		CreateTime: extractTimestamp(item),
		MsgType:    getStringField(item, "msg_type"),
	}
	record.ThreadID = getStringField(item, "thread_id")
	record.Content = extractBody(item)

	if sender, ok := item["sender"].(map[string]any); ok {
		record.SenderName = getStringField(sender, "name")
		record.SenderID = getStringField(sender, "id")
	}

	if mentions, ok := item["mentions"].([]any); ok {
		for _, m := range mentions {
			if mm, ok := m.(map[string]any); ok {
				if id := getStringField(mm, "id"); id != "" {
					record.Mentions = append(record.Mentions, id)
				}
			}
		}
	}

	return record
}

func getStringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (e *IMExtractor) classifyMessageChange(entityType, entityID, msgType, senderName, chatID, threadID, body string, timestamp int64) Change {
	var changeType, summary string

	switch msgType {
	case "text":
		changeType = "new_text"
		summary = fmt.Sprintf("[%s] %s: %s", entityTypeToLabel(entityType), senderName, truncateContent(body, 60))
	case "image":
		changeType = "new_image"
		summary = fmt.Sprintf("[%s] %s 发送了图片", entityTypeToLabel(entityType), senderName)
	case "file":
		changeType = "new_file"
		summary = fmt.Sprintf("[%s] %s 发送了文件", entityTypeToLabel(entityType), senderName)
	case "audio":
		changeType = "new_audio"
		summary = fmt.Sprintf("[%s] %s 发送了语音", entityTypeToLabel(entityType), senderName)
	case "video":
		changeType = "new_video"
		summary = fmt.Sprintf("[%s] %s 发送了视频", entityTypeToLabel(entityType), senderName)
	case "sticker":
		changeType = "new_sticker"
		summary = fmt.Sprintf("[%s] %s 发送了贴纸", entityTypeToLabel(entityType), senderName)
	case "post":
		changeType = "new_post"
		summary = fmt.Sprintf("[%s] %s 发送了富文本消息", entityTypeToLabel(entityType), senderName)
	case "interactive":
		changeType = "new_card"
		summary = fmt.Sprintf("[%s] %s 发送了卡片消息", entityTypeToLabel(entityType), senderName)
	case "share_chat":
		changeType = "new_share_chat"
		summary = fmt.Sprintf("[%s] %s 分享了群聊", entityTypeToLabel(entityType), senderName)
	case "share_user":
		changeType = "new_share_user"
		summary = fmt.Sprintf("[%s] %s 分享了名片", entityTypeToLabel(entityType), senderName)
	case "merge_forward":
		changeType = "new_merge_forward"
		summary = fmt.Sprintf("[%s] %s 转发了合并消息", entityTypeToLabel(entityType), senderName)
	default:
		changeType = "new"
		summary = fmt.Sprintf("[%s] %s: %s", entityTypeToLabel(entityType), senderName, truncateContent(body, 60))
	}

	return Change{
		Type:       changeType,
		EntityType: entityType,
		EntityID:   entityID,
		Summary:    summary,
		Timestamp:  timestamp,
		ChatID:     chatID,
		ThreadID:   threadID,
		SenderName: senderName,
		RawContent: body,
	}
}

func entityTypeToLabel(entityType string) string {
	if entityType == "group_message" {
		return "群聊"
	}
	return "双人会话"
}

// Extract 提取全量数据
func (e *IMExtractor) Extract() error {
	rawData := make(map[string]any)

	for _, chatID := range e.config.ChatIDs {
		items, err := e.getGroupMessageItems(chatID, time.Time{})
		if err == nil {
			rawData["chat_messages"] = items
		}
	}

	if e.config.UserID != "" {
		items, err := e.getP2PMessageItems(time.Time{})
		if err == nil {
			rawData["p2p_messages"] = items
		}
	}

	result := &ExtractionResult{
		Source:      e.Name(),
		ExtractedAt: time.Now(),
		RawData:     rawData,
		Formatted:   map[string]any{"extracted": true},
	}
	return SaveToJSON(e.Name(), result)
}

// ========== P2P 双人会话 ==========

func (e *IMExtractor) getP2PMessageItems(lastCheck time.Time) ([]map[string]any, error) {
	args := []string{"im", "+chat-messages-list", "--user-id", e.config.UserID, "--format", "json"}
	if !lastCheck.IsZero() {
		args = append(args, "--start", lastCheck.Format(time.RFC3339))
	}
	output, err := e.cli.RunCommand(args...)
	if err != nil {
		return nil, err
	}
	return parseChatMessageList(output)
}

// ========== 群聊 ==========

func (e *IMExtractor) getGroupMessageItems(chatID string, lastCheck time.Time) ([]map[string]any, error) {
	args := []string{"im", "+chat-messages-list", "--chat-id", chatID, "--format", "json"}
	if !lastCheck.IsZero() {
		args = append(args, "--start", lastCheck.Format(time.RFC3339))
	}
	output, err := e.cli.RunCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("getGroupMessageItems failed for %s: %w", chatID, err)
	}
	return parseChatMessageList(output)
}

// ========== 解析工具 ==========

func parseChatMessageList(output []byte) ([]map[string]any, error) {
	var list []map[string]any
	if err := json.Unmarshal(output, &list); err == nil {
		return list, nil
	}

	var wrapper map[string]any
	if err := json.Unmarshal(output, &wrapper); err != nil {
		return nil, fmt.Errorf("parseChatMessageList: invalid JSON: %s", err)
	}

	data, ok := wrapper["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parseChatMessageList: no 'data' key, top keys: %v", mapKeys(wrapper))
	}

	if msgs, ok := data["items"].([]any); ok {
		return extractMessageItems(msgs), nil
	}
	if msgs, ok := data["messages"].([]any); ok {
		return extractMessageItems(msgs), nil
	}

	return nil, fmt.Errorf("parseChatMessageList: no items/messages in data, data keys: %v", mapKeys(data))
}

func extractMessageItems(msgs []any) []map[string]any {
	var result []map[string]any
	for _, m := range msgs {
		if mm, ok := m.(map[string]any); ok {
			if del, ok := mm["deleted"].(bool); ok && del {
				continue
			}
			result = append(result, mm)
		}
	}
	return result
}

func mapKeys(m map[string]any) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func extractTimestamp(item map[string]any) int64 {
	if ct, ok := item["create_time"].(string); ok {
		if ts := parseMessageTime(ct); ts > 0 {
			return ts
		}
	}
	return 0
}

func extractBody(item map[string]any) string {
	if content, ok := item["content"].(string); ok && content != "" {
		if strings.HasPrefix(content, "{") {
			var obj map[string]any
			if err := json.Unmarshal([]byte(content), &obj); err == nil {
				if text, ok := obj["text"].(string); ok {
					return text
				}
			}
		}
		return content
	}
	if body, ok := item["body"].(map[string]any); ok {
		if content, ok := body["content"].(string); ok {
			return content
		}
	}
	return ""
}

func extractSender(item map[string]any) string {
	if sender, ok := item["sender"].(map[string]any); ok {
		if name, ok := sender["name"].(string); ok {
			return name
		}
		if id, ok := sender["id"].(string); ok {
			return id
		}
	}
	return "unknown"
}

func truncateContent(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func parseMessageTime(timeStr string) int64 {
	if ts, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return ts.Unix()
	}
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05-07:00",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, timeStr); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// ========== 长连接模式 ==========

// LongConnResultHandler 长连接结果处理器
type LongConnResultHandler func(*DetectResult) error

// StartLongConn 启动长连接监听
func (e *IMExtractor) StartLongConn(handler LongConnResultHandler) error {
	log.Printf("[IM] Starting long connection mode")

	args := []string{"event", "+subscribe", "--as", "bot"}
	cmd := exec.Command("lark-cli", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create stdout pipe failed: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe failed: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start lark-cli failed: %w", err)
	}

	// 读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[IM] [lark-cli] %s", scanner.Text())
		}
	}()

	// 读取 stdout 处理事件
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			result := e.processEventLine(line)
			if result != nil && result.HasChanges && handler != nil {
				if err := handler(result); err != nil {
					log.Printf("[IM] Handler error: %v", err)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("[IM] Scan error: %v", err)
		}
	}()

	log.Printf("[IM] Long connection started")
	return nil
}

func (e *IMExtractor) processEventLine(line string) *DetectResult {
	var event LarkEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		log.Printf("[IM] Parse event failed: %v", err)
		return nil
	}

	switch event.EventType {
	case "im.message.receive_v1", "im.message.message_receive_v1":
		return e.processIMMessageEvent(line)
	default:
		return nil
	}
}

func (e *IMExtractor) processIMMessageEvent(line string) *DetectResult {
	var msg IMMessageReceiveV1
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		log.Printf("[IM] Parse message event failed: %v", err)
		return nil
	}

	chatID := msg.Event.Message.ChatID

	// 检查是否是配置的群聊
	isWatched := false
	if len(e.config.ChatIDs) > 0 {
		isWatched = slices.Contains(e.config.ChatIDs, chatID)
	} else {
		isWatched = true // 没有配置群聊时，处理所有消息
	}

	if !isWatched {
		return nil
	}

	// 构建 MessageRecord
	record := MessageRecord{
		MessageID:  msg.Event.Message.MessageID,
		ChatID:     chatID,
		CreateTime: parseEventTime(msg.Event.Message.CreateTime),
		MsgType:    msg.Event.Message.MessageType,
		ThreadID:   msg.Event.Message.RootID,
		SenderID:   msg.Event.Sender.SenderID.OpenID,
	}

	// 解析内容
	if msg.Event.Message.Content != "" {
		var content map[string]any
		if err := json.Unmarshal([]byte(msg.Event.Message.Content), &content); err == nil {
			if text, ok := content["text"].(string); ok {
				record.Content = text
			} else {
				record.Content = msg.Event.Message.Content
			}
		} else {
			record.Content = msg.Event.Message.Content
		}
	}

	// 解析提及
	for _, m := range msg.Event.Message.Mentions {
		record.Mentions = append(record.Mentions, m.ID.OpenID)
		record.SenderName = m.Name // 使用第一个提及的名字？或者从 sender 获取
	}
	if record.SenderName == "" {
		record.SenderName = msg.Event.Sender.SenderID.OpenID
	}

	// 使用 batcher 聚合
	e.batcher.UpdateCache(chatID, []MessageRecord{record})
	groups := e.batcher.GroupMessages([]MessageRecord{record})

	var changes []Change
	for _, group := range groups {
		ctxMsg := e.batcher.BuildGroupContext(group)
		if ctxMsg != nil {
			changes = append(changes, ctxMsg.Change)
		}
	}

	// 如果没有聚合出变化，至少构建单个消息变化
	if len(changes) == 0 {
		change := e.classifyMessageChange(
			"group_message",
			record.MessageID,
			record.MsgType,
			record.SenderName,
			record.ChatID,
			record.ThreadID,
			record.Content,
			record.CreateTime,
		)
		change.SenderID = record.SenderID
		change.MentionIDs = record.Mentions
		changes = append(changes, change)
	}

	return &DetectResult{
		Source:     e.Name(),
		HasChanges: len(changes) > 0,
		DetectedAt: time.Now(),
		LastCheck:  time.Now(),
		Changes:    changes,
	}
}

func parseEventTime(timeStr string) int64 {
	if timeStr == "" {
		return time.Now().Unix()
	}
	// 飞书事件时间戳是毫秒
	if ts, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return ts.Unix()
	}
	// 尝试直接解析为毫秒时间戳
	var ms int64
	if _, err := fmt.Sscanf(timeStr, "%d", &ms); err == nil {
		return ms / 1000
	}
	return time.Now().Unix()
}
