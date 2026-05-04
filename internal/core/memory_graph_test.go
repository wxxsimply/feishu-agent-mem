package core

import (
	"testing"

	"feishu-mem/internal/decision"
)

func TestNewMemoryGraph(t *testing.T) {
	mg := NewMemoryGraph()
	if mg == nil {
		t.Fatal("NewMemoryGraph should not return nil")
	}
	if mg.Count() != 0 {
		t.Errorf("New graph should have 0 decisions, got %d", mg.Count())
	}
}

func TestMemoryGraph_UpsertAndGet(t *testing.T) {
	mg := NewMemoryGraph()
	node := &decision.DecisionNode{
		SDRID:  "DEC-001",
		Title:  "Test Decision",
		Topic:  "数据库架构",
		Status: decision.StatusPending,
		Relations: []decision.Relation{
			{Type: decision.RelationRelatesTo, TargetSDRID: "DEC-002"},
		},
		CrossTopicRefs: []string{"前端架构"},
		Project:        "feishu-mem",
	}

	mg.UpsertDecision(node, "feishu-mem")

	got, ok := mg.GetDecision("DEC-001")
	if !ok {
		t.Fatal("GetDecision should find the decision")
	}
	if got.Title != "Test Decision" {
		t.Errorf("Title = %q, want %q", got.Title, "Test Decision")
	}
}

func TestMemoryGraph_GetDecision_NotFound(t *testing.T) {
	mg := NewMemoryGraph()
	_, ok := mg.GetDecision("NONEXISTENT")
	if ok {
		t.Error("GetDecision for non-existent should return false")
	}
}

func TestMemoryGraph_Count(t *testing.T) {
	mg := NewMemoryGraph()
	if mg.Count() != 0 {
		t.Errorf("Empty graph count should be 0, got %d", mg.Count())
	}

	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "A", Topic: "t1", Status: decision.StatusPending, Project: "p"}, "p")
	if mg.Count() != 1 {
		t.Errorf("Count should be 1, got %d", mg.Count())
	}

	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Title: "B", Topic: "t2", Status: decision.StatusPending, Project: "p"}, "p")
	if mg.Count() != 2 {
		t.Errorf("Count should be 2, got %d", mg.Count())
	}
}

func TestMemoryGraph_QueryByTopic(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "DB Decision", Topic: "数据库架构", Status: decision.StatusDecided, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Title: "Frontend Decision", Topic: "前端架构", Status: decision.StatusDecided, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-003", Title: "Another DB", Topic: "数据库架构", Status: decision.StatusCompleted, Project: "p"}, "p")

	results := mg.QueryByTopic("p", "数据库架构")
	// DEC-003 is StatusCompleted, which is not active, so only DEC-001
	if len(results) != 1 {
		t.Errorf("QueryByTopic should return 1 active decision, got %d", len(results))
	}
	if len(results) > 0 && results[0].SDRID != "DEC-001" {
		t.Errorf("First result should be DEC-001, got %s", results[0].SDRID)
	}
}

func TestMemoryGraph_QueryByTopic_AllActive(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Topic: "数据库架构", Status: decision.StatusDecided, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Topic: "数据库架构", Status: decision.StatusExecuting, Project: "p"}, "p")

	results := mg.QueryByTopic("p", "数据库架构")
	if len(results) != 2 {
		t.Errorf("Should return 2 active decisions, got %d", len(results))
	}
}

func TestMemoryGraph_QueryByTopic_NoDecisions(t *testing.T) {
	mg := NewMemoryGraph()
	results := mg.QueryByTopic("p", "不存在")
	if len(results) != 0 {
		t.Errorf("Non-existent topic should return empty, got %d", len(results))
	}
}

