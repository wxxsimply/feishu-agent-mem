// internal/llm/types.go

package llm

import (
	"feishu-mem/internal/decision"
	"time"
)

// ========== 请求/响应类型 ==========

// ObjectionExtract 提取的反对意见
type ObjectionExtract struct {
	ObjectionContent string `json:"objection_content"`
	Rationale        string `json:"rationale,omitempty"`
	Alternative      string `json:"alternative,omitempty"`
	Objector         string `json:"objector"`
	Source           string `json:"source"` // "im" | "comment" | "doc" | "meeting"
}

// ExtractionResult 决策提取结果
type ExtractionResult struct {
	HasDecision   bool             `json:"has_decision"`
	Confidence    float64          `json:"confidence"`
	Decision      *DecisionExtract `json:"decision,omitempty"`
	HasObjections bool             `json:"has_objections,omitempty"`
	Objections    []ObjectionExtract `json:"objections,omitempty"`
	ExtractedFrom string           `json:"extracted_from"`
}

// DecisionExtract 提取的决策
type DecisionExtract struct {
	Title           string           `json:"title"`
	Decision        string           `json:"decision"`
	Rationale       string           `json:"rationale"`
	SuggestedTopic  string           `json:"suggested_topic"`
	ImpactLevel     string           `json:"impact_level"`
	PhaseScope      string           `json:"phase_scope,omitempty"`
	Proposer        string           `json:"proposer"`
	Executor        string           `json:"executor"`
	DecisionType    string           `json:"decision_type,omitempty"`
	RelatedEntities RelatedEntities  `json:"related_entities"`
}

// RelatedEntities 相关实体
type RelatedEntities struct {
	ChatIDs      []string `json:"chat_ids"`
	DocTokens    []string `json:"doc_tokens"`
	MeetingIDs   []string `json:"meeting_ids"`
	TaskGUIDs    []string `json:"task_guids"`
	EventIDs     []string `json:"event_ids"`
}

// ClassificationResult 议题分类结果
type ClassificationResult struct {
	Topic              string   `json:"topic"`
	Confidence         float64  `json:"confidence"`
	Reasoning          string   `json:"reasoning"`
	AlternativeTopics  []string `json:"alternative_topics,omitempty"`
}

// CrossTopicResult 跨议题检测结果
type CrossTopicResult struct {
	IsCrossTopic  bool              `json:"is_cross_topic"`
	CrossTopicRefs []string        `json:"cross_topic_refs,omitempty"`
	Reasons      map[string]string `json:"reasons,omitempty"`
	Confidence   float64           `json:"confidence"`
}

// ConflictResult 冲突评估结果
type ConflictResult struct {
	ContradictionScore float64 `json:"contradiction_score"`
	ContradictionType  string  `json:"contradiction_type"`
	Description        string  `json:"description"`
	Suggestion         string  `json:"suggestion,omitempty"`
	Action             string  `json:"action"`
	NeedsUser          bool    `json:"needs_user"`
}

// DedupActionType LLM 去重+冲突联合判断结果（用于 EvaluateDedupAction 的 JSON Schema）
type DedupActionType struct {
	Action string `json:"action" jsonschema_description:"skip/update/conflict — 三选一"`
	Reason string `json:"reason" jsonschema_description:"判断理由"`
}

// ConflictResolveResult LLM 冲突解决结果（用于 ResolveConflict 的 JSON Schema）
type ConflictResolveResult struct {
	Action         string `json:"action" jsonschema_description:"merge 或 keep_both"`
	MergedDecision string `json:"merged_decision" jsonschema_description:"合并后的决策内容，仅 action=merge 时需要"`
	Reason         string `json:"reason" jsonschema_description:"判断理由"`
}

// DecisionResult 决策处理结果
type DecisionResult struct {
	Decision   *decision.DecisionNode
	Topic      *ClassificationResult
	CrossTopic *CrossTopicResult
	Conflicts  []ConflictResult
	CreatedAt  time.Time
}

// ========== Agent 类型 ==========

// AgentConfig Agent 配置
type AgentConfig struct {
	MaxRetries  int
	Timeout     time.Duration
	MaxTokens   int
	Temperature float64
}

// AgentState Agent 状态
type AgentState struct {
	Name       string
	Status     string // "idle" | "running" | "completed" | "failed"
	Checkpoint *Checkpoint
	TokensUsed int
	LoopCount  int
}

// Checkpoint 检查点
type Checkpoint struct {
	StepID    string
	State     map[string]any
	Timestamp time.Time
}

// Context Agent 执行上下文
type Context struct {
	Signal     any
	Content    string
	Topics     []string
	Node       any
	OtherNode  any
	Budget     any
	History    []string
	LoopCount  int
	TokensUsed int
}

// AgentResult Agent 执行结果
type AgentResult struct {
	Success    bool
	Data       any
	TokensUsed int
	Checkpoint *Checkpoint
	Error      error
}

// Agent Agent 接口
type Agent interface {
	Name() string
	Run(ctx *Context) (*AgentResult, error)
	Checkpoint() *Checkpoint
	Rollback(checkpoint *Checkpoint) error
}
