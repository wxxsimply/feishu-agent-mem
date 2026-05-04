package llm

import (
	"testing"
)

func TestNewFallback(t *testing.T) {
	f := NewFallback()
	if f == nil {
		t.Fatal("NewFallback should not return nil")
	}
	if len(f.decisionKeywords) == 0 {
		t.Error("decisionKeywords should not be empty")
	}
}

func TestFallback_ExtractDecision_WithKeywords(t *testing.T) {
	f := NewFallback()

	tests := []struct {
		input string
	}{
		{"我们决定使用PostgreSQL"},
		{"This is decided"},
		{"确认采用这个方案"},
		{"LGTM"},
		{"通过审批"},
		{"定下来了"},
		{"结论是用Redis"},
		{"最终方案是A"},
	}

	for _, tt := range tests {
		result := f.ExtractDecision(tt.input)
		if !result.HasDecision {
			t.Errorf("ExtractDecision(%q) should have HasDecision=true", tt.input)
		}
		if result.Confidence != 0.7 {
			t.Errorf("Confidence should be 0.7 for fallback, got %.2f", result.Confidence)
		}
		if result.Decision == nil {
			t.Errorf("Decision should not be nil when HasDecision=true")
		} else if result.Decision.Proposer != "keyword_match" {
			t.Errorf("Proposer should be 'keyword_match', got %q", result.Decision.Proposer)
		}
	}
}

func TestFallback_ExtractDecision_WithoutKeywords(t *testing.T) {
	f := NewFallback()
	result := f.ExtractDecision("今天天气不错")

	if result.HasDecision {
		t.Error("Non-decision text should have HasDecision=false")
	}
	if result.Confidence != 0.0 {
		t.Errorf("Confidence should be 0.0, got %.2f", result.Confidence)
	}
	if result.Decision != nil {
		t.Error("Decision should be nil when HasDecision=false")
	}
}

func TestFallback_ExtractDecision_EmptyInput(t *testing.T) {
	f := NewFallback()
	result := f.ExtractDecision("")
	if result.HasDecision {
		t.Error("Empty input should have HasDecision=false")
	}
}

func TestFallback_ExtractDecision_FirstMatchWins(t *testing.T) {
	f := NewFallback()
	result := f.ExtractDecision("决定确认LGTM通过")
	// Should match "决定" first (first keyword in the list)
	if !result.HasDecision {
		t.Error("Should detect decision from keywords")
	}
}

func TestFallback_ClassifyTopic_Match(t *testing.T) {
	f := NewFallback()
	result := f.ClassifyTopic("使用PostgreSQL作为主数据库", []string{"数据库架构", "前端架构"})
	if result.Topic != "数据库架构" {
		t.Errorf("Topic = %q, want %q", result.Topic, "数据库架构")
	}
	if result.Confidence != 0.6 {
		t.Errorf("Confidence should be 0.6, got %.2f", result.Confidence)
	}
}

func TestFallback_ClassifyTopic_DatabaseKeywords(t *testing.T) {
	f := NewFallback()

	tests := []struct {
		input    string
		topic    string
		keywords []string
	}{
		{"使用MySQL作为数据库", "数据库架构", []string{"数据库架构", "前端"}},
		{"数据库表结构设计", "数据库架构", []string{"数据库架构", "后端"}},
		{"PostgreSQL vs MySQL", "数据库架构", []string{"数据库架构"}},
	}

	for _, tt := range tests {
		result := f.ClassifyTopic(tt.input, tt.keywords)
		if result.Topic != tt.topic {
			t.Errorf("ClassifyTopic(%q) = %q, want %q", tt.input, result.Topic, tt.topic)
		}
	}
}

func TestFallback_ClassifyTopic_NoMatch(t *testing.T) {
	f := NewFallback()
	result := f.ClassifyTopic("今天天气不错", []string{"数据库架构", "前端架构"})
	if result.Topic != "" {
		t.Errorf("Topic should be empty, got %q", result.Topic)
	}
	if result.Confidence != 0.0 {
		t.Errorf("Confidence should be 0.0, got %.2f", result.Confidence)
	}
}

func TestFallback_ClassifyTopic_EmptyTopics(t *testing.T) {
	f := NewFallback()
	result := f.ClassifyTopic("使用PostgreSQL", []string{})
	if result.Topic != "" {
		t.Errorf("Topic should be empty for empty topics list, got %q", result.Topic)
	}
}

func TestFallback_ClassifyTopic_ExactTopicMatch(t *testing.T) {
	f := NewFallback()
	result := f.ClassifyTopic("前端架构方案评审", []string{"数据库架构", "前端架构", "后端架构"})
	if result.Topic != "前端架构" {
		t.Errorf("Should match exact topic '前端架构', got %q", result.Topic)
	}
}

func TestFallback_DetectCrossTopic(t *testing.T) {
	f := NewFallback()
	result := f.DetectCrossTopic(nil)
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.IsCrossTopic {
		t.Error("Default should be IsCrossTopic=false")
	}
	if result.Confidence != 0.5 {
		t.Errorf("Default confidence should be 0.5, got %.2f", result.Confidence)
	}
}

func TestFallback_DetectCrossTopic_WithNode(t *testing.T) {
	f := NewFallback()
	result := f.DetectCrossTopic("some node")
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.IsCrossTopic {
		t.Error("Default should be IsCrossTopic=false")
	}
}
