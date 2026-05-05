package signal

import (
	"fmt"
	"log"
	"strings"
	"time"

	"feishu-mem/internal/decision"
	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/llm"
)

type MemoryGraphInterface interface {
	GetAllDecisions() []*decision.DecisionNode
	UpsertDecision(node *decision.DecisionNode, project string)
}

type PipelineInterface interface {
	ApplyMutation(mut *DecisionMutation) error
}

// DedupAction 去重动作
type DedupAction string

const (
	DedupNew      DedupAction = "new"      // 无匹配，创建新决策
	DedupSkip     DedupAction = "skip"     // 微小变更，跳过
	DedupUpdate   DedupAction = "update"   // 有意义变更，更新现有决策
	DedupConflict DedupAction = "conflict" // 重大变更，创建冲突/替代关系
)

type SignalActivationEngine struct {
	Emitters               map[AdapterType]StateChangeEmitter
	Router                 *ActivationRouter
	Assembler              *ContextAssembler
	StateMachine           *DecisionStateMachine
	Patterns               *PatternMatcher
	detector               *EnhancedDetector // 增强型多因子检测器
	Pipeline               PipelineInterface
	Memory                 MemoryGraphInterface
	llmAgent               *llm.MemoryAgent
	contextProviderFactory *ContextProviderFactory // 上下文提供者工厂
}

func NewSignalActivationEngine(pipeline PipelineInterface, memory MemoryGraphInterface) *SignalActivationEngine {
	return &SignalActivationEngine{
		Emitters:               NewEmitters(),
		Router:                 NewActivationRouter(),
		Assembler:              NewContextAssembler(),
		StateMachine:           NewDecisionStateMachine(),
		Patterns:               NewPatternMatcher(),
		Pipeline:               pipeline,
		Memory:                 memory,
		llmAgent:               llm.NewMemoryAgent(),
		contextProviderFactory: NewContextProviderFactory(),
	}
}

func (e *SignalActivationEngine) OnDetectResult(adapter AdapterType, result *larkadapter.DetectResult) (*ProcessingReport, error) {
	log.Println("[SignalEngine] OnDetectResult called")
	log.Printf("[SignalEngine] Detector: %s, %d changes", adapter, len(result.Changes))
	return nil, nil
}

