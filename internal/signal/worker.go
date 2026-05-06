package signal

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	larkadapter "feishu-mem/internal/lark-adapter"
)

// DetectionJob 检测任务
type DetectionJob struct {
	AdapterType AdapterType
	Change      larkadapter.Change
	ReceivedAt  time.Time
}

// DecisionResult 决策处理结果
type DecisionResult struct {
	Job       *DetectionJob
	Mutation  *DecisionMutation
	PendingMutations []*DecisionMutation // 附加变更（如反对意见），在 pipeline 中一并处理
	Processed time.Time
	Err       error
}

// WorkerPool 工作池
type WorkerPool struct {
	jobChan         chan *DetectionJob
	resultChan      chan *DecisionResult
	wg              sync.WaitGroup
	engine          *SignalActivationEngine
	maxWorkers      int
	stats           *WorkerStats
	debounceTracker *larkadapter.DocDebounceTracker
}

// WorkerStats 工作统计
type WorkerStats struct {
	ActiveWorkers int
	JobsProcessed int64
	LLMCalls      int64
	mu            sync.Mutex
}

// NewWorkerPool 创建工作池
func NewWorkerPool(engine *SignalActivationEngine, maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}

	return &WorkerPool{
		jobChan:    make(chan *DetectionJob, 100),
		resultChan: make(chan *DecisionResult, 100),
		engine:     engine,
		maxWorkers: maxWorkers,
		stats:      &WorkerStats{},
	}
}

// Start 启动工作池
func (wp *WorkerPool) Start() {
	log.Printf("[WorkerPool] Starting %d workers...", wp.maxWorkers)
	for i := 0; i < wp.maxWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// SetDebounceTracker 设置防抖追踪器
func (wp *WorkerPool) SetDebounceTracker(tracker *larkadapter.DocDebounceTracker) {
	wp.debounceTracker = tracker
}

// worker 工作协程
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	log.Printf("[Worker-%d] Started", id)

	for job := range wp.jobChan {
		wp.stats.mu.Lock()
		wp.stats.ActiveWorkers++
		wp.stats.mu.Unlock()

		log.Printf("[Worker-%d] Processing job for %s: %s", id, job.Change.EntityType, job.Change.Summary)

		result := wp.processJob(job)

		wp.stats.mu.Lock()
		wp.stats.ActiveWorkers--
		wp.stats.JobsProcessed++
		wp.stats.mu.Unlock()

		wp.resultChan <- result
	}

	log.Printf("[Worker-%d] Stopped", id)
}

// processJob 处理单个任务
func (wp *WorkerPool) processJob(job *DetectionJob) *DecisionResult {
	log.Println("========== WORKER PROCESS START ==========")
	result := &DecisionResult{
		Job:       job,
		Processed: time.Now(),
	}

	log.Printf("[Worker] AdapterType: %s", job.AdapterType)
	log.Printf("[Worker] Change.Type: %s", job.Change.Type)
	log.Printf("[Worker] Change.Summary: %s", truncateForLog(job.Change.Summary, 300))

	switch job.AdapterType {
	case AdapterIM:
		return wp.processIMJob(job, result)
	case AdapterVC:
		return wp.processVCJob(job, result)
	case AdapterDocs:
		return wp.processDocsJob(job, result)
	case AdapterCalendar:
		return wp.processCalendarJob(job, result)
	case AdapterTask:
		return wp.processTaskJob(job, result)
	case AdapterWiki:
		return wp.processWikiJob(job, result)
	default:
		log.Println("[Worker] Unknown adapter type, skipping")
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}
}