func TestMemoryGraph_QueryCrossTopic(t *testing.T) {
	mg := NewMemoryGraph()
	dec1 := &decision.DecisionNode{SDRID: "DEC-001", Topic: "数据库架构", CrossTopicRefs: []string{"前端架构"}, Status: decision.StatusDecided, Project: "p"}
	dec2 := &decision.DecisionNode{SDRID: "DEC-002", Topic: "前端架构", Status: decision.StatusDecided, Project: "p"}
	mg.UpsertDecision(dec1, "p")
	mg.UpsertDecision(dec2, "p")

	results := mg.QueryCrossTopic("p", "数据库架构")
	// Should include both the original topic and the cross-topic ref
	foundDB := false
	foundFE := false
	for _, r := range results {
		if r.SDRID == "DEC-001" {
			foundDB = true
		}
		if r.SDRID == "DEC-002" {
			foundFE = true
		}
	}
	if !foundDB {
		t.Error("Should include DEC-001 from current topic")
	}
	if !foundFE {
		t.Error("Should include DEC-002 from cross-topic ref")
	}
}

func TestMemoryGraph_DeleteDecision(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "Test", Topic: "t", Status: decision.StatusPending, Project: "p"}, "p")

	if mg.Count() != 1 {
		t.Fatalf("Count should be 1 after insert, got %d", mg.Count())
	}

	mg.DeleteDecision("DEC-001")
	if mg.Count() != 0 {
		t.Errorf("Count should be 0 after delete, got %d", mg.Count())
	}

	_, ok := mg.GetDecision("DEC-001")
	if ok {
		t.Error("Decision should not exist after delete")
	}
}

func TestMemoryGraph_DeleteDecision_NonExistent(t *testing.T) {
	mg := NewMemoryGraph()
	mg.DeleteDecision("NONEXISTENT") // Should not panic
}

func TestMemoryGraph_SearchByKeywords(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "使用PostgreSQL", Decision: "决定使用PostgreSQL", Topic: "数据库架构", Status: decision.StatusDecided, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Title: "使用Redis", Decision: "决定使用Redis", Topic: "缓存", Status: decision.StatusDecided, Project: "p"}, "p")

	results := mg.SearchByKeywords("PostgreSQL", "")
	if len(results) != 1 {
		t.Errorf("Should find 1 decision matching PostgreSQL, got %d", len(results))
	}
	if len(results) > 0 && results[0].SDRID != "DEC-001" {
		t.Errorf("Should find DEC-001, got %s", results[0].SDRID)
	}
}

func TestMemoryGraph_SearchByKeywords_WithTopicFilter(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "使用PostgreSQL", Decision: "决定使用PostgreSQL", Topic: "数据库架构", Status: decision.StatusDecided, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Title: "使用Redis", Decision: "决定使用Redis", Topic: "缓存", Status: decision.StatusDecided, Project: "p"}, "p")

	results := mg.SearchByKeywords("使用", "缓存")
	if len(results) != 1 {
		t.Errorf("Should find 1 decision in 缓存 topic, got %d", len(results))
	}
	if len(results) > 0 && results[0].SDRID != "DEC-002" {
		t.Errorf("Should find DEC-002, got %s", results[0].SDRID)
	}
}

func TestMemoryGraph_SearchByKeywords_NoMatch(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "使用PostgreSQL", Topic: "数据库架构", Status: decision.StatusDecided, Project: "p"}, "p")

	results := mg.SearchByKeywords("MongoDB", "")
	if len(results) != 0 {
		t.Errorf("No match should return empty, got %d", len(results))
	}
}

func TestMemoryGraph_GetAllDecisions(t *testing.T) {
	mg := NewMemoryGraph()
	all := mg.GetAllDecisions()
	if len(all) != 0 {
		t.Errorf("Empty graph should return 0, got %d", len(all))
	}

	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "A", Topic: "t1", Status: decision.StatusPending, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Title: "B", Topic: "t2", Status: decision.StatusPending, Project: "p"}, "p")

	all = mg.GetAllDecisions()
	if len(all) != 2 {
		t.Errorf("Should return 2 decisions, got %d", len(all))
	}
}

func TestMemoryGraph_DetectConflicts_NoConflicts(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{
		SDRID: "DEC-001", Title: "Use PostgreSQL", Topic: "db",
		Status: decision.StatusDecided, Project: "p",
	}, "p")

	newNode := &decision.DecisionNode{SDRID: "DEC-002", Title: "Use MySQL", Status: decision.StatusPending}
	conflicts := mg.DetectConflicts(newNode)
	if len(conflicts) != 0 {
		t.Errorf("No conflicts should exist, got %d", len(conflicts))
	}
}

