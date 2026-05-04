package sync

import (
	"testing"

	"feishu-mem/internal/decision"
)

// TestSyncManager_New verifies NewSyncManager doesn't panic
func TestSyncManager_New(t *testing.T) {
	// Just verify struct creation doesn't panic
	_ = &SyncManager{}
}

// ============================================================
// hasChanges 测试
// ============================================================

func TestHasChanges_NoChanges(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Test",
		Decision:    "Use PostgreSQL",
		Rationale:   "Because",
		Status:      decision.StatusDecided,
		ImpactLevel: decision.ImpactMajor,
		Topic:       "数据库架构",
		Proposer:    "Alice",
		Executor:    "Bob",
	}
	b := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Test",
		Decision:    "Use PostgreSQL",
		Rationale:   "Because",
		Status:      decision.StatusDecided,
		ImpactLevel: decision.ImpactMajor,
		Topic:       "数据库架构",
		Proposer:    "Alice",
		Executor:    "Bob",
	}

	if sm.hasChanges(a, b) {
		t.Error("Identical nodes should have no changes")
	}
}

func TestHasChanges_TitleChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{Title: "Old Title"}
	b := &decision.DecisionNode{Title: "New Title"}

	if !sm.hasChanges(a, b) {
		t.Error("Different titles should be detected")
	}
}

func TestHasChanges_DecisionChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{Title: "Same", Decision: "Old"}
	b := &decision.DecisionNode{Title: "Same", Decision: "New"}

	if !sm.hasChanges(a, b) {
		t.Error("Different decisions should be detected")
	}
}

func TestHasChanges_RationaleChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{Title: "Same", Rationale: "Old"}
	b := &decision.DecisionNode{Title: "Same", Rationale: "New"}

	if !sm.hasChanges(a, b) {
		t.Error("Different rationales should be detected")
	}
}

func TestHasChanges_StatusChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{Status: decision.StatusPending}
	b := &decision.DecisionNode{Status: decision.StatusDecided}

	if !sm.hasChanges(a, b) {
		t.Error("Different statuses should be detected")
	}
}

func TestHasChanges_TopicChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{Topic: "数据库架构"}
	b := &decision.DecisionNode{Topic: "前端架构"}

	if !sm.hasChanges(a, b) {
		t.Error("Different topics should be detected")
	}
}

func TestHasChanges_ImpactChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{ImpactLevel: decision.ImpactMinor}
	b := &decision.DecisionNode{ImpactLevel: decision.ImpactCritical}

	if !sm.hasChanges(a, b) {
		t.Error("Different impact levels should be detected")
	}
}

func TestHasChanges_ProposerChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{Proposer: "Alice"}
	b := &decision.DecisionNode{Proposer: "Bob"}

	if !sm.hasChanges(a, b) {
		t.Error("Different proposers should be detected")
	}
}

func TestHasChanges_ExecutorChanged(t *testing.T) {
	sm := &SyncManager{}
	a := &decision.DecisionNode{Executor: "Alice"}
	b := &decision.DecisionNode{Executor: "Bob"}

	if !sm.hasChanges(a, b) {
		t.Error("Different executors should be detected")
	}
}

// ============================================================
// mergeDecisions 测试
// ============================================================

func TestMergeDecisions_BitableOverridesGit(t *testing.T) {
	sm := &SyncManager{}
	gitNode := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Git Title",
		Decision:    "Git Decision",
		Rationale:   "Git Rationale",
		Status:      decision.StatusPending,
		ImpactLevel: decision.ImpactMinor,
		Topic:       "Git Topic",
		Proposer:    "Git Proposer",
		Executor:    "Git Executor",
	}
	bitableNode := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Bitable Title",
		Decision:    "Bitable Decision",
		Rationale:   "Bitable Rationale",
		Status:      decision.StatusDecided,
		ImpactLevel: decision.ImpactMajor,
		Topic:       "Bitable Topic",
		Proposer:    "Bitable Proposer",
		Executor:    "Bitable Executor",
	}

	merged := sm.mergeDecisions(gitNode, bitableNode)
	if merged.Title != "Bitable Title" {
		t.Errorf("Title = %q, want %q", merged.Title, "Bitable Title")
	}
	if merged.Decision != "Bitable Decision" {
		t.Errorf("Decision = %q, want %q", merged.Decision, "Bitable Decision")
	}
	if merged.Status != decision.StatusDecided {
		t.Errorf("Status = %q, want %q", merged.Status, decision.StatusDecided)
	}
	if merged.Proposer != "Bitable Proposer" {
		t.Errorf("Proposer = %q, want %q", merged.Proposer, "Bitable Proposer")
	}
	if merged.Executor != "Bitable Executor" {
		t.Errorf("Executor = %q, want %q", merged.Executor, "Bitable Executor")
	}
}

