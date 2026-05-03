package signal

import (
	"log"
	"strings"

	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/decision"
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

			if result.HasDecision && result.Confidence > 0.5 && result.Decision != nil {
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
			} else {
				log.Printf("[SignalEngine] LLM didn't find a confident decision, using fallback (HasDecision=%v, Confidence=%.2f)",
					result.HasDecision, result.Confidence)
				newNode = e.createDecisionFallback(proposer, content)
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

func matchDecisionKeywords(text string) []string {
	decisionWords := []string{
		"决定", "decided", "确认", "LGTM", "lgtm",
		"approve", "通过", "定下来", "就这么办", "confirmed",
		"倾向于", "建议", "选择", "推荐", "选", "用",
	}
	var matched []string
	for _, w := range decisionWords {
		if strings.Contains(text, w) {
			matched = append(matched, w)
		}
	}
	return matched
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
