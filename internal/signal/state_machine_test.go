package signal

import (
	"testing"

	"feishu-mem/internal/decision"
)

func TestGenerateSDRID_NotEmpty(t *testing.T) {
	id := GenerateSDRID()
	if id == "" {
		t.Error("SDRID should not be empty")
	}
}

func TestGenerateSDRID_Prefix(t *testing.T) {
	id := GenerateSDRID()
	if len(id) < 4 || id[:4] != "DEC-" {
		t.Errorf("SDRID should start with 'DEC-', got %q", id)
	}
}

func TestGenerateSDRID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	// Counter is idCounter % 10, so at most 10 unique per second
	for i := 0; i < 10; i++ {
		id := GenerateSDRID()
		if ids[id] {
			t.Errorf("Duplicate SDRID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestDecisionStateMachine_New(t *testing.T) {
	sm := NewDecisionStateMachine()
	if sm == nil {
		t.Fatal("NewDecisionStateMachine should not return nil")
	}
}

func TestStateMachine_EvaluateTransition_NilNode(t *testing.T) {
	sm := NewDecisionStateMachine()
	transitions := sm.EvaluateTransition(nil, nil, nil)
	// Go nil slice is valid to range over, len(nil) == 0
	if len(transitions) > 0 {
		t.Errorf("Nil node should return no transitions, got %d", len(transitions))
	}
}

func TestStateMachine_EvaluateTransition_StrongSignal(t *testing.T) {
	sm := NewDecisionStateMachine()
	node := &decision.DecisionNode{
		SDRID:  "DEC-001",
		Title:  "Test",
		Status: decision.StatusPending,
	}
	signal := &StateChangeSignal{
		SignalID: "SIG-001",
		Strength: StrengthStrong,
	}

	transitions := sm.EvaluateTransition(node, signal, nil)

	if len(transitions) == 0 {
		t.Fatal("Strong signal should trigger transition")
	}
	if transitions[0].FromStatus != decision.StatusPending {
		t.Errorf("FromStatus = %q, want %q", transitions[0].FromStatus, decision.StatusPending)
	}
	if transitions[0].ToStatus != decision.StatusInDiscussion {
		t.Errorf("ToStatus = %q, want %q", transitions[0].ToStatus, decision.StatusInDiscussion)
	}
	if transitions[0].TriggerSignal != "SIG-001" {
		t.Errorf("TriggerSignal = %q, want %q", transitions[0].TriggerSignal, "SIG-001")
	}
}

func TestStateMachine_EvaluateTransition_WeakSignal(t *testing.T) {
	sm := NewDecisionStateMachine()
	node := &decision.DecisionNode{
		SDRID:  "DEC-002",
		Title:  "Test",
		Status: decision.StatusPending,
	}
	signal := &StateChangeSignal{
		SignalID: "SIG-002",
		Strength: StrengthWeak,
	}

	transitions := sm.EvaluateTransition(node, signal, nil)
	if len(transitions) != 0 {
		t.Errorf("Weak signal should not trigger transition, got %d", len(transitions))
	}
}

func TestStateMachine_EvaluateTransition_NonPending(t *testing.T) {
	sm := NewDecisionStateMachine()
	node := &decision.DecisionNode{
		SDRID:  "DEC-003",
		Title:  "Test",
		Status: decision.StatusDecided,
	}
	signal := &StateChangeSignal{
		SignalID: "SIG-003",
		Strength: StrengthStrong,
	}

	transitions := sm.EvaluateTransition(node, signal, nil)
	// Strong signal only triggers transition from StatusPending → StatusInDiscussion
	// Other statuses should not transition
	if len(transitions) != 0 {
		t.Errorf("Non-pending status should not transition, got %d: %+v", len(transitions), transitions)
	}
}

func TestCreateMutationForNewDecision(t *testing.T) {
	sm := NewDecisionStateMachine()
	node := &decision.DecisionNode{
		SDRID:  "DEC-001",
		Title:  "Test Decision",
		Status: decision.StatusPending,
	}
	signal := &StateChangeSignal{
		SignalID: "SIG-001",
	}

	mut := sm.CreateMutationForNewDecision(node, signal)
	if mut == nil {
		t.Fatal("Mutation should not be nil")
	}
	if mut.Type != MutationCreate {
		t.Errorf("Type = %q, want %q", mut.Type, MutationCreate)
	}
	if mut.SDRID != "DEC-001" {
		t.Errorf("SDRID = %q, want %q", mut.SDRID, "DEC-001")
	}
	if mut.Node != node {
		t.Error("Node should be the same pointer")
	}
	if mut.CommitMessage == "" {
		t.Error("CommitMessage should not be empty")
	}
}

func TestCreateMutationForStatusChange(t *testing.T) {
	sm := NewDecisionStateMachine()
	mut := sm.CreateMutationForStatusChange("DEC-001", decision.StatusPending, decision.StatusDecided, "Approved")

	if mut == nil {
		t.Fatal("Mutation should not be nil")
	}
	if mut.Type != MutationStatusChange {
		t.Errorf("Type = %q, want %q", mut.Type, MutationStatusChange)
	}
	if mut.SDRID != "DEC-001" {
		t.Errorf("SDRID = %q, want %q", mut.SDRID, "DEC-001")
	}
	if mut.NewStatus != decision.StatusDecided {
		t.Errorf("NewStatus = %q, want %q", mut.NewStatus, decision.StatusDecided)
	}
	if mut.CommitMessage == "" {
		t.Error("CommitMessage should not be empty")
	}
}

func TestMutationTypeConstants(t *testing.T) {
	if MutationCreate != "create" {
		t.Errorf("MutationCreate = %q, want %q", MutationCreate, "create")
	}
	if MutationUpdate != "update" {
		t.Errorf("MutationUpdate = %q, want %q", MutationUpdate, "update")
	}
	if MutationStatusChange != "status_change" {
		t.Errorf("MutationStatusChange = %q, want %q", MutationStatusChange, "status_change")
	}
	if MutationConflict != "conflict" {
		t.Errorf("MutationConflict = %q, want %q", MutationConflict, "conflict")
	}
}

func TestDecisionMutation_StatusChangeFields(t *testing.T) {
	mut := &DecisionMutation{
		Type:      MutationStatusChange,
		SDRID:     "DEC-001",
		NewStatus: decision.StatusCompleted,
	}
	if mut.SDRID != "DEC-001" {
		t.Errorf("SDRID = %q, want %q", mut.SDRID, "DEC-001")
	}
	if mut.NewStatus != decision.StatusCompleted {
		t.Errorf("NewStatus = %q, want %q", mut.NewStatus, decision.StatusCompleted)
	}
	if mut.Node != nil {
		t.Error("Status change should not have a node")
	}
}

func TestDecisionMutation_UpdateFields(t *testing.T) {
	mut := &DecisionMutation{
		Type:         MutationUpdate,
		SDRID:        "DEC-001",
		FieldChanges: map[string]any{"title": "New Title"},
	}
	if mut.FieldChanges == nil {
		t.Fatal("FieldChanges should not be nil")
	}
	if mut.FieldChanges["title"] != "New Title" {
		t.Errorf("title = %q, want %q", mut.FieldChanges["title"], "New Title")
	}
}
