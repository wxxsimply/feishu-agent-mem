package decision

import "time"

// DecisionNode 决策节点 — 与 decision-tree.md §2.1 严格对齐
type DecisionNode struct {
	// === 标识与内容 ===
	SDRID            string       `json:"sdr_id" yaml:"sdr_id"`
	GitCommitHash    string       `json:"git_commit_hash" yaml:"git_commit_hash"`
	PreviousCommitHash string      `json:"previous_commit_hash,omitempty" yaml:"previous_commit_hash,omitempty"`
	Title           string       `json:"title" yaml:"title"`
	Decision        string       `json:"decision" yaml:"decision"`
	Rationale       string       `json:"rationale" yaml:"rationale"`

	// === 树位置 ===
	Project string `json:"project" yaml:"project"`
	Topic   string `json:"topic" yaml:"topic"` // 唯一位置锚点

	// === 时态标签 ===
	Phase       string      `json:"phase" yaml:"phase"`
	PhaseScope  PhaseScope  `json:"phase_scope" yaml:"phase_scope"`
	VersionRange VersionRange `json:"version_range" yaml:"version_range"`

	// === 时间关联（用于项目阶段追踪）===
	ProjectPhase  string  `json:"project_phase,omitempty" yaml:"project_phase,omitempty"`   // 项目阶段标识（如 "Phase 1: 数据库迁移"）
	DecisionTime  string  `json:"decision_time,omitempty" yaml:"decision_time,omitempty"`   // 决策时间点（如 "2026-05-06"、"上周五"）
	EffectiveTime string  `json:"effective_time,omitempty" yaml:"effective_time,omitempty"` // 生效时间
	Deadline      string  `json:"deadline,omitempty" yaml:"deadline,omitempty"`             // 截止时间
	DecisionType  string  `json:"decision_type,omitempty" yaml:"decision_type,omitempty"`   // 决策类型（new/confirmation/rejection）

	// === 影响级别 ===
	ImpactLevel ImpactLevel `json:"impact_level" yaml:"impact_level"`

	// === 跨议题 ===
	CrossTopicRefs []string `json:"cross_topic_refs" yaml:"cross_topic_refs"`

	// === 树内父子 ===
	ParentDecision string `json:"parent_decision" yaml:"parent_decision"`
	ChildrenCount  int    `json:"children_count" yaml:"children_count"`

	// === 人员 ===
	Proposer     string   `json:"proposer" yaml:"proposer"`
	Executor     string   `json:"executor" yaml:"executor"`
	Stakeholders []string `json:"stakeholders" yaml:"stakeholders"`

	// === 关系图谱 ===
	Relations []Relation `json:"relations" yaml:"relations"`

	// === 飞书关联 ===
	FeishuLinks FeishuLinks `json:"feishu_links" yaml:"feishu_links"`

	// === 反对意见 ===
	ObjectionIDs []string `json:"objection_ids,omitempty" yaml:"objection_ids,omitempty"`

	// === 状态 ===
	Status    DecisionStatus `json:"status" yaml:"status"`
	CreatedAt time.Time      `json:"created_at" yaml:"created_at"`
	DecidedAt *time.Time     `json:"decided_at" yaml:"decided_at"`

	// === 访问统计（用于热点值计算） ===
	AccessStats AccessStats `json:"access_stats" yaml:"access_stats"`
}

// PhaseScope 阶段范围
type PhaseScope string

const (
	PhaseScopePoint       PhaseScope = "Point"
	PhaseScopeSpan        PhaseScope = "Span"
	PhaseScopeRetroactive PhaseScope = "Retroactive"
)

// ImpactLevel 影响级别（取代 tree_level）
type ImpactLevel string

const (
	ImpactAdvisory ImpactLevel = "advisory"
	ImpactMinor    ImpactLevel = "minor"
	ImpactMajor    ImpactLevel = "major"
	ImpactCritical ImpactLevel = "critical"
)

// DecisionStatus 决策状态
type DecisionStatus string

