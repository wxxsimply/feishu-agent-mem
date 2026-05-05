package larkadapter

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DocExtractor 云文档提取器
type DocExtractor struct {
	config              *Config
	cli                 *LarkCLI
	contentCache        map[string]string
	cacheLock           sync.RWMutex
	processedDocs       map[string]int64   // doc_token -> 最后处理的Unix时间戳（秒），用于检测周期间去重
	processedLock       sync.Mutex
	commentCheckTicker  int                // 周期性评论检测计数器
	commentCheckEvery   int                // 每 N 次检测扫描一次评论（默认 5）
	lastCommentCheck    int64              // 上次检查评论时的 Unix 时间戳
	processedCommentIDs map[string]bool     // 已处理的评论 ID
	docTokensWhitelist    []string           // 白名单：只检测这些 token 的文档（秒），用于评论新增判断
}

// NewDocExtractor 创建云文档提取器
func NewDocExtractor(cfg *Config) *DocExtractor {
	return &DocExtractor{
		config:             cfg,
		cli:                NewLarkCLI(),
		contentCache:       make(map[string]string),
		processedDocs:      make(map[string]int64),
		commentCheckEvery:  5,
		lastCommentCheck:   time.Now().Unix(),
	}
}

// SetCommentCheckInterval 设置评论检测周期（秒）
// SetDocTokens 设置文档检测白名单（只检测这些 token 的文档正文和评论）
func (e *DocExtractor) SetDocTokens(tokens []string) {
	if len(tokens) > 0 {
		e.docTokensWhitelist = tokens
		log.Printf("[lark_doc] Tracked doc tokens: %v", tokens)
	}
}

func (e *DocExtractor) SetCommentCheckInterval(seconds int) {
	if seconds > 0 {
		// 将秒数转换为检测次数（每次检测间隔约 30s）
		// 例如 120s -> 每 4 次检测检查一次（120/30=4）
		cycles := seconds / 30
		if cycles < 1 {
			cycles = 1
		}
		e.commentCheckEvery = cycles
		log.Printf("[lark_doc] Comment check interval set to every %d cycles (~%ds)", cycles, cycles*30)
	}
}

// Name 实现 Extractor 接口
func (e *DocExtractor) Name() string {
	return "lark_doc"
}

// Detect 检测文档变化（新增/更新/评论/权限变更等）
func (e *DocExtractor) Detect(lastCheck time.Time) (*DetectResult, error) {
	log.Printf("[lark_doc] Detect called, lastCheck: %v", lastCheck)
	changes := []Change{}

	// 尝试使用 docs +search 按时间过滤获取文档变化
	docsResult, err := e.searchDocsByTime(lastCheck)
	if err != nil {
		log.Printf("[lark_doc] searchDocsByTime failed: %v, falling back to legacy search", err)
		// 如果搜索失败，尝试原来的方式
		output, err2 := e.cli.RunCommand("docs", "+search")
		if err2 != nil {
			log.Printf("[lark_doc] legacy +search failed: %v", err2)
			result := &DetectResult{
				Source:     e.Name(),
				HasChanges: false,
				DetectedAt: time.Now(),
				LastCheck:  lastCheck,
			}
			_ = SaveDetectResult(result)
			return result, nil
		}

		var docsList []any
		if err := json.Unmarshal(output, &docsList); err != nil {
			var single any
			if err := json.Unmarshal(output, &single); err != nil {
				log.Printf("[lark_doc] failed to unmarshal legacy search result: %v", err)
				result := &DetectResult{
					Source:     e.Name(),
					HasChanges: false,
					DetectedAt: time.Now(),
					LastCheck:  lastCheck,
				}
				_ = SaveDetectResult(result)
				return result, nil
			}
			docsList = []any{single}
		}

		cutoff := lastCheck.Unix()
		log.Printf("[lark_doc] processing %d docs from legacy search, cutoff: %d", len(docsList), cutoff)
		for _, doc := range docsList {
			docChanges := e.analyzeDocChanges(doc, cutoff, lastCheck.IsZero())
			log.Printf("[lark_doc] analyzed doc, got %d changes", len(docChanges))
			changes = append(changes, docChanges...)
		}
	} else {
		// 使用搜索到的结果
		log.Printf("[lark_doc] got %d changes from searchDocsByTime", len(docsResult))
		changes = append(changes, docsResult...)
	}

	result := &DetectResult{
		Source:     e.Name(),
		HasChanges: len(changes) > 0,
		DetectedAt: time.Now(),
		LastCheck:  lastCheck,
		Changes:    changes,
	}

	log.Printf("[lark_doc] Detect finished, HasChanges: %v, Changes: %d", result.HasChanges, len(changes))
	for i, ch := range changes {
		log.Printf("[lark_doc] Change[%d]: %s (Type: %s, EntityID: %s)", i, ch.Summary, ch.Type, ch.EntityID)
	}

	if err := SaveDetectResult(result); err != nil {
		log.Printf("[lark_doc] failed to save DetectResult: %v", err)
	}
	return result, nil
}