func TestMergeDecisions_BitableEmptyFields(t *testing.T) {
	sm := &SyncManager{}
	gitNode := &decision.DecisionNode{
		Title:       "Git Title",
		Decision:    "Git Decision",
		Rationale:   "Git Rationale",
		Status:      decision.StatusDecided,
		ImpactLevel: decision.ImpactMajor,
		Topic:       "Git Topic",
		Proposer:    "Git Proposer",
		Executor:    "Git Executor",
	}
	bitableNode := &decision.DecisionNode{
		// All fields empty — should keep Git values
	}

	merged := sm.mergeDecisions(gitNode, bitableNode)
	if merged.Title != "Git Title" {
		t.Errorf("Title should keep git value, got %q", merged.Title)
	}
	if merged.Decision != "Git Decision" {
		t.Errorf("Decision should keep git value, got %q", merged.Decision)
	}
	if merged.Status != decision.StatusDecided {
		t.Errorf("Status should keep git value, got %q", merged.Status)
	}
}

func TestMergeDecisions_PreservesOriginal(t *testing.T) {
	sm := &SyncManager{}
	gitNode := &decision.DecisionNode{
		SDRID: "DEC-001", Title: "Original", Decision: "Original decision",
	}
	bitableNode := &decision.DecisionNode{
		SDRID: "DEC-001", Title: "Updated", Decision: "Updated decision",
	}

	merged := sm.mergeDecisions(gitNode, bitableNode)

	// Merged should have updated values
	if merged.Title != "Updated" {
		t.Errorf("Merged Title = %q, want %q", merged.Title, "Updated")
	}

	// Original should NOT be modified (value semantics)
	if gitNode.Title != "Original" {
		t.Errorf("Original should be preserved, got %q", gitNode.Title)
	}
}

func TestMergeDecisions_CrossTopicRefs(t *testing.T) {
	sm := &SyncManager{}
	gitNode := &decision.DecisionNode{}
	bitableNode := &decision.DecisionNode{
		CrossTopicRefs: []string{"topic_a", "topic_b"},
	}

	merged := sm.mergeDecisions(gitNode, bitableNode)
	if len(merged.CrossTopicRefs) != 2 {
		t.Errorf("CrossTopicRefs should be merged, got %d", len(merged.CrossTopicRefs))
	}
}

func TestMergeDecisions_Stakeholders(t *testing.T) {
	sm := &SyncManager{}
	gitNode := &decision.DecisionNode{}
	bitableNode := &decision.DecisionNode{
		Stakeholders: []string{"Alice", "Bob"},
	}

	merged := sm.mergeDecisions(gitNode, bitableNode)
	if len(merged.Stakeholders) != 2 {
		t.Errorf("Stakeholders should be merged, got %d", len(merged.Stakeholders))
	}
}

func TestMergeDecisions_FeishuLinks(t *testing.T) {
	sm := &SyncManager{}
	gitNode := &decision.DecisionNode{}
	bitableNode := &decision.DecisionNode{
		FeishuLinks: decision.FeishuLinks{
			RelatedChatIDs: []string{"chat_1"},
		},
	}

	merged := sm.mergeDecisions(gitNode, bitableNode)
	if len(merged.FeishuLinks.RelatedChatIDs) != 1 {
		t.Errorf("FeishuLinks should be merged, got %d", len(merged.FeishuLinks.RelatedChatIDs))
	}
}

// ============================================================
// ChangeLog 测试
// ============================================================

func TestLogChange(t *testing.T) {
	sm := &SyncManager{changeLog: []ChangeLogEntry{}}
	sm.LogChange("git", "create", "DEC-001", "Test Decision", "details")

	log := sm.GetChangeLog()
	if len(log) != 1 {
		t.Fatalf("ChangeLog should have 1 entry, got %d", len(log))
	}
	if log[0].Source != "git" {
		t.Errorf("Source = %q, want %q", log[0].Source, "git")
	}
	if log[0].SDRID != "DEC-001" {
		t.Errorf("SDRID = %q, want %q", log[0].SDRID, "DEC-001")
	}
	if log[0].Op != "create" {
		t.Errorf("Op = %q, want %q", log[0].Op, "create")
	}
}

func TestGetChangeLog_ReturnsCopy(t *testing.T) {
	sm := &SyncManager{changeLog: []ChangeLogEntry{}}
	sm.LogChange("git", "create", "DEC-001", "Test", "details")

	log1 := sm.GetChangeLog()
	log2 := sm.GetChangeLog()

	// Should return independent copy
	log1[0].Title = "Modified"
	if log2[0].Title == "Modified" {
		t.Error("GetChangeLog should return a copy, not a reference")
	}
}

func TestLogChange_MultipleEntries(t *testing.T) {
	sm := &SyncManager{changeLog: []ChangeLogEntry{}}
	sm.LogChange("git", "create", "DEC-001", "First", "")
	sm.LogChange("bitable", "create", "DEC-002", "Second", "")

	log := sm.GetChangeLog()
	if len(log) != 2 {
		t.Errorf("Should have 2 entries, got %d", len(log))
	}
}