func (e *SignalActivationEngine) ProcessSignalForJob(sig *StateChangeSignal, proposer, content string) (*DecisionMutation, []*DecisionMutation, error) {
	log.Println("========== SIGNAL ENGINE PROCESS ==========")
	log.Printf("[SignalEngine] ProcessSignalForJob called")
	log.Printf("[SignalEngine] Proposer: %s", proposer)
	log.Printf("[SignalEngine] Content: %s", truncateForLog(content, 200))
	log.Printf("[SignalEngine] LLM available: %v", e.llmAgent.IsAvailable())

	var newNode *decision.DecisionNode
	allDecisions := e.Memory.GetAllDecisions()
	var lastLLMResult *llm.ExtractionResult

	// Step 0: 评论级别去重
	if sig.CommentID != "" || strings.TrimSpace(content) != "" {
		for _, existing := range allDecisions {
			if sig.CommentID != "" {
				for _, cid := range existing.FeishuLinks.RelatedCommentIDs {
					if cid == sig.CommentID {
						log.Printf("[SignalEngine] Comment %s already processed (ID match), skipping", sig.CommentID)
						log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
						return nil, nil, nil
					}
				}
			}
			if sig.PrimaryID != "" {
				for _, docToken := range existing.FeishuLinks.RelatedDocTokens {
					if docToken == sig.PrimaryID {
						if strings.Contains(existing.Decision, content) {
							log.Printf("[SignalEngine] Comment already processed (content+doc match with %s), skipping", existing.SDRID)
							log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
							return nil, nil, nil
						}
					}
				}
			}
		}
	}

	if e.llmAgent.IsAvailable() {
		log.Println("[SignalEngine] LLM is available, calling...")

		// Step 1: 使用对应的 ContextProvider 获取相关上下文
		var relatedDecisionSummaries []string
		if e.contextProviderFactory != nil {
			provider := e.contextProviderFactory.GetProvider(sig.Adapter)
			log.Printf("[SignalEngine] Using context provider: %s", provider.Name())
			var err error
			relatedDecisionSummaries, err = provider.GetRelevantContext(sig, allDecisions)
			if err != nil {
				log.Printf("[SignalEngine] Context provider error: %v", err)
			}
		}

		// 如果没有拿到上下文，使用简单的方式补充
		if len(relatedDecisionSummaries) == 0 {
			for _, d := range allDecisions {
				if len(relatedDecisionSummaries) >= 5 {
					break
				}
				summary := fmt.Sprintf("[%s] %s: %s", d.Status, d.Title, d.Decision)
				relatedDecisionSummaries = append(relatedDecisionSummaries, summary)
			}
		}
		log.Printf("[SignalEngine] Context summaries: %d items", len(relatedDecisionSummaries))

		topics := e.getAllTopics()
		log.Printf("[SignalEngine] Available topics: %v", topics)

		log.Println("[SignalEngine] Calling ExtractDecisionWithContext...")
		result, err := e.llmAgent.ExtractDecisionWithContext(
			content, topics, relatedDecisionSummaries)
			lastLLMResult = result
		if err != nil {
			log.Printf("[SignalEngine] LLM extraction failed: %v, falling back to heuristic", err)
			newNode = e.createDecisionFallback(proposer, content)
		} else {
			log.Printf("[SignalEngine] LLM extraction result: HasDecision=%v, Confidence=%.2f",
				result.HasDecision, result.Confidence)

			if result.Decision != nil {
				log.Printf("[SignalEngine] Decision details: Title=%s, Topic=%s",
					result.Decision.Title, result.Decision.SuggestedTopic)
			}

			if result.HasDecision && result.Confidence >= 0.6 && result.Decision != nil {
				lastLLMResult = result
				newNode = decision.NewDecisionNode(
					GenerateSDRID(),
					result.Decision.Title,
					"feishu-mem",
					result.Decision.SuggestedTopic,
				)
				newNode.Decision = result.Decision.Decision
				newNode.Rationale = result.Decision.Rationale
				newNode.Proposer = proposer
				newNode.Executor = result.Decision.Executor
				newNode.ImpactLevel = decision.ImpactLevel(result.Decision.ImpactLevel)
				newNode.Status = decision.StatusPending

				// 记录来源信息到 FeishuLinks
				newNode.FeishuLinks.RelatedChatIDs = appendRelatedIDs(newNode.FeishuLinks.RelatedChatIDs, sig)
				newNode.FeishuLinks.RelatedDocTokens = appendRelatedTokens(newNode.FeishuLinks.RelatedDocTokens, sig)
				if sig.PrimaryID != "" {
					newNode.FeishuLinks.RelatedDocTokens = appendUniqueString(newNode.FeishuLinks.RelatedDocTokens, sig.PrimaryID)
				}
				if sig.CommentID != "" {
					newNode.FeishuLinks.RelatedCommentIDs = appendUniqueString(newNode.FeishuLinks.RelatedCommentIDs, sig.CommentID)
				}

				log.Printf("[SignalEngine] Decision extracted from LLM: %s", result.Decision.Title)
			} else if !result.HasDecision {
				log.Printf("[SignalEngine] LLM determined no decision, skipping entirely")
				log.Println("========== SIGNAL ENGINE END ==========")
				return nil, nil, nil
			} else {
				log.Printf("[SignalEngine] LLM confidence too low (%.2f < 0.6), skipping",
					result.Confidence)
				log.Println("========== SIGNAL ENGINE END ==========")
				return nil, nil, nil
			}
		}
	} else {
		log.Println("[SignalEngine] LLM not available, using heuristic fallback")
		newNode = e.createDecisionFallback(proposer, content)
	}

		// 收集反对意见（在 dedup 之前执行）
	var pending []*DecisionMutation
	if lastLLMResult != nil && lastLLMResult.HasObjections && len(lastLLMResult.Objections) > 0 {
		pending = e.ProcessObjections(lastLLMResult, sig, allDecisions, proposer, "")
			delMuts := e.ProcessDeletedDecisions(lastLLMResult, allDecisions)
			pending = append(pending, delMuts...)
	}

// Step 2: 检查重复
	if newNode != nil {
		// Step 2a: 同文档匹配 → 直接 update，不做冲突判断
		if existing := e.findSameDocument(sig, allDecisions); existing != nil {
			log.Printf("[SignalEngine] Same document match, updating existing decision %s", existing.SDRID)
			mut := e.StateMachine.CreateMutationForUpdate(existing.SDRID, newNode, sig)
			log.Printf("[SignalEngine] Created update mutation: Type=%s, SDRID=%s", mut.Type, mut.SDRID)
			log.Println("========== SIGNAL ENGINE END ==========")
			return mut, nil, nil
		}

		// Step 2b: 跨文档匹配 → evaluateDedupAction（可能检测到冲突）
		if existing := e.findSimilarDecision(newNode, sig, allDecisions); existing != nil {
			log.Printf("[SignalEngine] Found similar/related decision: %s (%s)", existing.Title, existing.SDRID)

			action := e.evaluateDedupAction(newNode, existing)
			switch action {
			case DedupSkip:
				log.Printf("[SignalEngine] Minor change, skipping")
				log.Println("========== SIGNAL ENGINE END ==========")
				return nil, pending, nil
			case DedupUpdate:
				log.Printf("[SignalEngine] Updating existing decision %s", existing.SDRID)
				mut := e.StateMachine.CreateMutationForUpdate(existing.SDRID, newNode, sig)
				log.Printf("[SignalEngine] Created update mutation: Type=%s, SDRID=%s", mut.Type, mut.SDRID)
				log.Println("========== SIGNAL ENGINE END ==========")
				return mut, pending, nil
			case DedupConflict:
				log.Printf("[SignalEngine] Conflict with %s, attempting LLM resolution...", existing.SDRID)
				mut := e.resolveConflict(newNode, existing, sig)
				if mut != nil {
					return mut, pending, nil
				}
				log.Println("========== SIGNAL ENGINE END ==========")
				return nil, pending, nil
			}
		}
	}

	// 如果 newNode 为 nil（仅有反对意见无决策），只返回 pending
	if newNode == nil {
		if len(pending) > 0 {
			log.Printf("[SignalEngine] No decision, returning %d objection mutations only", len(pending))
		}
		return nil, pending, nil
	}
	mut := e.StateMachine.CreateMutationForNewDecision(newNode, sig)
	if len(pending) > 0 {
		log.Printf("[SignalEngine] Also created %d objection mutations", len(pending))
	}

	log.Printf("[SignalEngine] Created mutation: Type=%s, SDRID=%s", mut.Type, mut.SDRID)
	log.Println("========== SIGNAL ENGINE END ==========")
	return mut, pending, nil
}

