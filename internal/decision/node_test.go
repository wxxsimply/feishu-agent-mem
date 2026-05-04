package decision

import (
	"testing"
	"time"
)

func TestNewDecisionNode(t *testing.T) {
	node := NewDecisionNode("DEC-001", "使用PostgreSQL", "feishu-mem", "数据库架构")

	if node.SDRID != "DEC-001" {
		t.Errorf("SDRID = %q, want %q", node.SDRID, "DEC-001")
	}
	if node.Title != "使用PostgreSQL" {
		t.Errorf("Title = %q, want %q", node.Title, "使用PostgreSQL")
	}
	if node.Project != "feishu-mem" {
		t.Errorf("Project = %q, want %q", node.Project, "feishu-mem")
	}
	if node.Topic != "数据库架构" {
		t.Errorf("Topic = %q, want %q", node.Topic, "数据库架构")
	}
	if node.PhaseScope != PhaseScopePoint {
		t.Errorf("PhaseScope = %q, want %q", node.PhaseScope, PhaseScopePoint)
	}
	if node.ImpactLevel != ImpactMinor {
		t.Errorf("ImpactLevel = %q, want %q", node.ImpactLevel, ImpactMinor)
	}
	if node.Status != StatusPending {
		t.Errorf("Status = %q, want %q", node.Status, StatusPending)
	}
	if node.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if node.Relations == nil {
		t.Error("Relations should be initialized, not nil")
	}
	if node.CrossTopicRefs == nil {
		t.Error("CrossTopicRefs should be initialized, not nil")
	}
	if node.Stakeholders == nil {
		t.Error("Stakeholders should be initialized, not nil")
	}
}

func TestDecisionNode_IsActive(t *testing.T) {
	tests := []struct {
		status DecisionStatus
		active bool
	}{
		{StatusPending, true},
		{StatusInDiscussion, true},
		{StatusDecided, true},
		{StatusExecuting, true},
		{StatusCompleted, false},
		{StatusShelved, false},
		{StatusRejected, false},
		{StatusSuperseded, false},
		{StatusDeprecated, false},
	}

	for _, tt := range tests {
		node := &DecisionNode{Status: tt.status}
		got := node.IsActive()
		if got != tt.active {
			t.Errorf("IsActive() with Status=%q = %v, want %v", tt.status, got, tt.active)
		}
	}
}

func TestDecisionStatus_IsValid(t *testing.T) {
	tests := []struct {
		status DecisionStatus
		valid  bool
	}{
		{StatusPending, true},
		{StatusInDiscussion, true},
		{StatusDecided, true},
		{StatusExecuting, true},
		{StatusCompleted, true},
		{StatusShelved, true},
		{StatusRejected, true},
		{StatusSuperseded, true},
		{StatusDeprecated, true},
		{DecisionStatus("unknown"), false},
		{DecisionStatus(""), false},
		{DecisionStatus("deleted"), false},
	}

	for _, tt := range tests {
		got := tt.status.IsValid()
		if got != tt.valid {
			t.Errorf("IsValid() for Status=%q = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestImpactLevel_IsValid(t *testing.T) {
	tests := []struct {
		level ImpactLevel
		valid bool
	}{
		{ImpactAdvisory, true},
		{ImpactMinor, true},
		{ImpactMajor, true},
		{ImpactCritical, true},
		{ImpactLevel("unknown"), false},
		{ImpactLevel(""), false},
		{ImpactLevel("high"), false},
	}

	for _, tt := range tests {
		got := tt.level.IsValid()
		if got != tt.valid {
			t.Errorf("IsValid() for Level=%q = %v, want %v", tt.level, got, tt.valid)
		}
	}
}

func TestPhaseScopeConstants(t *testing.T) {
	if PhaseScopePoint != "Point" {
		t.Errorf("PhaseScopePoint = %q, want %q", PhaseScopePoint, "Point")
	}
	if PhaseScopeSpan != "Span" {
		t.Errorf("PhaseScopeSpan = %q, want %q", PhaseScopeSpan, "Span")
	}
	if PhaseScopeRetroactive != "Retroactive" {
		t.Errorf("PhaseScopeRetroactive = %q, want %q", PhaseScopeRetroactive, "Retroactive")
	}
}

func TestDecisionNode_DecidedAt(t *testing.T) {
	now := time.Now()
	node := &DecisionNode{
		SDRID:     "DEC-002",
		Title:     "test",
		DecidedAt: &now,
	}

	if node.DecidedAt == nil {
		t.Fatal("DecidedAt should not be nil")
	}
	if !node.DecidedAt.Equal(now) {
		t.Errorf("DecidedAt = %v, want %v", node.DecidedAt, now)
	}
}

func TestDecisionNode_ZeroValues_Pending(t *testing.T) {
	node := &DecisionNode{
		SDRID: "DEC-003",
		Title: "zero-value test",
	}

	if node.Status != "" {
		t.Errorf("Default Status should be empty, got %q", node.Status)
	}
	if node.ImpactLevel != "" {
		t.Errorf("Default ImpactLevel should be empty, got %q", node.ImpactLevel)
	}
	if node.Relations != nil {
		t.Error("Default Relations should be nil for zero-value struct")
	}
}
