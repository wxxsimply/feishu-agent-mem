package decision

import "time"

// ObjectionStatus 反对意见状态
type ObjectionStatus string

const (
	ObjectionActive    ObjectionStatus = "active"
	ObjectionResolved  ObjectionStatus = "resolved"
	ObjectionOverruled ObjectionStatus = "overruled"
)

// Objection 反对意见 — 轻量级记录，关联到决策
type Objection struct {
	OID              string          `json:"oid" yaml:"oid"`
	ObjectionContent string          `json:"objection_content" yaml:"objection_content"`
	Rationale        string          `json:"rationale,omitempty" yaml:"rationale,omitempty"`
	Alternative      string          `json:"alternative,omitempty" yaml:"alternative,omitempty"`
	Objector         string          `json:"objector" yaml:"objector"`
	Status           ObjectionStatus `json:"status" yaml:"status"`

	// Linking
	ReferencesDecision string `json:"references_decision,omitempty" yaml:"references_decision,omitempty"`

	// Source tracking
	SourceType      string `json:"source_type" yaml:"source_type"` // "comment" | "im" | "doc_content" | "meeting"
	SourceDocToken  string `json:"source_doc_token,omitempty" yaml:"source_doc_token,omitempty"`
	SourceMessageID string `json:"source_message_id,omitempty" yaml:"source_message_id,omitempty"`
	SourceChatID    string `json:"source_chat_id,omitempty" yaml:"source_chat_id,omitempty"`

	Topic     string    `json:"topic" yaml:"topic"`
	Project   string    `json:"project" yaml:"project"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}

// IsValid 检查反对意见状态是否有效
func (s ObjectionStatus) IsValid() bool {
	switch s {
	case ObjectionActive, ObjectionResolved, ObjectionOverruled:
		return true
	default:
		return false
	}
}