// ProcessSignalForDocJob 处理文档类型信号的决策提取（多决策 + 时序关联模式）
func (e *SignalActivationEngine) ProcessSignalForDocJob(sig *StateChangeSignal, proposer, content, docType, title string) (*DecisionMutation, []*DecisionMutation, error) {
	log.Println("========== SIGNAL ENGINE DOC PROCESS ==========")
	log.Printf("[SignalEngine] ProcessSignalForDocJob called")
	log.Printf("[SignalEngine] Proposer: %s", proposer)
	log.Printf("[SignalEngine] DocType: %s, Title: %s", docType, title)
	log.Printf("[SignalEngine] Content length: %d", len(content))
	log.Printf("[SignalEngine] LLM available: %v", e.llmAgent.IsAvailable())

	allDecisions := e.Memory.GetAllDecisions()
	var lastLLMResult *llm.ExtractionResult

	// Step 0: 评论级别去重
	if sig.CommentID != "" || strings.TrimSpace(content) != "" {
		for _, existing := range allDecisions {
			if sig.CommentID != "" {
				for _, cid := range existing.FeishuLinks.RelatedCommentIDs {
					if cid == sig.CommentID {
						log.Printf("[SignalEngine] Comment %s already processed (ID match), skipping", sig.CommentID)
						log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
						return nil, nil, nil
					}
				}
			}
			if sig.PrimaryID != "" {
				for _, docToken := range existing.FeishuLinks.RelatedDocTokens {
					if docToken == sig.PrimaryID {
						if strings.Contains(existing.Decision, content) {
							log.Printf("[SignalEngine] Comment already processed (content+doc match with %s), skipping", existing.SDRID)
							log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
							return nil, nil, nil
						}
					}
				}
			}
		}
	}

	// Step 1: LLM 提取
	if !e.llmAgent.IsAvailable() {
		log.Println("[SignalEngine] LLM not available, using heuristic fallback")
		newNode := e.createDecisionFallback(proposer, content)
		mut := e.StateMachine.CreateMutationForNewDecision(newNode, sig)
		return mut, nil, nil
	}

	log.Println("[SignalEngine] LLM is available, calling...")

	var relatedDecisionSummaries []string
	if e.contextProviderFactory != nil {
		provider := e.contextProviderFactory.GetProvider(sig.Adapter)
		var err error
		relatedDecisionSummaries, err = provider.GetRelevantContext(sig, allDecisions)
		if err != nil {
			log.Printf("[SignalEngine] Context provider error: %v", err)
		}
	}
	if len(relatedDecisionSummaries) == 0 {
		for _, d := range allDecisions {
			if len(relatedDecisionSummaries) >= 5 {
				break
			}
			summary := fmt.Sprintf("[%s] %s: %s", d.Status, d.Title, d.Decision)
			relatedDecisionSummaries = append(relatedDecisionSummaries, summary)
		}
	}
	log.Printf("[SignalEngine] Context summaries: %d items", len(relatedDecisionSummaries))

	topics := e.getAllTopics()
	log.Printf("[SignalEngine] Available topics: %v", topics)

	docResult, err := e.llmAgent.ExtractDecisionFromDocWithContext(content, topics, docType, title, relatedDecisionSummaries)
	lastLLMResult = docResult
	if err != nil {
		log.Printf("[SignalEngine] LLM doc extraction failed: %v, falling back to heuristic", err)
		newNode := e.createDecisionFallback(proposer, content)
		mut := e.StateMachine.CreateMutationForNewDecision(newNode, sig)
		return mut, nil, nil
	}

	log.Printf("[SignalEngine] LLM doc extraction result: HasDecision=%v, Confidence=%.2f, ChangeType=%s",
		docResult.HasDecision, docResult.Confidence, docResult.ChangeType)

	// 如果没有决策且没有反对意见，直接跳过
	if !docResult.HasDecision && !docResult.HasObjections {
		log.Printf("[SignalEngine] LLM determined no decision and no objections, skipping")
		log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
		return nil, nil, nil
	}

	// 如果置信度太低，跳过
	if docResult.HasDecision && docResult.Confidence < 0.6 {
		log.Printf("[SignalEngine] LLM confidence too low (%.2f < 0.6), skipping", docResult.Confidence)
		log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
		return nil, nil, nil
	}

	// Step 1.5: 处理反对意见和删除
	var pending []*DecisionMutation
	if lastLLMResult != nil && lastLLMResult.HasObjections && len(lastLLMResult.Objections) > 0 {
		pending = e.ProcessObjections(lastLLMResult, sig, allDecisions, proposer, "")
		delMuts := e.ProcessDeletedDecisions(lastLLMResult, allDecisions)
		pending = append(pending, delMuts...)
	}

	// Step 2: 获取决策列表（兼容新旧格式）
	extractedDecisions := docResult.Decisions
	if len(extractedDecisions) == 0 && docResult.Decision != nil {
		extractedDecisions = []llm.DecisionExtract{*docResult.Decision}
	}

	// Step 2.5: 筛选 + 合并
	if len(extractedDecisions) > 1 {
		extractedDecisions = FilterAndMergeDecisions(extractedDecisions)
	}

	if len(extractedDecisions) == 0 {
		log.Printf("[SignalEngine] No decisions extracted")
		if len(pending) > 0 {
			log.Printf("[SignalEngine] Returning %d objection/deletion mutations only", len(pending))
		}
		log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
		return nil, pending, nil
	}

	log.Printf("[SignalEngine] Processing %d extracted decisions", len(extractedDecisions))

	// Step 3: 逐条处理每个决策
	var mainMutation *DecisionMutation
	var additionalMutations []*DecisionMutation

	for i, dec := range extractedDecisions {
		log.Printf("[SignalEngine] Processing decision [%d/%d]: %s", i+1, len(extractedDecisions), dec.Title)

		// 构建 DecisionNode
		newNode := decision.NewDecisionNode(
			GenerateSDRID(),
			dec.Title,
			"feishu-mem",
			dec.SuggestedTopic,
		)
		newNode.Decision = dec.Decision
		newNode.Rationale = dec.Rationale
		newNode.Proposer = proposer
		newNode.Executor = dec.Executor
		newNode.ImpactLevel = decision.ImpactLevel(dec.ImpactLevel)
		newNode.Status = decision.StatusPending

		// 设置时间相关字段
		if dec.DecisionTime != "" {
			newNode.Phase = dec.DecisionTime
		}
		if dec.ProjectPhase != "" {
			newNode.Phase = dec.ProjectPhase
		}

		// 设置飞书关联
		newNode.FeishuLinks.RelatedDocTokens = appendRelatedTokens(newNode.FeishuLinks.RelatedDocTokens, sig)
		if sig.PrimaryID != "" {
			newNode.FeishuLinks.RelatedDocTokens = appendUniqueString(newNode.FeishuLinks.RelatedDocTokens, sig.PrimaryID)
		}
		if sig.CommentID != "" {
			newNode.FeishuLinks.RelatedCommentIDs = appendUniqueString(newNode.FeishuLinks.RelatedCommentIDs, sig.CommentID)
		}

		// Step 3a: 同文档匹配 → 直接 update
		if existing := e.findSameDocument(sig, allDecisions); existing != nil {
			log.Printf("[SignalEngine] Same document match, updating existing decision %s", existing.SDRID)
			mut := e.StateMachine.CreateMutationForUpdate(existing.SDRID, newNode, sig)
			if mainMutation == nil {
				mainMutation = mut
			} else {
				additionalMutations = append(additionalMutations, mut)
			}
			continue
		}

		// Step 3b: 跨文档匹配 → evaluateDedupAction
		if existing := e.findSimilarDecision(newNode, sig, allDecisions); existing != nil {
			log.Printf("[SignalEngine] Found similar decision: %s (%s)", existing.Title, existing.SDRID)

			action := e.evaluateDedupAction(newNode, existing)
			switch action {
			case DedupSkip:
				log.Printf("[SignalEngine] Skipping duplicate decision: %s", dec.Title)
				continue
			case DedupUpdate:
				log.Printf("[SignalEngine] Updating existing decision %s", existing.SDRID)
				mut := e.StateMachine.CreateMutationForUpdate(existing.SDRID, newNode, sig)
				if mainMutation == nil {
					mainMutation = mut
				} else {
					additionalMutations = append(additionalMutations, mut)
				}
				continue
			case DedupConflict:
				log.Printf("[SignalEngine] Conflict with %s, attempting LLM resolution...", existing.SDRID)
				mut := e.resolveConflict(newNode, existing, sig)
				if mut != nil {
					if mainMutation == nil {
						mainMutation = mut
					} else {
						additionalMutations = append(additionalMutations, mut)
					}
				}
				continue
			}
		}

		// Step 3c: 无匹配 → 创建新决策
		mut := e.StateMachine.CreateMutationForNewDecision(newNode, sig)
		log.Printf("[SignalEngine] Created new decision mutation: SDRID=%s, Title=%s", mut.SDRID, dec.Title)
		if mainMutation == nil {
			mainMutation = mut
		} else {
			additionalMutations = append(additionalMutations, mut)
		}
	}

	log.Printf("[SignalEngine] Doc process complete: main=%v, additional=%d, pending=%d",
		mainMutation != nil, len(additionalMutations), len(pending))
	log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
	return mainMutation, append(additionalMutations, pending...), nil
}