const (
	StatusPending              DecisionStatus = "pending"
	StatusInDiscussion         DecisionStatus = "in_discussion"
	StatusDecided              DecisionStatus = "decided"
	StatusExecuting            DecisionStatus = "executing"
	StatusCompleted            DecisionStatus = "completed"
	StatusShelved              DecisionStatus = "shelved"
	StatusRejected             DecisionStatus = "rejected"
	StatusSuperseded           DecisionStatus = "superseded"
	StatusDeprecated           DecisionStatus = "deprecated"
	StatusPendingConfirmation DecisionStatus = "pending_confirmation"
)

// VersionRange 版本范围
type VersionRange struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"` // null = 当前生效
}

// FeishuLinks 飞书实体关联
type FeishuLinks struct {
	RelatedChatIDs      []string `json:"related_chat_ids" yaml:"related_chat_ids"`
	RelatedMessageIDs []string `json:"related_message_ids" yaml:"related_message_ids"`
	RelatedDocTokens  []string `json:"related_doc_tokens" yaml:"related_doc_tokens"`
	RelatedEventIDs   []string `json:"related_event_ids" yaml:"related_event_ids"`
	RelatedMeetingIDs []string `json:"related_meeting_ids" yaml:"related_meeting_ids"`
	RelatedTaskGUIDs []string `json:"related_task_guids" yaml:"related_task_guids"`
	RelatedMinuteTokens []string `json:"related_minute_tokens" yaml:"related_minute_tokens"`
	RelatedCommentIDs   []string `json:"related_comment_ids,omitempty" yaml:"related_comment_ids,omitempty"`
}

// AccessStats 访问统计（用于热点值计算）
type AccessStats struct {
	LastAccessedAt *time.Time `json:"last_accessed_at" yaml:"last_accessed_at"`
	AccessCount    int        `json:"access_count" yaml:"access_count"`
	ReferenceCount int        `json:"reference_count" yaml:"reference_count"`
	HotScore       float64    `json:"hot_score" yaml:"hot_score"`
	LastCalculated *time.Time `json:"last_calculated" yaml:"last_calculated"`
}

// RecordAccess 记录一次访问
func (a *AccessStats) RecordAccess() {
	now := time.Now()
	a.LastAccessedAt = &now
	a.AccessCount++
}

// RecordReference 记录一次被引用
func (a *AccessStats) RecordReference() {
	a.ReferenceCount++
}

// NewDecisionNode 创建新的决策节点
func NewDecisionNode(sdrID, title, project, topic string) *DecisionNode {
	now := time.Now()
	return &DecisionNode{
		SDRID:         sdrID,
		Title:         title,
		Project:       project,
		Topic:         topic,
		PhaseScope:    PhaseScopePoint,
		ImpactLevel:   ImpactMinor,
		Status:        StatusPending,
		CreatedAt:     now,
		Relations:     make([]Relation, 0),
		CrossTopicRefs: make([]string, 0),
		Stakeholders:  make([]string, 0),
		AccessStats: AccessStats{
			AccessCount:    0,
			ReferenceCount: 0,
			HotScore:       100, // 新决策初始热点值为最大值
			LastCalculated: &now,
		},
	}
}

// IsActive 检查决策是否处于活动状态
func (d *DecisionNode) IsActive() bool {
	switch d.Status {
	case StatusPending, StatusPendingConfirmation, StatusInDiscussion, StatusDecided, StatusExecuting:
		return true
	default:
		return false
	}
}

// IsValid 检查状态是否有效
func (s DecisionStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusPendingConfirmation, StatusInDiscussion, StatusDecided, StatusExecuting,
		StatusCompleted, StatusShelved, StatusRejected, StatusSuperseded, StatusDeprecated:
		return true
	default:
		return false
	}
}

// IsValid 检查影响级别是否有效
func (i ImpactLevel) IsValid() bool {
	switch i {
	case ImpactAdvisory, ImpactMinor, ImpactMajor, ImpactCritical:
		return true
	default:
		return false
	}
}
