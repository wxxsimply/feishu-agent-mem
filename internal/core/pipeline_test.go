package core

import (
	"testing"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/signal"
)

// mockGitStorage implements GitStorageInterface for testing
type mockGitStorage struct {
	decisions map[string]*decision.DecisionNode
	writeHash string
	writeErr  error
}

func newMockGitStorage() *mockGitStorage {
	return &mockGitStorage{
		decisions: make(map[string]*decision.DecisionNode),
		writeHash: "abc123def456",
	}
}

func (m *mockGitStorage) WriteDecision(node *decision.DecisionNode) (string, error) {
	if m.writeErr != nil {
		return "", m.writeErr
	}
	m.decisions[node.SDRID] = node
	node.GitCommitHash = m.writeHash
	return m.writeHash, nil
}

func (m *mockGitStorage) ReadDecision(project, topic, sdrID string) (*decision.DecisionNode, error) {
	d, ok := m.decisions[sdrID]
	if !ok {
		return nil, nil
	}
	return d, nil
}

func (m *mockGitStorage) ListDecisions(project, topic string) ([]*decision.DecisionNode, error) {
	var result []*decision.DecisionNode
	for _, d := range m.decisions {
		result = append(result, d)
	}
	return result, nil
}

// mockBitableStore implements BitableStoreInterface for testing
type mockBitableStore struct {
	upsertCount int
}

func newMockBitableStore() *mockBitableStore {
	return &mockBitableStore{}
}

func (m *mockBitableStore) UpsertDecision(node *decision.DecisionNode) error {
	m.upsertCount++
	return nil
}

func (m *mockBitableStore) QueryByTopic(topic, status string) ([]*decision.DecisionNode, error) {
	return nil, nil
}

func (m *mockBitableStore) QueryCrossTopic(topic string) ([]*decision.DecisionNode, error) {
	return nil, nil
}

func TestNewPipelineEngine(t *testing.T) {
	git := newMockGitStorage()
	bitable := newMockBitableStore()
	mg := NewMemoryGraph()

	pe := NewPipelineEngine(git, bitable, mg)
	if pe == nil {
		t.Fatal("NewPipelineEngine should not return nil")
	}
	if pe.GitStorage != git {
		t.Error("GitStorage not set correctly")
	}
	if pe.BitableStore != bitable {
		t.Error("BitableStore not set correctly")
	}
	if pe.MemoryGraph != mg {
		t.Error("MemoryGraph not set correctly")
	}
}

func TestPipelineEngine_ValidateDecision_Valid(t *testing.T) {
	pe := NewPipelineEngine(nil, nil, NewMemoryGraph())
	node := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Test",
		Topic:       "数据库架构",
		Status:      decision.StatusPending,
		ImpactLevel: decision.ImpactMajor,
	}

	issues := pe.ValidateDecision(node)
	if len(issues) > 0 {
		t.Errorf("Valid node should have no issues, got %v", issues)
	}
}

func TestPipelineEngine_ValidateDecision_MissingFields(t *testing.T) {
	pe := NewPipelineEngine(nil, nil, NewMemoryGraph())
	node := &decision.DecisionNode{}

	issues := pe.ValidateDecision(node)
	if len(issues) == 0 {
		t.Fatal("Should have validation issues")
	}

	hasSDRID := false
	hasTitle := false
	hasTopic := false
	for _, issue := range issues {
		switch issue {
		case "SDRID is required":
			hasSDRID = true
		case "Title is required":
			hasTitle = true
		case "Topic is required":
			hasTopic = true
		}
	}
	if !hasSDRID {
		t.Error("Should report missing SDRID")
	}
	if !hasTitle {
		t.Error("Should report missing Title")
	}
	if !hasTopic {
		t.Error("Should report missing Topic")
	}
}

func TestPipelineEngine_ValidateDecision_InvalidStatus(t *testing.T) {
	pe := NewPipelineEngine(nil, nil, NewMemoryGraph())
	node := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Test",
		Topic:       "db",
		Status:      "invalid_status",
		ImpactLevel: decision.ImpactMajor,
	}

	issues := pe.ValidateDecision(node)
	found := false
	for _, issue := range issues {
		if issue == "Invalid status" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should flag invalid status, got %v", issues)
	}
}