// ProcessObjections 处理提取结果中的反对意见，创建 Objection 列表
// 在 worker 调用 ProcessSignalForJob/DocJob 之后调用，结果追加至 result.PendingMutations
func (e *SignalActivationEngine) ProcessObjections(
	result *llm.ExtractionResult,
	sig *StateChangeSignal,
	allDecisions []*decision.DecisionNode,
	_ string, // proposer
	docToken string,
) []*DecisionMutation {
	if result == nil || !result.HasObjections || len(result.Objections) == 0 {
		return nil
	}

	log.Printf("[SignalEngine] Processing %d objections from extraction result", len(result.Objections))
	var muts []*DecisionMutation

	for _, objExtract := range result.Objections {
		obj := &decision.Objection{
			OID:              GenerateObjectionID(),
			ObjectionContent: objExtract.ObjectionContent,
			Rationale:        objExtract.Rationale,
			Alternative:      objExtract.Alternative,
			Objector:         objExtract.Objector,
			Status:           decision.ObjectionActive,
			SourceType:       objExtract.Source,
			SourceDocToken:   docToken,
			Topic:            "general",
			Project:          "feishu-mem",
			CreatedAt:        time.Now(),
		}
		if sig != nil {
			obj.SourceChatID = extractChatID(sig)
			obj.SourceMessageID = sig.SignalID
		}

		// 尝试匹配到已有决策
		if matched := e.findMatchingDecisionForObjection(obj, allDecisions); matched != nil {
			obj.ReferencesDecision = matched.SDRID
			log.Printf("[SignalEngine] Objection %s linked to decision %s", obj.OID, matched.SDRID)
		}

		muts = append(muts, e.StateMachine.CreateMutationForNewObjection(obj, sig))
		log.Printf("[SignalEngine] Created objection mutation: %s", obj.OID)
	}

	return muts
}

// ProcessDeletedDecisions 处理提取结果中被删除的决策
func (e *SignalActivationEngine) ProcessDeletedDecisions(
	result *llm.ExtractionResult,
	allDecisions []*decision.DecisionNode,
) []*DecisionMutation {
	if result == nil || !result.HasDeletions || len(result.Deletions) == 0 {
		return nil
	}
	log.Printf("[SignalEngine] Processing %d deleted decisions", len(result.Deletions))
	var muts []*DecisionMutation
	for _, del := range result.Deletions {
		matched := e.findMatchingDeletedDecision(del, allDecisions)
		if matched == nil {
			log.Printf("[SignalEngine] No matching decision found for deleted: %s", del.OriginalDecision)
			continue
		}
		var newStatus decision.DecisionStatus
		switch del.Action {
		case "rejected":
			newStatus = decision.StatusRejected
		case "deprecated":
			newStatus = decision.StatusDeprecated
		case "superseded":
			newStatus = decision.StatusSuperseded
		default:
			newStatus = decision.StatusDeprecated
		}
		muts = append(muts, e.StateMachine.CreateMutationForDeprecation(
			matched.SDRID, newStatus, "Document deletion: "+del.OriginalDecision))
		log.Printf("[SignalEngine] Created deprecation mutation for %s -> %s", matched.SDRID, newStatus)
	}
	return muts
}

