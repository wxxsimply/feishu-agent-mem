package signal

import (
	"feishu-mem/internal/decision"
	"sync"
	"time"
)

var idCounter int64
var idMutex sync.Mutex

// StateTransition 状态转换
type StateTransition struct {
	SDRID        string
	FromStatus   decision.DecisionStatus
	ToStatus     decision.DecisionStatus
	Reason       string
	TriggerSignal string
}

// DecisionMutation 决策变更操作（传给 PipelineEngine）
type DecisionMutation struct {
	Type           MutationType            `json:"type"`
	SDRID          string                  `json:"sdr_id"`
	Node           *decision.DecisionNode  `json:"node,omitempty"`
	FieldChanges   map[string]any          `json:"field_changes,omitempty"`
	NewStatus      decision.DecisionStatus `json:"new_status,omitempty"`
	ConflictSDRID  string                  `json:"conflict_sdr_id,omitempty"` // 冲突关联的决策 SDRID
	ConflictAction string                  `json:"conflict_action,omitempty"` // "merge" | "keep_both"
	ConflictReason string                  `json:"conflict_reason,omitempty"` // LLM 给出理由
	Objection      *decision.Objection     `json:"objection,omitempty"`       // 反对意见（MutationObjection 时使用）
	CommitMessage  string                  `json:"commit_message"`
}

// MutationType 变更类型
type MutationType string

const (
	MutationCreate       MutationType = "create"
	MutationUpdate       MutationType = "update"
	MutationStatusChange MutationType = "status_change"
	MutationConflict     MutationType = "conflict"
	MutationObjection    MutationType = "objection"
)

// DecisionStateMachine 决策状态机
type DecisionStateMachine struct{}

// NewDecisionStateMachine 创建状态机
func NewDecisionStateMachine() *DecisionStateMachine {
	return &DecisionStateMachine{}
}

// EvaluateTransition 评估状态转换
func (sm *DecisionStateMachine) EvaluateTransition(
	current *decision.DecisionNode,
	signal *StateChangeSignal,
	ctx *AssembledContext,
) []*StateTransition {
	var transitions []*StateTransition

	// 简化的状态转换逻辑
	if current == nil {
		// 可能需要创建新决策
		return transitions
	}

	// 根据信号评估是否需要状态变更
	if signal.Strength == StrengthStrong {
		// 强信号可能触发状态变更
		switch current.Status {
		case decision.StatusPending:
			transitions = append(transitions, &StateTransition{
				SDRID:        current.SDRID,
				FromStatus:   current.Status,
				ToStatus:     decision.StatusInDiscussion,
				Reason:       "Strong signal detected",
				TriggerSignal: signal.SignalID,
			})
		}
	}

	return transitions
}

// CreateMutationForNewDecision 创建新决策的变更
func (sm *DecisionStateMachine) CreateMutationForNewDecision(
	node *decision.DecisionNode,
	signal *StateChangeSignal,
) *DecisionMutation {
	return &DecisionMutation{
		Type:          MutationCreate,
		SDRID:         node.SDRID,
		Node:          node,
		CommitMessage: "Create decision from signal: " + signal.SignalID,
	}
}

// CreateMutationForUpdate 创建更新现有决策的变更
func (sm *DecisionStateMachine) CreateMutationForUpdate(
	sdrID string,
	node *decision.DecisionNode,
	signal *StateChangeSignal,
) *DecisionMutation {
	return &DecisionMutation{
		Type:  MutationUpdate,
		SDRID: sdrID,
		Node:  node,
		FieldChanges: map[string]any{
			"title":        node.Title,
			"decision":     node.Decision,
			"rationale":    node.Rationale,
			"impact_level": string(node.ImpactLevel),
			"executor":     node.Executor,
			"proposer":     node.Proposer,
			"status":       string(node.Status),
		},
		CommitMessage: "Update decision " + sdrID + " from signal: " + signal.SignalID,
	}
}