func TestPipelineEngine_ValidateDecision_InvalidImpact(t *testing.T) {
	pe := NewPipelineEngine(nil, nil, NewMemoryGraph())
	node := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Test",
		Topic:       "db",
		Status:      decision.StatusPending,
		ImpactLevel: "invalid_impact",
	}

	issues := pe.ValidateDecision(node)
	found := false
	for _, issue := range issues {
		if issue == "Invalid impact level" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should flag invalid impact level, got %v", issues)
	}
}

func TestPipelineEngine_ApplyCreate(t *testing.T) {
	git := newMockGitStorage()
	bitable := newMockBitableStore()
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, bitable, mg)

	node := &decision.DecisionNode{
		SDRID:       "DEC-001",
		Title:       "Test Decision",
		Topic:       "数据库架构",
		Project:     "feishu-mem",
		Status:      decision.StatusPending,
		ImpactLevel: decision.ImpactMajor,
	}

	mut := &signal.DecisionMutation{
		Type: signal.MutationCreate,
		Node: node,
	}

	err := pe.ApplyMutation(mut)
	if err != nil {
		t.Fatalf("ApplyCreate failed: %v", err)
	}

	// Verify Git storage
	stored, ok := git.decisions["DEC-001"]
	if !ok {
		t.Fatal("Decision should be in Git storage")
	}
	if stored.Title != "Test Decision" {
		t.Errorf("Title = %q, want %q", stored.Title, "Test Decision")
	}
	if stored.GitCommitHash != "abc123def456" {
		t.Errorf("GitCommitHash should be set, got %q", stored.GitCommitHash)
	}

	// Verify MemoryGraph
	_, ok = mg.GetDecision("DEC-001")
	if !ok {
		t.Error("Decision should be in MemoryGraph")
	}

	// Verify Bitable was called
	if bitable.upsertCount != 1 {
		t.Errorf("Bitable upsert should have been called, count=%d", bitable.upsertCount)
	}
}

func TestPipelineEngine_ApplyCreate_NilNode(t *testing.T) {
	pe := NewPipelineEngine(newMockGitStorage(), nil, NewMemoryGraph())
	mut := &signal.DecisionMutation{
		Type: signal.MutationCreate,
		Node: nil,
	}

	err := pe.ApplyMutation(mut)
	if err == nil {
		t.Error("Should error when node is nil")
	}
}

func TestPipelineEngine_ApplyCreate_NoBitable(t *testing.T) {
	git := newMockGitStorage()
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, nil, mg)

	node := &decision.DecisionNode{
		SDRID: "DEC-001", Title: "Test", Topic: "db",
		Project: "p", Status: decision.StatusPending,
		ImpactLevel: decision.ImpactMajor,
	}

	mut := &signal.DecisionMutation{Type: signal.MutationCreate, Node: node}
	err := pe.ApplyMutation(mut)
	if err != nil {
		t.Fatalf("ApplyCreate without bitable failed: %v", err)
	}
}

func TestPipelineEngine_ApplyUpdate(t *testing.T) {
	git := newMockGitStorage()
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, nil, mg)

	// Pre-create decision
	pe.ApplyMutation(&signal.DecisionMutation{
		Type: signal.MutationCreate,
		Node: &decision.DecisionNode{
			SDRID: "DEC-001", Title: "Original", Topic: "db",
			Decision: "old decision", Project: "p",
			Status: decision.StatusPending, ImpactLevel: decision.ImpactMajor,
		},
	})

	// Update
	mut := &signal.DecisionMutation{
		Type:         signal.MutationUpdate,
		SDRID:        "DEC-001",
		FieldChanges: map[string]any{"title": "Updated Title", "decision": "new decision text"},
	}

	err := pe.ApplyMutation(mut)
	if err != nil {
		t.Fatalf("ApplyUpdate failed: %v", err)
	}

	// Verify
	updated, _ := mg.GetDecision("DEC-001")
	if updated.Title != "Updated Title" {
		t.Errorf("Title should be 'Updated Title', got %q", updated.Title)
	}
}