// findMatchingDeletedDecision 将被删除的决策内容匹配到现有决策
func (e *SignalActivationEngine) findMatchingDeletedDecision(
	del llm.DeletedDecisionExtract,
	allDecisions []*decision.DecisionNode,
) *decision.DecisionNode {
	for _, d := range allDecisions {
		if !d.IsActive() {
			continue
		}
		if strings.Contains(d.Decision, del.OriginalDecision) ||
			strings.Contains(d.Title, del.OriginalDecision) {
			log.Printf("[SignalEngine] Matched deleted content to decision %s", d.SDRID)
			return d
		}
	}
	return nil
}

// findMatchingDecisionForObjection 将反对意见匹配到相关决策
func (e *SignalActivationEngine) findMatchingDecisionForObjection(
	obj *decision.Objection,
	allDecisions []*decision.DecisionNode,
) *decision.DecisionNode {
	// 优先级1: 同一文档 token
	if obj.SourceDocToken != "" {
		for _, d := range allDecisions {
			for _, tok := range d.FeishuLinks.RelatedDocTokens {
				if tok == obj.SourceDocToken {
					log.Printf("[SignalEngine] Objection matched by doc token: %s", tok)
					return d
				}
			}
		}
	}

	// 优先级2: 同一群聊
	if obj.SourceChatID != "" {
		for _, d := range allDecisions {
			for _, chatID := range d.FeishuLinks.RelatedChatIDs {
				if chatID == obj.SourceChatID {
					log.Printf("[SignalEngine] Objection matched by chat ID: %s", chatID)
					return d
				}
			}
		}
	}

	// 优先级3: 活跃决策中内容包含反对意见关键词
	for _, d := range allDecisions {
		if !d.IsActive() {
			continue
		}
		// 如果决策的标题或内容与反对意见的主题相关
		if stringsContainsIgnoreCase(d.Title, obj.ObjectionContent) ||
			stringsContainsIgnoreCase(d.Decision, obj.ObjectionContent) {
			log.Printf("[SignalEngine] Objection matched by content overlap with: %s", d.SDRID)
			return d
		}
	}

	return nil
}

// extractChatID 从信号中提取群聊 ID
func extractChatID(sig *StateChangeSignal) string {
	if sig == nil {
		return ""
	}
	if len(sig.RelatedIDs) > 0 {
		return sig.RelatedIDs[0]
	}
	return ""
}

// === 决策去重和辅助函数 ===

// findSimilarDecision 查找相似决策
func (e *SignalActivationEngine) findSimilarDecision(
	newNode *decision.DecisionNode,
	sig *StateChangeSignal,
	allDecisions []*decision.DecisionNode,
) *decision.DecisionNode {
	log.Printf("[SignalEngine] Checking %d existing decisions for duplicates...", len(allDecisions))

	for _, existing := range allDecisions {
		log.Printf("[SignalEngine] Comparing with: SDRID=%s, Title=%s, DocTokens=%v",
			existing.SDRID, existing.Title, existing.FeishuLinks.RelatedDocTokens)

		// 规则1（已移至 findSameDocument）：同文档匹配由 caller 提前处理，不在此处做冲突判断

		// 规则2：Doc/Wiki 降级匹配 — 标题相似（跨文档匹配，可能触发冲突检测）
		if sig.Adapter == AdapterDocs || sig.Adapter == AdapterWiki {
			if e.hasSimilarTitle(newNode, existing) {
				log.Printf("[SignalEngine] Found decision with SIMILAR TITLE (cross-document)!")
				return existing
			}
		}

		// 规则3：Wiki 同主题匹配（跨文档匹配，可能触发冲突检测）
		if sig.Adapter == AdapterWiki {
			if e.isSameTopicWikiDocument(newNode, existing) {
				log.Printf("[SignalEngine] Found decision from SAME TOPIC (Wiki)!")
				return existing
			}
		}

		// 规则4：不同文档 → 不去重
	}
	log.Printf("[SignalEngine] No similar decision found.")
	return nil
}

// findSameDocument 仅通过文档 token 匹配已有决策（同文档编辑）
// 与 findSimilarDecision 不同：此函数不做冲突判断，由 caller 直接走 update 路径
func (e *SignalActivationEngine) findSameDocument(
	sig *StateChangeSignal,
	allDecisions []*decision.DecisionNode,
) *decision.DecisionNode {
	for _, existing := range allDecisions {
		if e.isSameDocument(existing, sig) {
			log.Printf("[SignalEngine] Found decision from SAME DOCUMENT: %s", existing.SDRID)
			return existing
		}
	}
	return nil
}

// isSameDocument 检查是否是同一文档的决策
func (e *SignalActivationEngine) isSameDocument(existing *decision.DecisionNode, sig *StateChangeSignal) bool {
	// 检查 PrimaryID
	if sig.PrimaryID != "" {
		for _, existingDocToken := range existing.FeishuLinks.RelatedDocTokens {
			if sig.PrimaryID == existingDocToken {
				log.Printf("[SignalEngine] DocToken match: PrimaryID=%s", sig.PrimaryID)
				return true
			}
		}
	}

	// 检查 EmbeddedURLs
	for _, url := range sig.Context.EmbeddedURLs {
		if url.ExtractedToken != "" {
			for _, existingDocToken := range existing.FeishuLinks.RelatedDocTokens {
				if url.ExtractedToken == existingDocToken {
					log.Printf("[SignalEngine] DocToken match: EmbeddedURL=%s", url.ExtractedToken)
					return true
				}
			}
		}
	}

	return false
}

// hasSimilarTitle 检查标题是否相似（降级匹配，覆盖 DocTokens 为空的遗留数据）
func (e *SignalActivationEngine) hasSimilarTitle(newNode, existing *decision.DecisionNode) bool {
	return stringsContainsIgnoreCase(existing.Title, newNode.Title) ||
		stringsContainsIgnoreCase(newNode.Title, existing.Title)
}