func TestMemoryGraph_DetectConflicts_WithConflicts(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{
		SDRID: "DEC-001", Title: "Use PostgreSQL", Topic: "db",
		Status: decision.StatusDecided, Project: "p",
		Relations: []decision.Relation{
			{Type: decision.RelationConflictsWith, TargetSDRID: "DEC-002"},
		},
	}, "p")

	newNode := &decision.DecisionNode{SDRID: "DEC-002", Title: "Use MySQL", Status: decision.StatusPending}
	conflicts := mg.DetectConflicts(newNode)
	if len(conflicts) == 0 {
		t.Fatal("Should detect conflict")
	}
	if conflicts[0].DecisionA != "DEC-001" {
		t.Errorf("DecisionA = %q, want %q", conflicts[0].DecisionA, "DEC-001")
	}
	if conflicts[0].DecisionB != "DEC-002" {
		t.Errorf("DecisionB = %q, want %q", conflicts[0].DecisionB, "DEC-002")
	}
}

func TestMemoryGraph_ListAllTopics(t *testing.T) {
	mg := NewMemoryGraph()
	topics := mg.ListAllTopics("p")
	if len(topics) != 0 {
		t.Errorf("Empty project should return 0 topics, got %d", len(topics))
	}

	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Topic: "数据库架构", Status: decision.StatusPending, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Topic: "前端架构", Status: decision.StatusPending, Project: "p"}, "p")

	topics = mg.ListAllTopics("p")
	if len(topics) != 2 {
		t.Errorf("Should return 2 topics, got %d: %v", len(topics), topics)
	}
}

func TestMemoryGraph_TopicCount(t *testing.T) {
	mg := NewMemoryGraph()
	if mg.TopicCount("p") != 0 {
		t.Errorf("Empty project should have 0 topics, got %d", mg.TopicCount("p"))
	}
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Topic: "数据库架构", Status: decision.StatusPending, Project: "p"}, "p")
	if mg.TopicCount("p") != 1 {
		t.Errorf("TopicCount should be 1, got %d", mg.TopicCount("p"))
	}
}

func TestMemoryGraph_GetRelations(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{
		SDRID: "DEC-001", Topic: "db", Status: decision.StatusPending, Project: "p",
		Relations: []decision.Relation{
			{Type: decision.RelationRelatesTo, TargetSDRID: "DEC-002", Description: "related"},
		},
	}, "p")

	rels := mg.GetRelations("DEC-001")
	if len(rels) != 1 {
		t.Errorf("Should have 1 relation, got %d", len(rels))
	}
	if len(rels) > 0 && rels[0].Type != decision.RelationRelatesTo {
		t.Errorf("Type = %q, want %q", rels[0].Type, decision.RelationRelatesTo)
	}
}

func TestMemoryGraph_GetRelations_Nonexistent(t *testing.T) {
	mg := NewMemoryGraph()
	rels := mg.GetRelations("NONEXISTENT")
	if len(rels) != 0 {
		t.Errorf("Non-existent should return empty, got %d", len(rels))
	}
}

func TestMemoryGraph_GetRelatedDecisions(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Topic: "db", Status: decision.StatusPending, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-002", Topic: "cache", Status: decision.StatusPending, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{
		SDRID: "DEC-003", Topic: "arch", Status: decision.StatusPending, Project: "p",
		Relations: []decision.Relation{
			{Type: decision.RelationRelatesTo, TargetSDRID: "DEC-001"},
			{Type: decision.RelationRelatesTo, TargetSDRID: "DEC-002"},
		},
	}, "p")

	related := mg.GetRelatedDecisions("DEC-003")
	if len(related) != 2 {
		t.Errorf("Should have 2 related decisions, got %d", len(related))
	}
}

func TestMemoryGraph_Upsert_Overwrite(t *testing.T) {
	mg := NewMemoryGraph()
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "Original", Topic: "db", Status: decision.StatusPending, Project: "p"}, "p")
	mg.UpsertDecision(&decision.DecisionNode{SDRID: "DEC-001", Title: "Updated", Topic: "db", Status: decision.StatusDecided, Project: "p"}, "p")

	got, _ := mg.GetDecision("DEC-001")
	if got.Title != "Updated" {
		t.Errorf("Title should be 'Updated', got %q", got.Title)
	}
	if got.Status != decision.StatusDecided {
		t.Errorf("Status should be 'decided', got %q", got.Status)
	}
	if mg.Count() != 1 {
		t.Errorf("Count should still be 1 after upsert, got %d", mg.Count())
	}
}