// searchDocsByTime 使用 docs +search 按时间过滤搜索文档
func (e *DocExtractor) searchDocsByTime(lastCheck time.Time) ([]Change, error) {
	log.Printf("[lark_doc] searchDocsByTime called, lastCheck: %v", lastCheck)
	var changes []Change

	// 直接搜索所有文档（不使用filter，避免时间格式问题）
	log.Printf("[lark_doc] calling docs +search")
	output, err := e.cli.RunCommand("docs", "+search")
	if err != nil {
		log.Printf("[lark_doc] docs +search failed: %v", err)
		return changes, err
	}

	// 解析结果
	var searchResult map[string]any
	if err := json.Unmarshal(output, &searchResult); err != nil {
		log.Printf("[lark_doc] failed to unmarshal +search result: %v", err)
		return changes, err
	}

	// 解析 data.results（实际返回结构）
	var results []any
	if data, ok := searchResult["data"].(map[string]any); ok {
		if items, ok := data["results"].([]any); ok {
			results = items
		}
	}
	log.Printf("[lark_doc] got %d results from +search", len(results))

	cutoff := lastCheck.Unix()
	filteredCount := 0
	dedupSkipped := 0

	// 清理超过30分钟的processedDocs记录
	e.processedLock.Lock()
	for token, ts := range e.processedDocs {
		if ts < cutoff-1800 {
			delete(e.processedDocs, token)
		}
	}
	e.processedLock.Unlock()

	for _, item := range results {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		// 先检查时间是否在lastCheck之后
		var updateTimestamp int64
		var createTimestamp int64
		var docToken string
		if resultMeta, ok := itemMap["result_meta"].(map[string]any); ok {
			if ut, ok := resultMeta["update_time"].(float64); ok {
				updateTimestamp = int64(ut)
			}
			if ct, ok := resultMeta["create_time"].(float64); ok {
				createTimestamp = int64(ct)
			}
			docToken, _ = resultMeta["token"].(string)
		}
		// 取较新的时间
		checkTimestamp := updateTimestamp
		if createTimestamp > checkTimestamp {
			checkTimestamp = createTimestamp
		}
		// 比较时间
		if checkTimestamp == 0 || checkTimestamp <= cutoff {
			continue
		}
		// 文档级别去重：同一token在同一秒或更早被处理过 → 跳过
		if docToken != "" {
			e.processedLock.Lock()
			lastSeen, exists := e.processedDocs[docToken]
			if exists && checkTimestamp <= lastSeen {
				e.processedLock.Unlock()
				dedupSkipped++
				continue
			}
			// 更新记录
			if checkTimestamp > lastSeen {
				e.processedDocs[docToken] = checkTimestamp
			}
			e.processedLock.Unlock()
		}

		filteredCount++
		change := e.parseSearchItemToChange(itemMap)
		if change.Type != "" {
			changes = append(changes, change)
		}
	}

	log.Printf("[lark_doc] filtered out %d old results, %d dedup skipped, %d new changes to process",
		len(results)-filteredCount-dedupSkipped, dedupSkipped, len(changes))

	// 周期性评论检测（每 commentCheckEvery 次检测执行一次）
	e.commentCheckTicker++
	if e.commentCheckTicker >= e.commentCheckEvery {
		e.commentCheckTicker = 0
		commentCutoff := e.lastCommentCheck
		e.lastCommentCheck = time.Now().Unix()
		e.processedCommentIDs = make(map[string]bool)
		log.Printf("[lark_doc] Periodic comment check starting (since cutoff=%d)...", commentCutoff)
		commentChanges := e.detectNewComments(results, commentCutoff)
		if len(commentChanges) > 0 {
			log.Printf("[lark_doc] Found %d new comment changes", len(commentChanges))
			changes = append(changes, commentChanges...)
		}
	}

	return changes, nil
}