// isSameTopicWikiDocument 判断两个 Wiki 文档是否属于同一主题
// 目前简化实现：通过标题和内容相似性判断
// 后续可以根据知识库节点层级关系精确判断
func (e *SignalActivationEngine) isSameTopicWikiDocument(newNode, existing *decision.DecisionNode) bool {
	// 检查标题相似性（比如都包含"后端方案"）
	titleSimilar := stringsContainsIgnoreCase(existing.Title, newNode.Title) ||
		stringsContainsIgnoreCase(newNode.Title, existing.Title)
	if titleSimilar {
		log.Printf("[SignalEngine] Wiki decisions have similar titles - consider same topic")
		return true
	}

	// 检查主题是否相同（newNode.Topic 和 existing.Topic）
	if newNode.Topic != "" && existing.Topic != "" && newNode.Topic == existing.Topic {
		log.Printf("[SignalEngine] Wiki decisions have same topic: %s", newNode.Topic)
		return true
	}

	return false
}

// hasOverlappingEntities 检查是否有重叠的实体关联
func (e *SignalActivationEngine) hasOverlappingEntities(existing *decision.DecisionNode, sig *StateChangeSignal) bool {
	for _, chatID := range sig.RelatedIDs {
		for _, existingChatID := range existing.FeishuLinks.RelatedChatIDs {
			if chatID == existingChatID {
				return true
			}
		}
	}

	for _, url := range sig.Context.EmbeddedURLs {
		if url.ExtractedToken != "" {
			for _, existingDocToken := range existing.FeishuLinks.RelatedDocTokens {
				if url.ExtractedToken == existingDocToken {
					return true
				}
			}
		}
	}

	if sig.PrimaryID != "" {
		for _, existingDocToken := range existing.FeishuLinks.RelatedDocTokens {
			if sig.PrimaryID == existingDocToken {
				return true
			}
		}
	}

	return false
}

// evaluateDedupAction 使用 LLM 同时判断去重和冲突
func (e *SignalActivationEngine) evaluateDedupAction(
	newNode *decision.DecisionNode,
	existing *decision.DecisionNode,
) DedupAction {
	log.Printf("[SignalEngine] Evaluating dedup action via LLM:")
	log.Printf("[SignalEngine]  - New: Title='%s', Decision='%s'",
		newNode.Title, truncateForLog(newNode.Decision, 100))
	log.Printf("[SignalEngine]  - Old: Title='%s', Decision='%s'",
		existing.Title, truncateForLog(existing.Decision, 100))

	// 优先使用 LLM 判断
	if e.llmAgent.IsAvailable() {
		result, err := e.llmAgent.EvaluateDedupAction(
			newNode.Title, newNode.Decision,
			existing.Title, existing.Decision,
		)
		if err == nil && result != nil {
			log.Printf("[SignalEngine] LLM dedup result: action=%s, reason=%s", result.Action, result.Reason)
			switch result.Action {
			case "skip":
				return DedupSkip
			case "update":
				return DedupUpdate
			case "conflict":
				return DedupConflict
			}
		}
		log.Printf("[SignalEngine] LLM dedup failed or returned nil, defaulting to update: %v", err)
	} else {
		log.Printf("[SignalEngine] LLM not available, defaulting to update")
	}

	// LLM 不可用时默认行为：更新（不丢弃任何决策，保留人工判断空间）
	return DedupUpdate
}

// resolveConflict 冲突解决 — 先由 LLM 判断能否自动合并
func (e *SignalActivationEngine) resolveConflict(
	newNode, existing *decision.DecisionNode,
	sig *StateChangeSignal,
) *DecisionMutation {
	if e.llmAgent.IsAvailable() {
		result, err := e.llmAgent.ResolveConflictAction(
			newNode.Title, newNode.Decision,
			existing.Title, existing.Decision,
		)
		if err == nil && result != nil {
			switch result.Action {
			case "merge":
				log.Printf("[SignalEngine] ✅ LLM auto-resolved conflict: %s", result.Reason)
				newNode.Decision = result.MergedDecision
				mut := e.StateMachine.CreateMutationForConflictMerge(
					existing.SDRID, newNode, sig, "merge", result.Reason, result.MergedDecision)
				log.Printf("[SignalEngine] Created merge mutation for %s", existing.SDRID)
				return mut
			default: // keep_both
				log.Printf("[SignalEngine] ⚠️ CONFLICT needs manual resolution: %s", result.Reason)
				mut := e.StateMachine.CreateMutationForConflictKeepBoth(
					newNode, existing.SDRID, sig, result.Reason)
				log.Printf("[SignalEngine] Created keep_both mutation, conflict: %s vs %s",
					mut.SDRID, mut.ConflictSDRID)
				return mut
			}
		}
	}

	// LLM 不可用或调用失败 → 默认 keep_both，不丢失决策
	log.Printf("[SignalEngine] LLM conflict resolve unavailable, defaulting to keep_both")
	mut := e.StateMachine.CreateMutationForConflictKeepBoth(
		newNode, existing.SDRID, sig, "LLM unavailable, manual resolution needed")
	return mut
}

// appendUniqueString 添加唯一字符串
func appendUniqueString(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func stringsContainsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(substr) == 0 || strings.Contains(stringsToLower(s), stringsToLower(substr)))
}

func stringsToLower(s string) string {
	return strings.ToLower(s)
}

func (e *SignalActivationEngine) getAllTopics() []string {
	var topics []string
	seen := make(map[string]bool)
	for _, d := range e.Memory.GetAllDecisions() {
		if !seen[d.Topic] {
			seen[d.Topic] = true
			topics = append(topics, d.Topic)
		}
	}
	if len(topics) == 0 {
		topics = []string{"general"}
	}
	return topics
}

