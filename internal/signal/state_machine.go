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
	Type          MutationType            `json:"type"`
	SDRID         string                  `json:"sdr_id"`
	Node          *decision.DecisionNode  `json:"node,omitempty"`
	FieldChanges  map[string]any          `json:"field_changes,omitempty"`
	NewStatus     decision.DecisionStatus `json:"new_status,omitempty"`
	CommitMessage string                  `json:"commit_message"`
}

// MutationType 变更类型
type MutationType string

const (
	MutationCreate       MutationType = "create"
	MutationUpdate       MutationType = "update"
	MutationStatusChange MutationType = "status_change"
	MutationConflict     MutationType = "conflict"
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

// GenerateSDRID 生成决策 ID
func GenerateSDRID() string {
	idMutex.Lock()
	idCounter++
	idMutex.Unlock()

	return "DEC-" + time.Now().Format("20060102150405") + "-" + string([]byte{byte('0' + idCounter%10)})
}