// resolveActualDocToken 从 result_meta 中解析实际文档 token（Wiki 节点需先解析）
func (e *DocExtractor) resolveActualDocToken(resultMeta map[string]any, fallbackToken string) string {
	if iconInfoStr, ok := resultMeta["icon_info"].(string); ok && iconInfoStr != "" {
		var iconInfo struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal([]byte(iconInfoStr), &iconInfo); err == nil && iconInfo.Token != "" {
			return iconInfo.Token
		}
	}
	return fallbackToken
}

// detectNewComments 从搜索结果中检测新评论
func (e *DocExtractor) detectNewComments(results []any, cutoff int64) []Change {
	var changes []Change

	for _, item := range results {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		resultMeta, ok := itemMap["result_meta"].(map[string]any)
		if !ok {
			continue
		}
		token, _ := resultMeta["token"].(string)
		if token == "" {
			continue
		}

		// 解析实际文档 token
		docToken := e.resolveActualDocToken(resultMeta, token)

		// 白名单过滤
		if len(e.docTokensWhitelist) > 0 {
			if !stringSliceContains(e.docTokensWhitelist, docToken) {
				continue
			}
		}

		// 获取文档评论
		comments, err := e.FetchDocumentComments(docToken)
		if err != nil {
			log.Printf("[lark_doc] Failed to fetch comments for %s: %v", docToken, err)
			continue
		}

		// 检查是否有新评论（创建时间 > cutoff）
		for _, c := range comments {
			if c.CreatedAt > cutoff && c.Text != "" {
				title, _ := itemMap["title_highlighted"].(string)
				author := c.Author
				if author == "" {
					if v, ok := resultMeta["edit_user_name"].(string); ok {
						author = v
					}
				}

				// 侧边评论带上引用内容，全文评论不带引用
				summary := fmt.Sprintf("%s新评论: %s (by %s)", title, c.Text, author)
				if !c.IsWhole && c.Quote != "" {
					summary = fmt.Sprintf("%s新评论: %s (引用: %s) (by %s)", title, c.Text, c.Quote, author)
				}
				change := Change{
					Type:       "doc_comment_added",
					EntityType: "doc",
					EntityID:   token,
					Summary:    summary,
					Timestamp:  c.CreatedAt,
				}
				if docToken != token {
					change.Meta = map[string]string{"actual_doc_token": docToken}
				}
				changes = append(changes, change)
			}
		}
	}

	return changes
}