func TestMemoryGraph_ConcurrencySafe(t *testing.T) {
	mg := NewMemoryGraph()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(i int) {
			sdrID := string(rune('A' + i))
			mg.UpsertDecision(&decision.DecisionNode{
				SDRID: sdrID, Title: "Test", Topic: "t",
				Status: decision.StatusPending, Project: "p",
			}, "p")
			mg.GetDecision(sdrID)
			mg.Count()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if mg.Count() != 10 {
		t.Errorf("Should have 10 decisions after concurrent upsert, got %d", mg.Count())
	}
}

func TestAppendUnique(t *testing.T) {
	slice := appendUnique(nil, "a")
	if len(slice) != 1 || slice[0] != "a" {
		t.Errorf("appendUnique nil = %v, want [a]", slice)
	}

	slice = appendUnique(slice, "b")
	if len(slice) != 2 {
		t.Errorf("appendUnique should append, got %d", len(slice))
	}

	slice = appendUnique(slice, "a")
	if len(slice) != 2 {
		t.Errorf("appendUnique should not duplicate, got %d: %v", len(slice), slice)
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkMemoryGraph_UpsertAndQuery(b *testing.B) {
	mg := NewMemoryGraph()
	projects := []string{"p1", "p2"}
	topics := []string{"数据库架构", "缓存方案", "前端架构", "后端架构", "基础设施"}

	nodes := make([]*decision.DecisionNode, 100)
	for i := 0; i < 100; i++ {
		nodes[i] = &decision.DecisionNode{
			SDRID:       string(rune('A' + i)),
			Title:       "Decision " + string(rune('0'+i%10)),
			Topic:       topics[i%len(topics)],
			Status:      decision.StatusDecided,
			ImpactLevel: decision.ImpactMajor,
			Project:     projects[i%len(projects)],
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % 100
		mg.UpsertDecision(nodes[idx], nodes[idx].Project)
		mg.QueryByTopic(nodes[idx].Project, nodes[idx].Topic)
	}
}

func BenchmarkMemoryGraph_Search(b *testing.B) {
	mg := NewMemoryGraph()
	for i := 0; i < 1000; i++ {
		sdrID := "DEC-" + string(rune('A'+i%26)) + string(rune('0'+i%10))
		mg.UpsertDecision(&decision.DecisionNode{
			SDRID: sdrID, Title: "使用PostgreSQL " + string(rune('0'+i%10)),
			Decision: "决定使用PostgreSQL", Topic: "数据库架构",
			Status: decision.StatusDecided, Project: "p",
		}, "p")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mg.SearchByKeywords("PostgreSQL", "")
	}
}

func BenchmarkMemoryGraph_ConcurrentRead(b *testing.B) {
	mg := NewMemoryGraph()
	for i := 0; i < 100; i++ {
		mg.UpsertDecision(&decision.DecisionNode{
			SDRID: "DEC-" + string(rune('A'+i)), Title: "Test",
			Topic: "db", Status: decision.StatusDecided, Project: "p",
		}, "p")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mg.GetDecision("DEC-A")
			mg.Count()
			mg.QueryByTopic("p", "db")
		}
	})
}

func BenchmarkMemoryGraph_DetectConflicts(b *testing.B) {
	mg := NewMemoryGraph()
	for i := 0; i < 100; i++ {
		mg.UpsertDecision(&decision.DecisionNode{
			SDRID: "DEC-" + string(rune('A'+i)), Title: "Test",
			Topic: "db", Status: decision.StatusDecided, Project: "p",
			Relations: []decision.Relation{
				{Type: decision.RelationConflictsWith, TargetSDRID: "DEC-NEW"},
			},
		}, "p")
	}

	newNode := &decision.DecisionNode{SDRID: "DEC-NEW", Title: "New"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mg.DetectConflicts(newNode)
	}
}
