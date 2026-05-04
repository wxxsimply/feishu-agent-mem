package llm

import (
	"testing"
)

// ============================================================
// ExtractJSON 测试
// ============================================================

func TestExtractJSON_WithJSONFence(t *testing.T) {
	input := "Here is the result:\n```json\n{\"key\": \"value\"}\n```\nEnd"
	result := ExtractJSON(input)
	if result != `{"key": "value"}` {
		t.Errorf("ExtractJSON = %q, want %q", result, `{"key": "value"}`)
	}
}

func TestExtractJSON_WithGenericFence(t *testing.T) {
	input := "```\n{\"key\": \"value\"}\n```"
	result := ExtractJSON(input)
	if result != `{"key": "value"}` {
		t.Errorf("ExtractJSON = %q, want %q", result, `{"key": "value"}`)
	}
}

func TestExtractJSON_BareJSON(t *testing.T) {
	input := `{"key": "value", "number": 42}`
	result := ExtractJSON(input)
	if result != input {
		t.Errorf("ExtractJSON = %q, want %q", result, input)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "This is plain text without JSON"
	result := ExtractJSON(input)
	if result != input {
		t.Errorf("ExtractJSON should return original text, got %q", result)
	}
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	input := `{"outer": {"inner": "value"}, "arr": [1, 2, {"deep": true}]}`
	result := ExtractJSON(input)
	if result != input {
		t.Errorf("ExtractJSON should handle nested braces, got %q", result)
	}
}

func TestExtractJSON_JSONFenceWithExtraText(t *testing.T) {
	input := "Some text\n```json\n{\"a\": 1, \"b\": [1, 2, 3]}\n```\nMore text"
	expected := `{"a": 1, "b": [1, 2, 3]}`
	result := ExtractJSON(input)
	if result != expected {
		t.Errorf("ExtractJSON = %q, want %q", result, expected)
	}
}

func TestExtractJSON_EmptyInput(t *testing.T) {
	result := ExtractJSON("")
	if result != "" {
		t.Errorf("Empty input should return empty, got %q", result)
	}
}

func TestExtractJSON_PartialBraces(t *testing.T) {
	input := "just { a partial json"
	result := ExtractJSON(input)
	// Should extract from first { to last }
	if result != input {
		t.Errorf("ExtractJSON should return original for partial match, got %q", result)
	}
}

func TestExtractJSON_MultipleJSONBlocks(t *testing.T) {
	input := "```json\n{\"first\": true}\n```\n...\n```json\n{\"second\": true}\n```"
	result := ExtractJSON(input)
	// Should extract only the first JSON block
	if result != `{"first": true}` {
		t.Errorf("ExtractJSON should extract first block, got %q", result)
	}
}

// ============================================================
// ParseExtractionResult 测试 (mock JSON inputs, no real LLM)
// ============================================================

func TestParseExtractionResult_ValidJSON(t *testing.T) {
	input := `{"has_decision": true, "confidence": 0.85, "decision": {"title": "Test", "decision": "content", "proposer": "me"}}`
	result, err := ParseExtractionResult(input)
	if err != nil {
		t.Fatalf("ParseExtractionResult failed: %v", err)
	}
	if !result.HasDecision {
		t.Error("HasDecision should be true")
	}
	if result.Confidence != 0.85 {
		t.Errorf("Confidence = %.2f, want 0.85", result.Confidence)
	}
	if result.Decision == nil {
		t.Fatal("Decision should not be nil")
	}
	if result.Decision.Title != "Test" {
		t.Errorf("Title = %q, want %q", result.Decision.Title, "Test")
	}
}

func TestParseExtractionResult_NoDecision(t *testing.T) {
	input := `{"has_decision": false, "confidence": 0.0}`
	result, err := ParseExtractionResult(input)
	if err != nil {
		t.Fatalf("ParseExtractionResult failed: %v", err)
	}
	if result.HasDecision {
		t.Error("HasDecision should be false")
	}
	if result.Confidence != 0.0 {
		t.Errorf("Confidence should be 0.0, got %.2f", result.Confidence)
	}
}

func TestParseExtractionResult_InFence(t *testing.T) {
	input := "Response:\n```json\n{\"has_decision\": true, \"confidence\": 0.9}\n```"
	result, err := ParseExtractionResult(input)
	if err != nil {
		t.Fatalf("ParseExtractionResult with fence failed: %v", err)
	}
	if !result.HasDecision {
		t.Error("HasDecision should be true")
	}
}

func TestParseExtractionResult_InvalidJSON(t *testing.T) {
	input := "not json at all"
	_, err := ParseExtractionResult(input)
	if err == nil {
		t.Error("ParseExtractionResult should fail on invalid JSON")
	}
}

func TestParseExtractionResult_TrailingComma(t *testing.T) {
	// Trailing comma should be fixed by ParseTool
	input := `{"has_decision": true, "confidence": 0.8,}`
	result, err := ParseExtractionResult(input)
	if err != nil {
		t.Fatalf("ParseExtractionResult with trailing comma should be handled: %v", err)
	}
	if !result.HasDecision {
		t.Error("HasDecision should be true")
	}
}

func TestParseExtractionResult_EmptyObject(t *testing.T) {
	input := `{}`
	result, err := ParseExtractionResult(input)
	if err != nil {
		t.Fatalf("ParseExtractionResult empty object: %v", err)
	}
	if result.HasDecision {
		t.Error("HasDecision should be false for empty")
	}
}

// ============================================================
// ParseClassificationResult 测试
// ============================================================

func TestParseClassificationResult_Valid(t *testing.T) {
	input := `{"topic": "数据库架构", "confidence": 0.85, "reasoning": "keyword match"}`
	result, err := ParseClassificationResult(input)
	if err != nil {
		t.Fatalf("ParseClassificationResult failed: %v", err)
	}
	if result.Topic != "数据库架构" {
		t.Errorf("Topic = %q, want %q", result.Topic, "数据库架构")
	}
	if result.Confidence != 0.85 {
		t.Errorf("Confidence = %.2f, want 0.85", result.Confidence)
	}
}

func TestParseClassificationResult_Invalid(t *testing.T) {
	_, err := ParseClassificationResult("invalid")
	if err == nil {
		t.Error("ParseClassificationResult should fail on invalid JSON")
	}
}

// ============================================================
// ParseCrossTopicResult 测试
// ============================================================

func TestParseCrossTopicResult_Valid(t *testing.T) {
	input := `{"is_cross_topic": true, "cross_topic_refs": ["topic_a", "topic_b"], "confidence": 0.8}`
	result, err := ParseCrossTopicResult(input)
	if err != nil {
		t.Fatalf("ParseCrossTopicResult failed: %v", err)
	}
	if !result.IsCrossTopic {
		t.Error("IsCrossTopic should be true")
	}
	if len(result.CrossTopicRefs) != 2 {
		t.Errorf("len(CrossTopicRefs) = %d, want 2", len(result.CrossTopicRefs))
	}
}

func TestParseCrossTopicResult_Invalid(t *testing.T) {
	_, err := ParseCrossTopicResult("not json")
	if err == nil {
		t.Error("ParseCrossTopicResult should fail on invalid JSON")
	}
}

// ============================================================
// ParseConflictResult 测试
// ============================================================

func TestParseConflictResult_Valid(t *testing.T) {
	input := `{"contradiction_score": 0.9, "contradiction_type": "direct", "action": "notify", "needs_user": true}`
	result, err := ParseConflictResult(input)
	if err != nil {
		t.Fatalf("ParseConflictResult failed: %v", err)
	}
	if result.ContradictionScore != 0.9 {
		t.Errorf("ContradictionScore = %.2f, want 0.9", result.ContradictionScore)
	}
	if !result.NeedsUser {
		t.Error("NeedsUser should be true")
	}
}

func TestParseConflictResult_Invalid(t *testing.T) {
	_, err := ParseConflictResult("bad")
	if err == nil {
		t.Error("ParseConflictResult should fail on invalid JSON")
	}
}

// ============================================================
// Client.IsAvailable 测试
// ============================================================

func TestClient_IsAvailable_NoKey(t *testing.T) {
	// Create client with empty config
	c := &Client{config: &Config{APIKey: ""}}
	if c.IsAvailable() {
		t.Error("IsAvailable should be false when APIKey is empty")
	}
}

func TestClient_IsAvailable_WithKey(t *testing.T) {
	c := &Client{config: &Config{APIKey: "test-key"}}
	if !c.IsAvailable() {
		t.Error("IsAvailable should be true when APIKey is set")
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkExtractJSON(b *testing.B) {
	inputs := []string{
		`Here is the result:\n\` + "`" + `` + "`" + `` + "`" + `json\n{"has_decision": true, "confidence": 0.85}\n` + "`" + `` + "`" + `` + "`" + `\nEnd`,
		`{"has_decision": true, "confidence": 0.85}`,
		`Some text without JSON at all`,
		`{"outer": {"inner": [1, 2, {"deep": true}]}, "arr": []}`,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractJSON(inputs[i%len(inputs)])
	}
}

func BenchmarkParseExtractionResult(b *testing.B) {
	inputs := []string{
		`{"has_decision": true, "confidence": 0.85, "decision": {"title": "Test", "decision": "content", "proposer": "me"}}`,
		`{"has_decision": false, "confidence": 0.0}`,
		`{"has_decision": true, "confidence": 0.9, "decision": {"title": "A", "decision": "B", "rationale": "C", "impact_level": "major"}}`,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseExtractionResult(inputs[i%len(inputs)])
	}
}

func BenchmarkParseClassificationResult(b *testing.B) {
	input := `{"topic": "数据库架构", "confidence": 0.85, "reasoning": "keyword match"}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseClassificationResult(input)
	}
}

func BenchmarkParseConflictResult(b *testing.B) {
	input := `{"contradiction_score": 0.9, "contradiction_type": "direct", "action": "notify", "needs_user": true}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseConflictResult(input)
	}
}