// parseSearchItemToChange 将搜索结果项转换为 Change
func (e *DocExtractor) parseSearchItemToChange(item map[string]any) Change {
	// 提取基本信息
	title, _ := item["title_highlighted"].(string)

	// 提取 result_meta（实际元数据在这里）
	var token string
	var timestamp int64
	var author string
	var docTypes string
	var actualDocToken string
	if resultMeta, ok := item["result_meta"].(map[string]any); ok {
		token, _ = resultMeta["token"].(string)

		// 提取时间
		if updateTime, ok := resultMeta["update_time"].(float64); ok {
			timestamp = int64(updateTime)
		} else if createTime, ok := resultMeta["create_time"].(float64); ok {
			timestamp = int64(createTime)
		}

		// 提取作者
		author, _ = resultMeta["owner_name"].(string)
		if author == "" {
			author, _ = resultMeta["edit_user_name"].(string)
		}

		// 提取文档类型
		docTypes, _ = resultMeta["doc_types"].(string)

		// 提取实际 doc token（Wiki 节点的 token 可能与实际 doc token 不同）
		if iconInfoStr, ok := resultMeta["icon_info"].(string); ok && iconInfoStr != "" {
			var iconInfo struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal([]byte(iconInfoStr), &iconInfo); err == nil && iconInfo.Token != "" {
				actualDocToken = iconInfo.Token
			}
		}
	}

	// 确定变化类型
	changeType := "doc_updated"
	summary := fmt.Sprintf("文档更新: %s", title)

	// 检查标题是否包含决策关键词
	if containsDecisionKeyword(title) {
		changeType = "doc_decision"
		summary = fmt.Sprintf("决策文档更新: %s", title)
	}

	// 获取文档类型标签
	typeLabel := e.getDocTypeLabel(docTypes)
	if typeLabel != "" {
		summary = fmt.Sprintf("%s更新: %s", typeLabel, title)
		if changeType == "doc_decision" {
			summary = fmt.Sprintf("决策%s更新: %s", typeLabel, title)
		}
	}

	// 补充作者信息
	if author != "" {
		summary = fmt.Sprintf("%s (by %s)", summary, author)
	}

	meta := map[string]string{}
	if actualDocToken != "" {
		meta["actual_doc_token"] = actualDocToken
	}

	return Change{
		Type:       changeType,
		EntityType: "doc",
		EntityID:   token,
		Summary:    summary,
		Timestamp:  timestamp,
		Meta:       meta,
	}
}