// processIMJob 处理 IM 类型任务 — 使用增强型多因子检测 + 上下文
func (wp *WorkerPool) processIMJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	if job.Change.Type != "new_text" && job.Change.Type != "new_post" {
		log.Println("[Worker] Not a text message, skipping")
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	if wp.engine.detector == nil {
		wp.engine.detector = NewEnhancedDetector()
	}

	// 使用 Change 中的上下文字段构建真实 DetectContext
	change := job.Change
	hasThread := change.ThreadID != ""
	hasMentions := len(change.MentionIDs) > 0
	// 优先使用拼接上下文文本（含历史讨论），其次原文，最后摘要
	content := change.ContextText
	if content == "" {
		content = change.RawContent
	}
	if content == "" {
		content = change.Summary
	}
	if change.ContextText != "" {
		log.Printf("[Worker] Using context text (%d chars)", len(change.ContextText))
	} else if change.RawContent != "" {
		log.Printf("[Worker] Using raw content (%d chars)", len(change.RawContent))
	}

	ctx := &DetectContext{
		IsReply:      hasThread,
		HasMention:   hasMentions,
		MessageIndex: 1,
		SenderName:   change.SenderName,
	}
	if ctx.SenderName == "" {
		ctx.SenderName = extractSenderFromSummary(change.Summary)
	}

	if hasThread {
		log.Printf("[Worker] Message is in thread %s", change.ThreadID)
	}
	if hasMentions {
		log.Printf("[Worker] Message mentions %d user(s): %v", len(change.MentionIDs), change.MentionIDs)
	}

	log.Printf("[Worker] Running enhanced decision detection on: %s", truncateForLog(content, 100))

	// 多因子检测（使用原文而非截断的 summary）
	detectResult := wp.engine.detector.Analyze(content, ctx)
	log.Printf("[Worker] Detection score=%.2f, level=%s, signals=%d, anti=%d",
		detectResult.Score, detectResult.Level,
		len(detectResult.SignalDetails), len(detectResult.AntiSignals))

	if detectResult.Factors != nil {
		log.Printf("[Worker]  Breakdown: lexical=%.2f structural=%.2f dynamic=%.2f pattern=%.2f anti=%.2f",
			detectResult.Factors.Lexical, detectResult.Factors.Structural,
			detectResult.Factors.Dynamic, detectResult.Factors.Pattern,
			detectResult.Factors.AntiScore)
	}

	switch detectResult.Level {
	case LevelHigh:
		log.Printf("[Worker] High-confidence decision signal (score=%.2f)", detectResult.Score)
	case LevelMedium:
		log.Printf("[Worker] Medium-confidence decision signal (score=%.2f), will process", detectResult.Score)
	case LevelLow:
		log.Printf("[Worker] Low-confidence signal (score=%.2f), skipping", detectResult.Score)
		log.Println("========== WORKER PROCESS END ==========")
		return result
	case LevelNone:
		log.Println("[Worker] No decision signal detected, skipping")
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	var signalNames []string
	for _, s := range detectResult.SignalDetails {
		signalNames = append(signalNames, s.Name)
	}

	signalStrength := StrengthMedium
	if detectResult.Level == LevelHigh {
		signalStrength = StrengthStrong
	}

	// 创建信号（使用完整原文）
	sig := NewSignal(job.AdapterType, content)
	sig.Strength = signalStrength
	sig.Context.ContentSnippet = content
	sig.Context.Keywords = signalNames
	sig.Context.DecisionSignals = signalNames
	sig.Context.Score = detectResult.Score

	// 通过信号引擎处理（传递完整内容而非截断的 summary）
	proposer := change.SenderName
	if proposer == "" {
		proposer = extractSenderFromSummary(change.Summary)
	}
	mut, pendingMuts, err := wp.engine.ProcessSignalForJob(sig, proposer, content)
	if err != nil {
		log.Printf("[Worker] Error processing signal: %v", err)
		result.Err = err
		return result
	}

	result.Mutation = mut
	result.PendingMutations = append(result.PendingMutations, pendingMuts...)
	return result
}

// processVCJob 处理 VC 类型任务
func (wp *WorkerPool) processVCJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	if job.Change.Type == "meeting_minutes_available" || job.Change.Type == "meeting_todos" || job.Change.Type == "minute_created" ||
		job.Change.Type == "minutes_created" || job.Change.Type == "minutes_updated" || job.Change.Type == "minutes_ai_summary_ready" {
		sig := NewSignal(job.AdapterType, job.Change.Summary)
		sig.Strength = StrengthStrong
		sig.Context.ContentSnippet = job.Change.Summary

		proposer := "会议系统"
		mut, pendingMuts, err := wp.engine.ProcessSignalForJob(sig, proposer, job.Change.Summary)
		if err != nil {
			log.Printf("[Worker] Error processing VC signal: %v", err)
			result.Err = err
			return result
		}
		result.Mutation = mut
	result.PendingMutations = append(result.PendingMutations, pendingMuts...)
	}

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processDocsJob 处理 Docs 类型任务 — 4 阶段分阶段分析
func (wp *WorkerPool) processDocsJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	// 只处理文档内容变更类型
	contentTypes := map[string]bool{
		"doc_decision":         true,
		"doc_updated":          true,
		"doc_content_updated":  true,
		"doc_created":          true,
		"doc_comment_added":    true,
	}
	if !contentTypes[job.Change.Type] {
		log.Println("[Worker] Not a document content change, skipping")
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	docToken := job.Change.EntityID
	if docToken == "" {
		log.Println("[Worker] No document token, skipping")
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	// 评论变更：不获取完整文档内容，直接用评论摘要送 LLM
	if job.Change.Type == "doc_comment_added" {
		log.Printf("[Worker] Processing document comment: %s", job.Change.Summary)
		return wp.processDocComment(job, result, docToken)
	}

	// Phase 1: Context Gathering — 获取文档内容
	cfg := larkadapter.LoadConfig()
	docExt := larkadapter.NewDocExtractor(cfg)

	content, err := docExt.FetchDocumentContent(docToken)
	if err != nil {
		log.Printf("[Worker] Failed to fetch doc content: %v, skipping docToken=%s", err, docToken)
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	title := extractDocTitleFromSummary(job.Change.Summary)
	log.Printf("[Worker] Doc content fetched: title=%s, content_len=%d", title, len(content))

	// 使用 go-diff 比较内容变化，只提取变更部分
	diffContent, diffs, _ := docExt.GetDocumentContentDiff(docToken)
	if diffContent != "" {
		log.Printf("[Worker] Content diff found: %d changes", len(diffs))
		content = diffContent
		if len(content) > 3000 {
			content = content[:3000] + "\n...（diff已截断）"
		}
	} else {
		// 首次检测（无缓存），取文档开头部分（决策通常在开头）
		log.Printf("[Worker] No cached content, using document head")
		if len(content) > 3000 {
			content = content[:3000] + "\n...（文档后面已省略）"
		}
	}

	// 获取文档评论（用于反对意见提取）
	actualDocToken := docToken
	if tok, ok := job.Change.Meta["actual_doc_token"]; ok && tok != "" {
		actualDocToken = tok
	}
	comments, _ := docExt.FetchDocumentComments(actualDocToken)
	if len(comments) > 0 {
		log.Printf("[Worker] Fetched %d comments for doc %s", len(comments), actualDocToken)
		var commentTexts []string
		for _, c := range comments {
			if c.Text != "" {
				commentTexts = append(commentTexts, fmt.Sprintf("[comment by %s] %s (quote: %s)", c.Author, c.Text, c.Quote))
			}
		}
		if len(commentTexts) > 0 {
			content += "\n\n## 文档评论\n" + strings.Join(commentTexts, "\n")
		}
	}

	// 直接送 LLM 判断，不经过本地 pattern 过滤
	docType := classifyDocType(title, content)

	// Phase 3-4: LLM Extraction + Output
	sig := NewSignal(job.AdapterType, job.Change.Summary)
	sig.PrimaryID = docToken
	sig.Strength = StrengthStrong
	sig.Context.ContentSnippet = truncateForLog(content, 1000)
	sig.Context.IsDecision = true

	// 提取前 3000 字符送 LLM 分析（决策通常在文档开头）
	analysisContent := content
	if len(analysisContent) > 3000 {
		analysisContent = analysisContent[:3000] + "\n...（后面已截断）"
	}

	proposer := "文档系统"
	mut, pendingMuts, err := wp.engine.ProcessSignalForDocJob(sig, proposer, analysisContent, string(docType), title)
	if err != nil {
		log.Printf("[Worker] Error processing doc signal: %v", err)
		result.Err = err
		return result
	}
	result.Mutation = mut
	result.PendingMutations = append(result.PendingMutations, pendingMuts...)

	// 如果成功提取到决策（main 或 pending），标记文档已处理（防抖用）
	if wp.debounceTracker != nil && (mut != nil || len(pendingMuts) > 0) {
		actualDocToken := docToken
		if tok, ok := job.Change.Meta["actual_doc_token"]; ok && tok != "" {
			actualDocToken = tok
		}
		contentHash := larkadapter.ComputeContentHash(content)
		wp.debounceTracker.MarkProcessed(actualDocToken, contentHash)
		log.Printf("[Debounce] Marked doc %s as processed (hash: %s, main=%v, pending=%d)",
			actualDocToken, contentHash[:16]+"...", mut != nil, len(pendingMuts))
	}

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processCalendarJob 处理 Calendar 类型任务
func (wp *WorkerPool) processCalendarJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	log.Printf("[Worker] Processing calendar event: %s", job.Change.Summary)
	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processTaskJob 处理 Task 类型任务
func (wp *WorkerPool) processTaskJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	if job.Change.Type == "task_completed" {
		sig := NewSignal(job.AdapterType, job.Change.Summary)
		sig.Strength = StrengthMedium
		sig.Context.ContentSnippet = job.Change.Summary
		sig.Context.DecisionSignals = []string{"task_done"}

		proposer := "任务系统"
		mut, pendingMuts, err := wp.engine.ProcessSignalForJob(sig, proposer, job.Change.Summary)
		if err != nil {
			log.Printf("[Worker] Error processing Task signal: %v", err)
			result.Err = err
			return result
		}
		result.Mutation = mut
	result.PendingMutations = append(result.PendingMutations, pendingMuts...)
	}

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processDocComment 处理文档评论（反对意见来源之一）
func (wp *WorkerPool) processDocComment(job *DetectionJob, result *DecisionResult, docToken string) *DecisionResult {
	sig := NewSignal(job.AdapterType, job.Change.Summary)
	sig.PrimaryID = docToken
	sig.Strength = StrengthMedium
	sig.Context.ContentSnippet = job.Change.Summary
	sig.CommentID = job.Change.CommentID

	proposer := "文档系统"
	mut, pendingMuts, err := wp.engine.ProcessSignalForDocJob(sig, proposer, job.Change.Summary, "comment", "")
	if err != nil {
		log.Printf("[Worker] Error processing doc comment signal: %v", err)
		result.Err = err
		return result
	}
	result.Mutation = mut
	result.PendingMutations = append(result.PendingMutations, pendingMuts...)

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processWikiJob 处理 Wiki 类型任务 — 4 阶段分阶段分析
func (wp *WorkerPool) processWikiJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	// 只处理节点内容变更
	if job.Change.Type != "updated" && job.Change.Type != "new" {
		log.Println("[Worker] Not a wiki content change, skipping")
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	nodeToken := job.Change.EntityID
	if nodeToken == "" {
		log.Println("[Worker] No wiki node token, skipping")
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	// Phase 1: Context Gathering — 获取知识库节点内容
	cfg := larkadapter.LoadConfig()
	wikiExt := larkadapter.NewWikiExtractor(cfg)

	content, err := wikiExt.FetchWikiNodeContent(nodeToken)
	if err != nil {
		log.Printf("[Worker] Failed to fetch wiki content: %v, skipping nodeToken=%s", err, nodeToken)
		log.Println("========== WORKER PROCESS END ==========")
		return result
	}

	title := extractDocTitleFromSummary(job.Change.Summary)
	log.Printf("[Worker] Wiki content fetched: title=%s, content_len=%d", title, len(content))

	// 使用 go-diff 比较内容变化，只提取变更部分
	diffContent, diffs, _ := wikiExt.GetWikiNodeContentDiff(nodeToken)
	if diffContent != "" {
		log.Printf("[Worker] Wiki content diff found: %d changes", len(diffs))
		content = diffContent
		if len(content) > 3000 {
			content = content[:3000] + "\n...（diff已截断）"
		}
	} else {
		// 首次检测（无缓存），取文档开头部分（决策通常在开头）
		log.Printf("[Worker] No cached wiki content, using head")
		if len(content) > 3000 {
			content = content[:3000] + "\n...（文档后面已省略）"
		}
	}
	if len(content) > 0 {
		preview := content
		if len(preview) > 800 {
			preview = preview[:800] + "\n...（截断）"
		}
		log.Printf("[Worker] Wiki analysis content preview:\n%s", preview)
	}

	// 直接送 LLM 判断，不经过本地 pattern 过滤
	docType := classifyDocType(title, content)

	// Phase 3-4: LLM Extraction + Output
	sig := NewSignal(job.AdapterType, job.Change.Summary)
	sig.PrimaryID = nodeToken
	sig.Strength = StrengthMedium
	sig.Context.ContentSnippet = truncateForLog(content, 1000)
	sig.Context.IsDecision = true

	analysisContent := content

	proposer := "知识库系统"
	mut, pendingMuts, err := wp.engine.ProcessSignalForDocJob(sig, proposer, analysisContent, string(docType), title)
	if err != nil {
		log.Printf("[Worker] Error processing Wiki signal: %v", err)
		result.Err = err
		return result
	}
	result.Mutation = mut
	result.PendingMutations = append(result.PendingMutations, pendingMuts...)

	// 如果成功提取到决策（main 或 pending），标记文档已处理（防抖用）
	if wp.debounceTracker != nil && (mut != nil || len(pendingMuts) > 0) {
		contentHash := larkadapter.ComputeContentHash(content)
		wp.debounceTracker.MarkProcessed(nodeToken, contentHash)
		log.Printf("[Debounce] Marked wiki node %s as processed (hash: %s, main=%v, pending=%d)",
			nodeToken, contentHash[:16]+"...", mut != nil, len(pendingMuts))
	}

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// SubmitJob 提交任务（非阻塞）
func (wp *WorkerPool) SubmitJob(job *DetectionJob) {
	log.Printf("[WorkerPool] Submitting job (queue size: %d)", len(wp.jobChan))

	select {
	case wp.jobChan <- job:
		log.Printf("[WorkerPool] Job submitted successfully")
	default:
		log.Printf("[WorkerPool] Job queue full, dropping job: %s", job.Change.Summary)
	}
}

// Results 获取结果通道
func (wp *WorkerPool) Results() <-chan *DecisionResult {
	return wp.resultChan
}

// LogStats 记录统计
func (wp *WorkerPool) LogStats() {
	wp.stats.mu.Lock()
	defer wp.stats.mu.Unlock()

	log.Printf("[WorkerPool] Stats: Active=%d, Processed=%d, LLMCalls=%d",
		wp.stats.ActiveWorkers, wp.stats.JobsProcessed, wp.stats.LLMCalls)
}

// IncrementLLMCalls 增加 LLM 调用计数
func (wp *WorkerPool) IncrementLLMCalls() {
	wp.stats.mu.Lock()
	defer wp.stats.mu.Unlock()
	wp.stats.LLMCalls++
}

// Stop 停止工作池
func (wp *WorkerPool) Stop() {
	close(wp.jobChan)
	wp.wg.Wait()
	close(wp.resultChan)
	log.Println("[WorkerPool] Stopped")
}

func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractDocTitleFromSummary 从 Change.Summary 中提取文档标题
func extractDocTitleFromSummary(summary string) string {
	for _, sep := range []string{": ", "：", " — ", " - "} {
		idx := strings.LastIndex(summary, sep)
		if idx < 0 {
			continue
		}
		candidate := summary[idx+len(sep):]
		if parenIdx := strings.Index(candidate, " (by "); parenIdx > 0 {
			candidate = candidate[:parenIdx]
		}
		if parenIdx := strings.Index(candidate, "（"); parenIdx > 0 && strings.Contains(candidate[parenIdx:], "）") {
			candidate = candidate[:parenIdx]
		}
		if candidate != "" {
			return candidate
		}
	}
	return summary
}