func (e *SignalActivationEngine) createDecisionFallback(proposer, content string) *decision.DecisionNode {
	newNode := decision.NewDecisionNode(
		GenerateSDRID(),
		"Auto-extracted decision",
		"feishu-mem",
		"general",
	)
	newNode.Status = decision.StatusPending
	newNode.Decision = content
	newNode.Proposer = proposer
	newNode.Executor = extractExecutorFromSummary(content)
	newNode.ImpactLevel = extractImpactFromSummary(content)
	return newNode
}

// matchDecisionKeywords 使用增强型检测器进行关键词检测
// Deprecated: 新代码应直接使用 EnhancedDetector.Analyze()
func matchDecisionKeywords(text string) []string {
	detector := NewEnhancedDetector()
	result := detector.Analyze(text, nil)
	if !result.IsDecision {
		return nil
	}
	var keywords []string
	for _, s := range result.SignalDetails {
		keywords = append(keywords, s.Name)
	}
	return keywords
}

func extractSenderFromSummary(summary string) string {
	if idx := strings.Index(summary, "] "); idx >= 0 {
		rest := summary[idx+2:]
		if colonIdx := strings.Index(rest, ": "); colonIdx >= 0 {
			return rest[:colonIdx]
		}
	}
	return ""
}

func extractExecutorFromSummary(text string) string {
	executorLabels := []string{"执行人"}
	for _, label := range executorLabels {
		if idx := strings.Index(text, label); idx >= 0 {
			rest := text[idx+len(label):]
			endIdx := len(rest)
			for i, c := range rest {
				if c == ',' || c == '，' || c == '。' || c == ' ' {
					endIdx = i
					break
				}
			}
			return strings.TrimSpace(rest[:endIdx])
		}
	}
	return ""
}

func extractImpactFromSummary(text string) decision.ImpactLevel {
	if strings.Contains(text, "critical") || strings.Contains(text, "关键") {
		return decision.ImpactCritical
	}
	if strings.Contains(text, "major") || strings.Contains(text, "重要") {
		return decision.ImpactMajor
	}
	if strings.Contains(text, "minor") || strings.Contains(text, "轻微") {
		return decision.ImpactMinor
	}
	return decision.ImpactAdvisory
}

// appendRelatedIDs 添加相关的聊天 ID（去重）
func appendRelatedIDs(existing []string, sig *StateChangeSignal) []string {
	result := append([]string{}, existing...)
	for _, id := range sig.RelatedIDs {
		found := false
		for _, e := range result {
			if e == id {
				found = true
				break
			}
		}
		if !found {
			result = append(result, id)
		}
	}
	return result
}

// appendRelatedTokens 添加相关的文档 token（去重）
func appendRelatedTokens(existing []string, sig *StateChangeSignal) []string {
	result := append([]string{}, existing...)
	for _, url := range sig.Context.EmbeddedURLs {
		if url.ExtractedToken != "" {
			found := false
			for _, e := range result {
				if e == url.ExtractedToken {
					found = true
					break
				}
			}
			if !found {
				result = append(result, url.ExtractedToken)
			}
		}
	}
	return result
}

type PatternMatcher struct{}

func NewPatternMatcher() *PatternMatcher {
	return &PatternMatcher{}
}

// ========== 决策筛选与合并 ==========

// FilterAndMergeDecisions 对提取的决策进行筛选、合并和降噪
// 流程：硬过滤 → 相关性分组 → 组内合并
func FilterAndMergeDecisions(decisions []llm.DecisionExtract) []llm.DecisionExtract {
	if len(decisions) <= 1 {
		return decisions
	}

	log.Printf("[Filter] Input: %d decisions", len(decisions))

	// Step 1: 硬过滤 — 移除低质量决策
	filtered := hardFilterDecisions(decisions)
	log.Printf("[Filter] After hard filter: %d decisions", len(filtered))

	if len(filtered) <= 1 {
		return filtered
	}

	// Step 2: 按 project_phase + topic 分组
	groups := groupDecisionsByRelation(filtered)
	log.Printf("[Filter] Grouped into %d clusters", len(groups))

	// Step 3: 组内合并
	var merged []llm.DecisionExtract
	for _, group := range groups {
		if len(group) == 1 {
			merged = append(merged, group[0])
		} else {
			m := mergeDecisionGroup(group)
			log.Printf("[Filter] Merged %d decisions into: %s", len(group), m.Title)
			merged = append(merged, m)
		}
	}

	log.Printf("[Filter] Output: %d decisions (merged from %d)", len(merged), len(filtered))
	return merged
}

