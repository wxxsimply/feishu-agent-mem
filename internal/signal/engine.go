package signal

import (
	"log"
	"strings"

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

type SignalActivationEngine struct {
	Emitters     map[AdapterType]StateChangeEmitter
	Router       *ActivationRouter
	Assembler    *ContextAssembler
	StateMachine *DecisionStateMachine
	Patterns     *PatternMatcher
	detector     *EnhancedDetector // 增强型多因子检测器
	Pipeline     PipelineInterface
	Memory       MemoryGraphInterface
	llmAgent     *llm.MemoryAgent
}

func NewSignalActivationEngine(pipeline PipelineInterface, memory MemoryGraphInterface) *SignalActivationEngine {
	return &SignalActivationEngine{
		Emitters:     NewEmitters(),
		Router:       NewActivationRouter(),
		Assembler:    NewContextAssembler(),
		StateMachine: NewDecisionStateMachine(),
		Patterns:     NewPatternMatcher(),
		Pipeline:     pipeline,
		Memory:       memory,
		llmAgent:     llm.NewMemoryAgent(),
	}
}

func (e *SignalActivationEngine) OnDetectResult(adapter AdapterType, result *larkadapter.DetectResult) (*ProcessingReport, error) {
	log.Println("[SignalEngine] OnDetectResult called")
	log.Printf("[SignalEngine] Detector: %s, %d changes", adapter, len(result.Changes))
	return nil, nil
}

func (e *SignalActivationEngine) ProcessSignalForJob(sig *StateChangeSignal, proposer, content string) (*DecisionMutation, error) {
	log.Println("========== SIGNAL ENGINE PROCESS ==========")
	log.Printf("[SignalEngine] ProcessSignalForJob called")
	log.Printf("[SignalEngine] Proposer: %s", proposer)
	log.Printf("[SignalEngine] Content: %s", truncateForLog(content, 200))
	log.Printf("[SignalEngine] LLM available: %v", e.llmAgent.IsAvailable())

	var newNode *decision.DecisionNode

	if e.llmAgent.IsAvailable() {
		log.Println("[SignalEngine] LLM is available, calling...")

		topics := e.getAllTopics()
		log.Printf("[SignalEngine] Available topics: %v", topics)

		log.Println("[SignalEngine] Calling ExtractDecision...")
		result, err := e.llmAgent.ExtractDecision(content, topics)
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

				log.Printf("[SignalEngine] Decision extracted from LLM: %s", result.Decision.Title)
			} else if !result.HasDecision {
				log.Printf("[SignalEngine] LLM determined no decision, skipping entirely")
				log.Println("========== SIGNAL ENGINE END ==========")
				return nil, nil
			} else {
				log.Printf("[SignalEngine] LLM confidence too low (%.2f < 0.6), skipping",
					result.Confidence)
				log.Println("========== SIGNAL ENGINE END ==========")
				return nil, nil
			}
		}
	} else {
		log.Println("[SignalEngine] LLM not available, using heuristic fallback")
		newNode = e.createDecisionFallback(proposer, content)
	}

	mut := e.StateMachine.CreateMutationForNewDecision(newNode, sig)

	log.Printf("[SignalEngine] Created mutation: Type=%s, SDRID=%s", mut.Type, mut.SDRID)
	log.Println("========== SIGNAL ENGINE END ==========")
	return mut, nil
}

// ProcessSignalForDocJob 处理文档类型信号的决策提取（使用分阶段分析 prompt）
func (e *SignalActivationEngine) ProcessSignalForDocJob(sig *StateChangeSignal, proposer, content, docType, title string) (*DecisionMutation, error) {
	log.Println("========== SIGNAL ENGINE DOC PROCESS ==========")
	log.Printf("[SignalEngine] ProcessSignalForDocJob called")
	log.Printf("[SignalEngine] Proposer: %s", proposer)
	log.Printf("[SignalEngine] DocType: %s, Title: %s", docType, title)
	log.Printf("[SignalEngine] Content length: %d", len(content))
	log.Printf("[SignalEngine] LLM available: %v", e.llmAgent.IsAvailable())

	var newNode *decision.DecisionNode

	if e.llmAgent.IsAvailable() {
		log.Println("[SignalEngine] LLM is available, calling ExtractDecisionFromDoc...")

		topics := e.getAllTopics()
		log.Printf("[SignalEngine] Available topics: %v", topics)

		result, err := e.llmAgent.ExtractDecisionFromDoc(content, topics, docType, title)
		if err != nil {
			log.Printf("[SignalEngine] LLM doc extraction failed: %v, falling back to heuristic", err)
			newNode = e.createDecisionFallback(proposer, content)
		} else {
			log.Printf("[SignalEngine] LLM doc extraction result: HasDecision=%v, Confidence=%.2f",
				result.HasDecision, result.Confidence)

			if result.Decision != nil {
				log.Printf("[SignalEngine] Doc decision details: Title=%s, Topic=%s",
					result.Decision.Title, result.Decision.SuggestedTopic)
			}

			if result.HasDecision && result.Confidence >= 0.6 && result.Decision != nil {
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

				log.Printf("[SignalEngine] Decision extracted from doc: %s", result.Decision.Title)
			} else if !result.HasDecision {
				log.Printf("[SignalEngine] LLM determined no decision in doc, skipping entirely")
				log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
				return nil, nil
			} else {
				log.Printf("[SignalEngine] LLM confidence too low (%.2f < 0.6), skipping",
					result.Confidence)
				log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
				return nil, nil
			}
		}
	} else {
		log.Println("[SignalEngine] LLM not available, using heuristic fallback")
		newNode = e.createDecisionFallback(proposer, content)
	}

	mut := e.StateMachine.CreateMutationForNewDecision(newNode, sig)

	log.Printf("[SignalEngine] Created mutation: Type=%s, SDRID=%s", mut.Type, mut.SDRID)
	log.Println("========== SIGNAL ENGINE DOC PROCESS END ==========")
	return mut, nil
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

type PatternMatcher struct{}

func NewPatternMatcher() *PatternMatcher {
	return &PatternMatcher{}
}