// FetchDocumentContent 获取文档内容
func (e *DocExtractor) FetchDocumentContent(docToken string) (string, error) {
	output, err := e.cli.RunCommand("docs", "+fetch", "--doc", docToken)
	if err != nil {
		return "", err
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	if data, ok := result["data"].(map[string]any); ok {
		if markdown, ok := data["markdown"].(string); ok {
			return markdown, nil
		}
	}

	return "", fmt.Errorf("could not extract markdown content")
}

// FetchDocumentComments 获取文档评论
// 使用 lark-cli drive file.comments list 获取评论列表，返回评论内容列表
func (e *DocExtractor) FetchDocumentComments(docToken string) ([]CommentEntry, error) {
	output, err := e.cli.RunCommand(
		"drive", "file.comments", "list",
		"--params", fmt.Sprintf(`{"file_token":"%s","file_type":"docx"}`, docToken),
		"--as", "user",
		"--format", "json",
	)
	if err != nil {
		// API error 1069307 = "not exist" 表示该文档没有评论，不是真正的错误
		if strings.Contains(err.Error(), "1069307") {
			log.Printf("[Doc] No comments for doc %s", docToken)
			return []CommentEntry{}, nil
		}
		log.Printf("[Doc] Failed to fetch comments for %s: %v", docToken, err)
		return nil, err
	}

	var resp struct {
		Data struct {
			Items []json.RawMessage `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &resp); err != nil {
		log.Printf("[Doc] Failed to parse comments response: %v", err)
		return nil, err
	}

	var comments []CommentEntry
	for _, raw := range resp.Data.Items {
		comment := e.parseCommentItem(raw)
		if comment.Text != "" {
			comments = append(comments, comment)
		}
	}

	log.Printf("[Doc] Fetched %d comments for doc %s", len(comments), docToken)
	return comments, nil
}

// CommentEntry 文档评论条目
type CommentEntry struct {
	CommentID string `json:"comment_id"`
	Text      string `json:"text"`      // 最新一条回复的文本
	Author    string `json:"author"`    // 回复者姓名
	Quote     string `json:"quote"`     // 被引用的内容片段
	IsWhole   bool   `json:"is_whole"`  // 是否为全文评论
	CreatedAt int64  `json:"created_at"`
}

// parseCommentItem 解析单个评论条目，提取最新回复文本
func (e *DocExtractor) parseCommentItem(raw json.RawMessage) CommentEntry {
	var item struct {
		CommentID string `json:"comment_id"`
		IsWhole   bool   `json:"is_whole"`
		Quote     string `json:"quote"`
		CreatedAt int64  `json:"create_time"`
		ReplyList struct {
			Replies []json.RawMessage `json:"replies"`
		} `json:"reply_list"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return CommentEntry{}
	}

	entry := CommentEntry{
		CommentID: item.CommentID,
		Quote:     item.Quote,
		IsWhole:   item.IsWhole,
		CreatedAt: item.CreatedAt,
	}

	// 提取最新一条回复的文本和作者
	if len(item.ReplyList.Replies) > 0 {
		lastReply := item.ReplyList.Replies[len(item.ReplyList.Replies)-1]
		entry.Text = e.extractReplyText(lastReply)
		entry.Author = e.extractReplyAuthor(lastReply)
	}

	return entry
}

// extractReplyText 从回复中提取文本内容
func (e *DocExtractor) extractReplyText(raw json.RawMessage) string {
	var reply struct {
		Content struct {
			Elements []struct {
				TextRun *struct {
					Text string `json:"text"`
				} `json:"text_run,omitempty"`
			} `json:"elements"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &reply); err != nil {
		return ""
	}
	var text string
	for _, elem := range reply.Content.Elements {
		if elem.TextRun != nil {
			text += elem.TextRun.Text
		}
	}
	return text
}

// extractReplyAuthor 从回复中提取作者
func (e *DocExtractor) extractReplyAuthor(_ json.RawMessage) string {
	// 评论 API 的回复结构中暂不直接返回作者名，留空由调用方补充
	return ""
}

// CompareDocumentContent 比较文档内容变化
func (e *DocExtractor) CompareDocumentContent(docToken, oldContent, newContent string) (string, []diffmatchpatch.Diff) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)
	prettyDiff := dmp.DiffPrettyText(diffs)
	return prettyDiff, diffs
}

// GetDocumentContentDiff 获取文档内容变化
func (e *DocExtractor) GetDocumentContentDiff(docToken string) (string, []diffmatchpatch.Diff, error) {
	// 获取当前内容
	newContent, err := e.FetchDocumentContent(docToken)
	if err != nil {
		return "", nil, err
	}

	// 获取缓存的旧内容
	e.cacheLock.RLock()
	oldContent, hasOld := e.contentCache[docToken]
	e.cacheLock.RUnlock()

	// 如果没有旧内容，缓存当前内容并返回
	if !hasOld {
		e.cacheLock.Lock()
		e.contentCache[docToken] = newContent
		e.cacheLock.Unlock()
		return "", nil, nil
	}

	// 更新缓存
	e.cacheLock.Lock()
	e.contentCache[docToken] = newContent
	e.cacheLock.Unlock()

	// 比较内容
	prettyDiff, diffs := e.CompareDocumentContent(docToken, oldContent, newContent)
	return prettyDiff, diffs, nil
}

// HasContentChanged 检查文档内容是否有变化
func (e *DocExtractor) HasContentChanged(docToken, oldContent, newContent string) bool {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)

	for _, diff := range diffs {
		if diff.Type != diffmatchpatch.DiffEqual {
			return true
		}
	}
	return false
}

// GetContentChangeSummary 获取内容变化摘要
func (e *DocExtractor) GetContentChangeSummary(diffs []diffmatchpatch.Diff) map[string]int {
	changes := map[string]int{
		"added":   0,
		"deleted": 0,
		"changed": 0,
	}

	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			changes["added"] += len(diff.Text)
		case diffmatchpatch.DiffDelete:
			changes["deleted"] += len(diff.Text)
		}
	}

	if changes["added"] > 0 && changes["deleted"] > 0 {
		changes["changed"] = changes["added"] + changes["deleted"]
	}

	return changes
}

// analyzeDocChanges 分析文档的详细变化类型
func (e *DocExtractor) analyzeDocChanges(doc interface{}, cutoff int64, isFirstCheck bool) []Change {
	var changes []Change

	// 如果是第一次检测，跳过初始扫描
	if isFirstCheck {
		return changes
	}

	docMap, ok := doc.(map[string]any)
	if !ok {
		return changes
	}

	docToken, _ := docMap["doc_token"].(string)
	title, _ := docMap["title"].(string)
	objType, _ := docMap["obj_type"].(string)

	if docToken == "" && title == "" {
		return changes
	}
	if docToken == "" {
		docToken = title
	}

	// 获取文档时间戳
	docTime := e.getDocTimestamp(docMap)
	if docTime == 0 {
		return changes
	}

	// 只处理最近的变化
	if docTime <= cutoff {
		return changes
	}

	// 确定文档类型标签
	typeLabel := e.getDocTypeLabel(objType)

	// 检测不同的时间字段来判断变化类型
	changeTypes := e.detectChangeTypes(docMap, cutoff)

	for _, ct := range changeTypes {
		var changeType, summary string
		switch ct {
		case "created":
			changeType = "doc_created"
			summary = fmt.Sprintf("新建%s: %s", typeLabel, title)
		case "content_updated":
			changeType = "doc_content_updated"
			summary = fmt.Sprintf("%s内容更新: %s", typeLabel, title)
		case "comment_added":
			changeType = "doc_comment_added"
			summary = fmt.Sprintf("%s新评论: %s", typeLabel, title)
		case "permission_changed":
			changeType = "doc_permission_changed"
			summary = fmt.Sprintf("%s权限变更: %s", typeLabel, title)
		default:
			changeType = "doc_updated"
			summary = fmt.Sprintf("%s更新: %s", typeLabel, title)
		}

		// 检查是否包含决策关键词
		if containsDecisionKeyword(title) {
			changeType = "doc_decision"
			summary = fmt.Sprintf("决策%s: %s", typeLabel, title)
		}

		changes = append(changes, Change{
			Type:       changeType,
			EntityType: "doc",
			EntityID:   docToken,
			Summary:    summary,
			Timestamp:  docTime,
		})
	}

	return changes
}

// getDocTypeLabel 获取文档类型的可读标签
func (e *DocExtractor) getDocTypeLabel(objType string) string {
	switch objType {
	case "doc":
		return "文档"
	case "docx":
		return "新文档"
	case "sheet":
		return "电子表格"
	case "bitable":
		return "多维表格"
	case "mindnote":
		return "思维导图"
	case "file":
		return "文件"
	case "slides":
		return "幻灯片"
	case "wiki":
		return "知识库"
	case "folder":
		return "文件夹"
	default:
		return "文档"
	}
}

// getDocTimestamp 获取文档的时间戳
func (e *DocExtractor) getDocTimestamp(doc map[string]any) int64 {
	// 尝试不同的时间字段
	for _, key := range []string{"edit_time", "update_time", "modified_time", "create_time", "created_at"} {
		if v, ok := doc[key]; ok {
			switch val := v.(type) {
			case float64:
				return int64(val)
			case int64:
				return val
			case string:
				t, err := time.Parse(time.RFC3339, val)
				if err == nil {
					return t.Unix()
				}
				// 尝试解析 Unix 时间戳字符串
				if ts, err := time.Parse(time.UnixDate, val); err == nil {
					return ts.Unix()
				}
			}
		}
	}
	return 0
}

// detectChangeTypes 检测文档的变化类型
func (e *DocExtractor) detectChangeTypes(doc map[string]any, cutoff int64) []string {
	var types []string

	// 检查更新时间
	for _, key := range []string{"edit_time", "update_time", "modified_time"} {
		if v, ok := doc[key]; ok {
			var ts int64
			switch val := v.(type) {
			case float64:
				ts = int64(val)
			case int64:
				ts = val
			case string:
				t, err := time.Parse(time.RFC3339, val)
				if err == nil {
					ts = t.Unix()
				}
			}
			if ts > cutoff {
				types = append(types, "content_updated")
				break
			}
		}
	}

	// 检查创建时间
	if len(types) == 0 {
		for _, key := range []string{"create_time", "created_at"} {
			if v, ok := doc[key]; ok {
				var ts int64
				switch val := v.(type) {
				case float64:
					ts = int64(val)
				case int64:
					ts = val
				case string:
					t, err := time.Parse(time.RFC3339, val)
					if err == nil {
						ts = t.Unix()
					}
				}
				if ts > cutoff {
					types = append(types, "created")
					break
				}
			}
		}
	}

	// 如果没有检测到具体变化，使用默认
	if len(types) == 0 {
		types = append(types, "default")
	}

	return types
}

// Extract 提取云文档决策信息
func (e *DocExtractor) Extract() error {
	rawData := make(map[string]any)

	if docs, err := e.searchDocs(); err == nil {
		rawData["documents"] = docs
	}

	formatted := map[string]any{
		"extracted": true,
	}

	result := &ExtractionResult{
		Source:      e.Name(),
		ExtractedAt: time.Now(),
		RawData:     rawData,
		Formatted:   formatted,
	}

	if err := SaveToJSON(e.Name(), result); err != nil {
		return fmt.Errorf("save result failed: %w", err)
	}

	return nil
}

func (e *DocExtractor) searchDocs() ([]any, error) {
	output, err := e.cli.RunCommand("docs", "+search")
	if err != nil {
		return nil, err
	}

	var result []any
	if err := json.Unmarshal(output, &result); err != nil {
		var single any
		if err := json.Unmarshal(output, &single); err != nil {
			return nil, err
		}
		result = []any{single}
	}
	return result, nil
}

func (e *DocExtractor) isDocChanged(doc map[string]any, cutoff int64, isFirstCheck bool) bool {
	// 如果是第一次检测，跳过初始扫描（避免大量初始变化）
	if isFirstCheck {
		return false
	}

	// 检查更新时间
	for _, key := range []string{"edit_time", "update_time", "modified_time"} {
		if v, ok := doc[key]; ok {
			switch val := v.(type) {
			case float64:
				if int64(val) > cutoff {
					return true
				}
			case int64:
				if val > cutoff {
					return true
				}
			case string:
				t, err := time.Parse(time.RFC3339, val)
				if err == nil && t.Unix() > cutoff {
					return true
				}
			}
		}
	}
	return false
}

// decisionKeywords 决策关键词列表
var decisionKeywords = []string{
	"决定", "确认", "结论", "通过", "定下来", "决策", "决议", "评审",
	"批准", "同意", "达成共识", "确定", "采纳", "批准", "通过", "方案",
	"决定", "approve", "decided", "confirmed", "decision", "resolution",
	"review", "agree", "conclusion", "finalize", "OKR", "KPI", "里程碑",
}

// containsDecisionKeyword 检查文本是否包含决策关键词
func containsDecisionKeyword(text string) bool {
	lowerText := strings.ToLower(text)
	for _, kw := range decisionKeywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// stringSliceContains 检查字符串切片中是否包含目标
func stringSliceContains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
