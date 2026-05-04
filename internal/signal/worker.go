package signal

import (
	"log"
	"runtime"
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
	Processed time.Time
	Err       error
}

// WorkerPool 工作池
type WorkerPool struct {
	jobChan    chan *DetectionJob
	resultChan chan *DecisionResult
	wg         sync.WaitGroup
	engine     *SignalActivationEngine
	maxWorkers int
	stats      *WorkerStats
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
	mut, err := wp.engine.ProcessSignalForJob(sig, proposer, content)
	if err != nil {
		log.Printf("[Worker] Error processing signal: %v", err)
		result.Err = err
		return result
	}

	result.Mutation = mut
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
		mut, err := wp.engine.ProcessSignalForJob(sig, proposer, job.Change.Summary)
		if err != nil {
			log.Printf("[Worker] Error processing VC signal: %v", err)
			result.Err = err
			return result
		}
		result.Mutation = mut
	}

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processDocsJob 处理 Docs 类型任务
func (wp *WorkerPool) processDocsJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	if job.Change.Type == "doc_decision" || job.Change.Type == "doc_comment_approval" || containsDecisionKeyword(job.Change.Summary) {
		sig := NewSignal(job.AdapterType, job.Change.Summary)
		sig.Strength = StrengthStrong
		sig.Context.ContentSnippet = job.Change.Summary

		proposer := "文档系统"
		mut, err := wp.engine.ProcessSignalForJob(sig, proposer, job.Change.Summary)
		if err != nil {
			log.Printf("[Worker] Error processing Docs signal: %v", err)
			result.Err = err
			return result
		}
		result.Mutation = mut
	}

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processCalendarJob 处理 Calendar 类型任务
func (wp *WorkerPool) processCalendarJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	if containsDecisionKeyword(job.Change.Summary) {
		log.Printf("[Worker] Found decision-related calendar event: %s", job.Change.Summary)
	}

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
		mut, err := wp.engine.ProcessSignalForJob(sig, proposer, job.Change.Summary)
		if err != nil {
			log.Printf("[Worker] Error processing Task signal: %v", err)
			result.Err = err
			return result
		}
		result.Mutation = mut
	}

	log.Println("========== WORKER PROCESS END ==========")
	return result
}

// processWikiJob 处理 Wiki 类型任务
func (wp *WorkerPool) processWikiJob(job *DetectionJob, result *DecisionResult) *DecisionResult {
	if containsDecisionKeyword(job.Change.Summary) {
		sig := NewSignal(job.AdapterType, job.Change.Summary)
		sig.Strength = StrengthMedium
		sig.Context.ContentSnippet = job.Change.Summary

		proposer := "知识库系统"
		mut, err := wp.engine.ProcessSignalForJob(sig, proposer, job.Change.Summary)
		if err != nil {
			log.Printf("[Worker] Error processing Wiki signal: %v", err)
			result.Err = err
			return result
		}
		result.Mutation = mut
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