// hardFilterDecisions 硬过滤：移除不符合决策标准的条目
func hardFilterDecisions(decisions []llm.DecisionExtract) []llm.DecisionExtract {
	var result []llm.DecisionExtract

	// 任务/截止日期模式
	taskPatterns := []string{"完成", "交付", "提交", "上线", "发布", "前完成", "前交付", "前提交"}
	// 优先级模式
	priorityPatterns := []string{"最高优先级", "第一优先级", "首要", "优先处理"}
	// 确认/附和模式
	confirmPatterns := []string{"确认", "同意", "赞同", "批准", "通过"}

	for _, dec := range decisions {
		// 规则 1: impact_level 为 advisory 的直接跳过
		if dec.ImpactLevel == "advisory" {
			log.Printf("[Filter] Skipping advisory decision: %s", dec.Title)
			continue
		}

		// 规则 2: 没有依据且没有执行者的低置信度决策跳过
		if dec.Rationale == "" && dec.Executor == "" && dec.ImpactLevel != "critical" && dec.ImpactLevel != "major" {
			log.Printf("[Filter] Skipping low-info decision: %s", dec.Title)
			continue
		}

		// 规则 3: 标题匹配任务模式 → 跳过
		titleLower := strings.ToLower(dec.Title)
		isTask := false
		for _, p := range taskPatterns {
			if strings.Contains(titleLower, p) {
				isTask = true
				break
			}
		}
		// 但如果决策内容本身有实质选型，则不是纯任务
		if isTask && dec.Rationale == "" && !hasSubstantiveChoice(dec.Decision) {
			log.Printf("[Filter] Skipping task-like decision: %s", dec.Title)
			continue
		}

		// 规则 4: 标题匹配优先级模式 → 跳过
		isPriority := false
		for _, p := range priorityPatterns {
			if strings.Contains(titleLower, p) {
				isPriority = true
				break
			}
		}
		if isPriority {
			log.Printf("[Filter] Skipping priority decision: %s", dec.Title)
			continue
		}

		// 规则 5: decision_type 为 confirmation 且无新信息 → 跳过
		if dec.DecisionType == "confirmation" && dec.Rationale == "" {
			log.Printf("[Filter] Skipping confirmation decision: %s", dec.Title)
			continue
		}

		// 规则 6: 标题是确认模式且决策内容不包含具体选型 → 跳过
		isConfirm := false
		for _, p := range confirmPatterns {
			if strings.Contains(titleLower, p) {
				isConfirm = true
				break
			}
		}
		if isConfirm && !hasSubstantiveChoice(dec.Decision) {
			log.Printf("[Filter] Skipping confirm-only decision: %s", dec.Title)
			continue
		}

		result = append(result, dec)
	}

	return result
}

// hasSubstantiveChoice 判断文本是否包含实质性技术选型
func hasSubstantiveChoice(text string) bool {
	choicePatterns := []string{
		"使用", "采用", "选用", "选择", "决定", "替换",
		"use", "adopt", "select", "replace", "switch",
		"PostgreSQL", "MySQL", "Redis", "Kafka", "RabbitMQ",
		"Docker", "Kubernetes", "Go", "Python", "Java",
	}
	textLower := strings.ToLower(text)
	for _, p := range choicePatterns {
		if strings.Contains(textLower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// groupDecisionsByRelation 将相关决策分组
// 判定规则：相同 project_phase + 相同 topic → 同组
func groupDecisionsByRelation(decisions []llm.DecisionExtract) [][]llm.DecisionExtract {
	type groupKey struct {
		Phase string
		Topic string
	}

	groups := make(map[groupKey][]llm.DecisionExtract)
	var keyOrder []groupKey

	for _, dec := range decisions {
		key := groupKey{
			Phase: normalizePhase(dec.ProjectPhase),
			Topic: dec.SuggestedTopic,
		}
		if _, exists := groups[key]; !exists {
			keyOrder = append(keyOrder, key)
		}
		groups[key] = append(groups[key], dec)
	}

	var result [][]llm.DecisionExtract
	for _, key := range keyOrder {
		result = append(result, groups[key])
	}
	return result
}

// normalizePhase 标准化项目阶段名称
func normalizePhase(phase string) string {
	if phase == "" {
		return "_unknown"
	}
	// 提取阶段编号，如 "Phase 1: 数据库迁移" → "phase_1"
	lower := strings.ToLower(phase)
	for i := 1; i <= 10; i++ {
		if strings.Contains(lower, fmt.Sprintf("phase %d", i)) || strings.Contains(lower, fmt.Sprintf("阶段%d", i)) {
			return fmt.Sprintf("phase_%d", i)
		}
	}
	return lower
}

// mergeDecisionGroup 合并一组相关决策为一条
func mergeDecisionGroup(group []llm.DecisionExtract) llm.DecisionExtract {
	if len(group) == 0 {
		return llm.DecisionExtract{}
	}
	if len(group) == 1 {
		return group[0]
	}

	// 以第一个决策为基础
	base := group[0]

	// 选择最具体的标题（最长的）
	bestTitle := base.Title
	bestDecision := base.Decision
	bestRationale := base.Rationale
	bestImpact := base.ImpactLevel

	var details []string
	var executors []string
	var deadlines []string

	if base.Executor != "" {
		executors = append(executors, base.Executor)
	}
	if base.Deadline != "" {
		deadlines = append(deadlines, base.Deadline)
	}

	for _, dec := range group[1:] {
		// 选更长的标题
		if len(dec.Title) > len(bestTitle) {
			bestTitle = dec.Title
		}
		// 合并决策内容
		if dec.Decision != "" && !strings.Contains(bestDecision, dec.Decision) {
			details = append(details, dec.Decision)
		}
		// 合并依据
		if dec.Rationale != "" && !strings.Contains(bestRationale, dec.Rationale) {
			if bestRationale != "" {
				bestRationale += "; "
			}
			bestRationale += dec.Rationale
		}
		// 选更高的影响级别
		if impactLevelRank(dec.ImpactLevel) > impactLevelRank(bestImpact) {
			bestImpact = dec.ImpactLevel
		}
		// 收集执行者
		if dec.Executor != "" {
			executors = appendUniqueStr(executors, dec.Executor)
		}
		// 收集截止时间
		if dec.Deadline != "" {
			deadlines = appendUniqueStr(deadlines, dec.Deadline)
		}
	}

	// 合并决策内容
	if len(details) > 0 {
		bestDecision = bestDecision + "（实施细节：" + strings.Join(details, "；") + "）"
	}

	// 合并执行者
	if len(executors) > 0 {
		base.Executor = strings.Join(executors, "、")
	}

	// 取最早的截止时间
	if len(deadlines) > 0 {
		base.Deadline = deadlines[0]
	}

	base.Title = bestTitle
	base.Decision = bestDecision
	base.Rationale = bestRationale
	base.ImpactLevel = bestImpact

	return base
}

func impactLevelRank(level string) int {
	switch level {
	case "critical":
		return 4
	case "major":
		return 3
	case "minor":
		return 2
	case "advisory":
		return 1
	default:
		return 0
	}
}

func appendUniqueStr(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
