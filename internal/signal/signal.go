package signal

import (
	"time"
)

// AdapterType 适配器类型
type AdapterType string

const (
	AdapterIM       AdapterType = "IM"
	AdapterVC       AdapterType = "VC"
	AdapterDocs     AdapterType = "Docs"
	AdapterCalendar AdapterType = "Calendar"
	AdapterTask     AdapterType = "Task"
	AdapterOKR      AdapterType = "OKR"
	AdapterContact  AdapterType = "Contact"
	AdapterWiki     AdapterType = "Wiki"
)

// ChangeType 变更类型
type ChangeType string

const (
	ChangeCreated      ChangeType = "created"
	ChangeUpdated      ChangeType = "updated"
	ChangeDeleted      ChangeType = "deleted"
	ChangeStatusChange ChangeType = "status_change"
)

// SignalStrength 信号强度
type SignalStrength string

const (
	StrengthStrong SignalStrength = "strong"
	StrengthMedium SignalStrength = "medium"
	StrengthWeak   SignalStrength = "weak"
)

// StateChangeSignal 每个适配器检测到变化时输出的标准信号
type StateChangeSignal struct {
	SignalID      string         `json:"signal_id"`
	Adapter       AdapterType    `json:"adapter"`
	Timestamp     time.Time      `json:"timestamp"`
	EventType     string         `json:"event_type"`
	ChangeType    ChangeType     `json:"change_type"`
	ChangeSummary string         `json:"change_summary"`
	PrimaryID     string         `json:"primary_id"`
	RelatedIDs    []string       `json:"related_ids"`
	CommentID     string         `json:"comment_id,omitempty"`
	Context       SignalContext  `json:"context"`
	Strength      SignalStrength `json:"strength"`
}

// SignalContext 信号上下文
type SignalContext struct {
	Keywords        []string      `json:"keywords"`
	DecisionSignals []string      `json:"decision_signals"`
	MentionedIDs    []string      `json:"mentioned_ids"`
	ActorID         string        `json:"actor_id"`
	ParticipantIDs  []string      `json:"participant_ids"`
	EmbeddedURLs    []EmbeddedURL `json:"embedded_urls"`
	ContentSnippet  string        `json:"content_snippet"`
	EventTime       time.Time     `json:"event_time"`
	Score           float64       `json:"score"`       // 检测器综合评分
	IsDecision      bool          `json:"is_decision"` // 是否被判定为决策
}

// EmbeddedURL 嵌入的 URL
type EmbeddedURL struct {
	RawURL         string `json:"raw_url"`
	ExtractedToken string `json:"extracted_token"`
	URLType        string `json:"url_type"` // doc | wiki | meeting | minute | sheet | bitable
}

// NewSignal 创建新信号
func NewSignal(adapter AdapterType, summary string) *StateChangeSignal {
	return &StateChangeSignal{
		SignalID:      generateID(),
		Adapter:       adapter,
		Timestamp:     time.Now(),
		ChangeSummary: summary,
		Strength:      StrengthMedium,
		RelatedIDs:    []string{},
	}
}

func generateID() string {
	// 简单的 ID 生成
	return "sig-" + time.Now().Format("20060102150405")
}

// Conflict 冲突
type Conflict struct {
	ConflictID         string
	DecisionA          string
	DecisionB          string
	ContradictionScore float64
	Description        string
}

// ProcessingReport 处理报告
type ProcessingReport struct {
	SignalID      string
	Mutations     []*DecisionMutation
	Conflicts     []Conflict
	TokenUsage    int
	DecisionCount int
}