// CreateMutationForConflict 创建带冲突关系的新决策变更
func (sm *DecisionStateMachine) CreateMutationForConflict(
	node *decision.DecisionNode,
	conflictWithSDRID string,
	signal *StateChangeSignal,
) *DecisionMutation {
	node.Relations = append(node.Relations, decision.Relation{
		Type:        decision.RelationConflictsWith,
		TargetSDRID: conflictWithSDRID,
		Description: "Decision content changed significantly from previous version",
	})

	return &DecisionMutation{
		Type:          MutationConflict,
		SDRID:         node.SDRID,
		Node:          node,
		ConflictSDRID: conflictWithSDRID,
		CommitMessage: "Conflict detected with " + conflictWithSDRID + " from signal: " + signal.SignalID,
	}
}

// CreateMutationForConflictMerge LLM 可自动合并的冲突 — 覆盖更新已有决策
func (sm *DecisionStateMachine) CreateMutationForConflictMerge(
	sdrID string,
	node *decision.DecisionNode,
	signal *StateChangeSignal,
	resolveAction, resolveReason, mergedDecision string,
) *DecisionMutation {
	return &DecisionMutation{
		Type:           MutationConflict,
		SDRID:          sdrID,
		Node:           node,
		ConflictSDRID:  sdrID,
		ConflictAction: "merge",
		ConflictReason: resolveReason,
		CommitMessage:  "Conflict auto-merged: " + resolveReason + " (signal: " + signal.SignalID + ")",
	}
}

// CreateMutationForConflictKeepBoth LLM 无法解决的冲突 — 创建新决策并标记冲突
func (sm *DecisionStateMachine) CreateMutationForConflictKeepBoth(
	node *decision.DecisionNode,
	conflictWithSDRID string,
	signal *StateChangeSignal,
	resolveReason string,
) *DecisionMutation {
	node.Relations = append(node.Relations, decision.Relation{
		Type:        decision.RelationConflictsWith,
		TargetSDRID: conflictWithSDRID,
		Description: "Conflicts with " + conflictWithSDRID + ": " + resolveReason,
	})

	return &DecisionMutation{
		Type:           MutationConflict,
		SDRID:          node.SDRID,
		Node:           node,
		ConflictSDRID:  conflictWithSDRID,
		ConflictAction: "keep_both",
		ConflictReason: resolveReason,
		CommitMessage:  "Conflict needs manual resolution: " + resolveReason + " (signal: " + signal.SignalID + ")",
	}
}

// CreateMutationForStatusChange 创建状态变更
func (sm *DecisionStateMachine) CreateMutationForStatusChange(
	sdrID string,
	from, to decision.DecisionStatus,
	reason string,
) *DecisionMutation {
	return &DecisionMutation{
		Type:          MutationStatusChange,
		SDRID:         sdrID,
		NewStatus:     to,
		CommitMessage: "Status change: " + string(from) + " -> " + string(to) + " (" + reason + ")",
	}
}

// CreateMutationForNewObjection 创建新反对意见的变更
func (sm *DecisionStateMachine) CreateMutationForNewObjection(
	obj *decision.Objection,
	signal *StateChangeSignal,
) *DecisionMutation {
	return &DecisionMutation{
		Type:          MutationObjection,
		SDRID:         obj.OID,
		Objection:     obj,
		CommitMessage: "Create objection from signal: " + signal.SignalID,
	}
}

// GenerateSDRID 生成决策 ID
func GenerateSDRID() string {
	idMutex.Lock()
	idCounter++
	idMutex.Unlock()

	return "DEC-" + time.Now().Format("20060102150405") + "-" + string([]byte{byte('0' + idCounter%10)})
}

// GenerateObjectionID 生成反对意见 ID
func GenerateObjectionID() string {
	idMutex.Lock()
	idCounter++
	idMutex.Unlock()

	return "OBJ-" + time.Now().Format("20060102150405") + "-" + string([]byte{byte('0' + idCounter%10)})
}