func TestPipelineEngine_ApplyStatusChange(t *testing.T) {
	git := newMockGitStorage()
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, nil, mg)

	// Pre-create
	pe.ApplyMutation(&signal.DecisionMutation{
		Type: signal.MutationCreate,
		Node: &decision.DecisionNode{
			SDRID: "DEC-001", Title: "Test", Topic: "db",
			Project: "p", Status: decision.StatusPending,
			ImpactLevel: decision.ImpactMajor,
		},
	})

	// Change status
	mut := &signal.DecisionMutation{
		Type:      signal.MutationStatusChange,
		SDRID:     "DEC-001",
		NewStatus: decision.StatusDecided,
	}

	err := pe.ApplyMutation(mut)
	if err != nil {
		t.Fatalf("ApplyStatusChange failed: %v", err)
	}

	updated, _ := mg.GetDecision("DEC-001")
	if updated.Status != decision.StatusDecided {
		t.Errorf("Status should be 'decided', got %q", updated.Status)
	}
}

func TestPipelineEngine_ApplyUnknownMutation(t *testing.T) {
	pe := NewPipelineEngine(nil, nil, NewMemoryGraph())
	mut := &signal.DecisionMutation{Type: "unknown"}
	err := pe.ApplyMutation(mut)
	if err == nil {
		t.Error("Should error on unknown mutation type")
	}
}

func TestPipelineEngine_BatchApply(t *testing.T) {
	git := newMockGitStorage()
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, nil, mg)

	mutations := []*signal.DecisionMutation{
		{
			Type: signal.MutationCreate,
			Node: &decision.DecisionNode{
				SDRID: "DEC-001", Title: "A", Topic: "t1",
				Project: "p", Status: decision.StatusPending,
				ImpactLevel: decision.ImpactMinor,
			},
		},
		{
			Type: signal.MutationCreate,
			Node: &decision.DecisionNode{
				SDRID: "DEC-002", Title: "B", Topic: "t2",
				Project: "p", Status: decision.StatusPending,
				ImpactLevel: decision.ImpactMinor,
			},
		},
	}

	err := pe.BatchApply(mutations)
	if err != nil {
		t.Fatalf("BatchApply failed: %v", err)
	}

	if mg.Count() != 2 {
		t.Errorf("Should have 2 decisions, got %d", mg.Count())
	}
}

func TestPipelineEngine_BatchApply_FailFast(t *testing.T) {
	git := newMockGitStorage()
	git.writeErr = nil // first write succeeds
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, nil, mg)

	mutations := []*signal.DecisionMutation{
		{
			Type: signal.MutationCreate,
			Node: &decision.DecisionNode{
				SDRID: "DEC-001", Title: "A", Topic: "t1",
				Project: "p", Status: decision.StatusPending,
				ImpactLevel: decision.ImpactMinor,
			},
		},
		{
			Type: "unknown",
		},
	}

	err := pe.BatchApply(mutations)
	if err == nil {
		t.Error("BatchApply should fail on unknown mutation type")
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkPipelineEngine_BatchApply(b *testing.B) {
	git := newMockGitStorage()
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, nil, mg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		muts := make([]*signal.DecisionMutation, 10)
		for j := 0; j < 10; j++ {
			sdrID := "DEC-" + string(rune('A'+j))
			muts[j] = &signal.DecisionMutation{
				Type: signal.MutationCreate,
				Node: &decision.DecisionNode{
					SDRID: sdrID, Title: "Benchmark Decision",
					Topic: "bench", Project: "p",
					Status:      decision.StatusPending,
					ImpactLevel: decision.ImpactMinor,
				},
			}
		}
		b.StartTimer()
		pe.BatchApply(muts)
	}
}

func BenchmarkPipelineEngine_ValidateDecision(b *testing.B) {
	pe := NewPipelineEngine(nil, nil, NewMemoryGraph())
	node := &decision.DecisionNode{
		SDRID: "DEC-001", Title: "Test", Topic: "db",
		Status: decision.StatusPending, ImpactLevel: decision.ImpactMajor,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pe.ValidateDecision(node)
	}
}

func BenchmarkPipelineEngine_ApplyCreate(b *testing.B) {
	git := newMockGitStorage()
	mg := NewMemoryGraph()
	pe := NewPipelineEngine(git, nil, mg)

	node := &decision.DecisionNode{
		SDRID: "DEC-001", Title: "Benchmark", Topic: "db",
		Project: "p", Status: decision.StatusPending,
		ImpactLevel: decision.ImpactMinor,
	}
	mut := &signal.DecisionMutation{Type: signal.MutationCreate, Node: node}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pe.ApplyMutation(mut)
	}
}
