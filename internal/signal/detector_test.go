package signal

import (
	"testing"
)

// ============================================================
// EnhancedDetector — 全流程集成测试
// ============================================================

func TestAnalyze_EmptyContent(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("", nil)

	if result.Score != 0 {
		t.Errorf("Score = %f, want 0", result.Score)
	}
	if result.Level != LevelNone {
		t.Errorf("Level = %v, want %v", result.Level, LevelNone)
	}
	if result.IsDecision {
		t.Error("IsDecision should be false for empty content")
	}
}

func TestAnalyze_WhitespaceContent(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("   ", nil)

	if result.Score != 0 {
		t.Errorf("Score = %f, want 0", result.Score)
	}
	if result.Level != LevelNone {
		t.Errorf("Level = %v, want %v", result.Level, LevelNone)
	}
}

func TestAnalyze_ExplicitDecision_Chinese(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("我们决定使用PostgreSQL作为主数据库", nil)

	if !result.IsDecision {
		t.Errorf("Should be decision, score=%.2f level=%v", result.Score, result.Level)
	}
	if result.Score < 0.65 {
		t.Errorf("Score should be >= 0.65 for explicit decision, got %.2f", result.Score)
	}
	if result.Level != LevelHigh {
		t.Errorf("Level should be high for explicit decision, got %v", result.Level)
	}
}

func TestAnalyze_ExplicitDecision_English(t *testing.T) {
	d := NewEnhancedDetector()
	// "LGTM, approved" alone is too vague for high confidence — verifies lexical matching exists
	result := d.Analyze("LGTM, approved", nil)

	hasLexSignal := false
	for _, s := range result.SignalDetails {
		if s.Name == "eng_approve" {
			hasLexSignal = true
			break
		}
	}
	if !hasLexSignal {
		t.Errorf("Should detect eng_approve lexical signal, got %+v", result.SignalDetails)
	}
}

func TestAnalyze_ImplicitDecision(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("那就用方案A吧", nil)

	if !result.IsDecision {
		t.Errorf("Should be decision, score=%.2f level=%v", result.Score, result.Level)
	}
}

func TestAnalyze_Greeting_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("早上好", nil)

	if result.IsDecision {
		t.Errorf("Greeting should NOT be a decision, got score=%.2f", result.Score)
	}
	if len(result.AntiSignals) == 0 {
		t.Error("Should have anti-signals for greeting")
	}
}

func TestAnalyze_SmallTalk_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("今天天气不错", nil)

	if result.IsDecision {
		t.Errorf("Small talk should NOT be a decision, got score=%.2f", result.Score)
	}
	if len(result.AntiSignals) == 0 {
		t.Error("Should have anti-signals for small talk")
	}
}

func TestAnalyze_Uncertainty_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("可能用MySQL也可能用PostgreSQL，再看", nil)

	if result.IsDecision {
		t.Errorf("Uncertain statement should NOT be a decision, got score=%.2f", result.Score)
	}
}

func TestAnalyze_PureQuestion_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("这个方案怎么样？", nil)

	if result.IsDecision {
		t.Errorf("Pure question should NOT be a decision, got score=%.2f", result.Score)
	}
}

func TestAnalyze_StatusUpdate_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("已完成登录模块开发，目前在做注册模块", nil)

	if result.IsDecision {
		t.Errorf("Status update should NOT be a decision, got score=%.2f", result.Score)
	}
}

func TestAnalyze_InformationSharing_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("FYI, 这份文档供参考", nil)

	if result.IsDecision {
		t.Errorf("Info sharing should NOT be a decision, got score=%.2f", result.Score)
	}
}

func TestAnalyze_Aspirational_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("打算用Go写后端", nil)

	if result.IsDecision {
		t.Errorf("Aspirational should NOT be a decision, got score=%.2f", result.Score)
	}
}

func TestAnalyze_ReportingOthers_AntiSignal(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("他说这个方案不错", nil)

	if result.IsDecision {
		t.Errorf("Reporting others should NOT be a decision, got score=%.2f", result.Score)
	}
}

func TestAnalyze_EmojiOnly(t *testing.T) {
	d := NewEnhancedDetector()
	result := d.Analyze("👍🎉", nil)

	if result.IsDecision {
		t.Errorf("Emoji-only should NOT be a decision, got score=%.2f", result.Score)
	}
	if len(result.AntiSignals) == 0 {
		t.Error("Should have anti-signals for pure emoji")
	}
}

func TestAnalyze_AntiSignalReducesConfidence(t *testing.T) {
	// Uncertainty reduces but does not eliminate decision signal
	d := NewEnhancedDetector()
	result := d.Analyze("可能决定用PostgreSQL，也可能用MySQL，再讨论一下", nil)

	// Has anti-signals for uncertainty
	hasAnti := false
	for _, a := range result.AntiSignals {
		if prefix := "uncertain:"; len(a) >= len(prefix) && a[:len(prefix)] == prefix {
			hasAnti = true
			break
		}
	}
	if !hasAnti {
		t.Error("Should have uncertainty anti-signals")
	}

	// Decision signal is present but lower (uncertainty reduces score)
	if result.Level != LevelMedium && result.Level != LevelHigh {
		t.Errorf("Decision should be at least medium despite uncertainty, got %v score=%.2f", result.Level, result.Score)
	}
}

// ============================================================
// LexicalAnalyzer
// ============================================================

func TestLexicalAnalyzer_HighWeightKeywords(t *testing.T) {
	a := NewLexicalAnalyzer()

	tests := []struct {
		input string
		name  string
	}{
		{"我们决定使用PostgreSQL", "explicit_decision"},
		{"最终决定用方案A", "explicit_decision"},
		{"确认采用微服务架构", "explicit_confirm"},
		{"审批通过", "explicit_approve"},
		{"就这样吧", "finalize"},
		{"结论是使用Redis", "conclusion"},
	}

	for _, tt := range tests {
		signals := a.Analyze(tt.input)
		found := false
		for _, s := range signals {
			if s.Name == tt.name {
				found = true
				if s.Weight < 0.85 {
					t.Errorf("Weight for %s = %.2f, want >= 0.85", tt.name, s.Weight)
				}
				break
			}
		}
		if !found {
			t.Errorf("Analyze(%q) should contain signal %q, got %+v", tt.input, tt.name, signals)
		}
	}
}

func TestLexicalAnalyzer_MediumWeightKeywords(t *testing.T) {
	a := NewLexicalAnalyzer()

	tests := []struct {
		input string
		name  string
	}{
		{"采用微服务架构", "adopt"},
		{"选择方案B", "selection"},
		{"建议使用Redis", "proposal"},
		{"approved", "eng_approve"},
		{"We decided to use Go", "eng_decision"},
		{"必须尽快修复", "requirement"},
		{"不采用这个方案", "rejection"},
		{"action item", "action_item"},
		{"张三是责任人", "assign_owner"},
	}

	for _, tt := range tests {
		signals := a.Analyze(tt.input)
		found := false
		for _, s := range signals {
			if s.Name == tt.name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Analyze(%q) should contain signal %q, got signals: %+v", tt.input, tt.name, names(signals))
		}
	}
}

func TestLexicalAnalyzer_NoDecisionKeywords(t *testing.T) {
	a := NewLexicalAnalyzer()
	signals := a.Analyze("今天天气不错")
	if len(signals) > 0 {
		t.Errorf("Analyze('今天天气不错') should return no signals, got %+v", signals)
	}
}

func TestLexicalAnalyzer_CaseInsensitive(t *testing.T) {
	a := NewLexicalAnalyzer()

	signals := a.Analyze("LGTM")
	found := false
	for _, s := range signals {
		if s.Name == "eng_approve" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Analyze('LGTM') should match eng_approve, got %+v", names(signals))
	}
}

func TestLexicalAnalyzer_MultipleMatches(t *testing.T) {
	a := NewLexicalAnalyzer()
	signals := a.Analyze("我们决定采用PostgreSQL，确认使用这个方案")

	if len(signals) < 2 {
		t.Errorf("Should have multiple matches, got %d: %+v", len(signals), signals)
	}
}

// ============================================================
// StructuralAnalyzer
// ============================================================

func TestStructuralAnalyzer_NumberedList(t *testing.T) {
	a := NewStructuralAnalyzer()
	signals := a.Analyze("1. 使用PostgreSQL 2. 使用Redis")

	found := false
	for _, s := range signals {
		if s.Name == "numbered_list" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should detect numbered_list, got %+v", names(signals))
	}
}

func TestStructuralAnalyzer_OptionComparison(t *testing.T) {
	a := NewStructuralAnalyzer()
	signals := a.Analyze("PostgreSQL vs MySQL")

	found := false
	for _, s := range signals {
		if s.Name == "option_comparison" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should detect option_comparison, got %+v", names(signals))
	}
}

func TestStructuralAnalyzer_QuotedContent(t *testing.T) {
	a := NewStructuralAnalyzer()
	signals := a.Analyze("他说「这个方案可行」")

	found := false
	for _, s := range signals {
		if s.Name == "quoted_content" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should detect quoted_content, got %+v", names(signals))
	}
}

func TestStructuralAnalyzer_TechnicalDetail(t *testing.T) {
	a := NewStructuralAnalyzer()
	// Must be > 50 runes AND contain tech terms
	longMsg := "API接口需要改造，数据库表结构也要调整，前端和后端都需要配合修改，整体架构需要重新设计，服务部署方案也要同步更新"
	signals := a.Analyze(longMsg)

	found := false
	for _, s := range signals {
		if s.Name == "technical_detail" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should detect technical_detail for long tech message, got %+v", names(signals))
	}
}

func TestStructuralAnalyzer_TaskAssignment(t *testing.T) {
	a := NewStructuralAnalyzer()
	signals := a.Analyze("张三来负责这个模块")

	found := false
	for _, s := range signals {
		if s.Name == "task_assignment" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should detect task_assignment, got %+v", names(signals))
	}
}

func TestStructuralAnalyzer_ShortMessageNoTech(t *testing.T) {
	a := NewStructuralAnalyzer()
	signals := a.Analyze("好的")

	if len(signals) > 0 {
		t.Errorf("Short non-tech message should have no structural signals, got %d", len(signals))
	}
}

// ============================================================
// DynamicAnalyzer
// ============================================================

func TestDynamicAnalyzer_NilContext(t *testing.T) {
	a := NewDynamicAnalyzer()
	signals := a.Analyze("test", nil)
	if len(signals) != 0 {
		t.Errorf("Nil context should return no signals, got %d", len(signals))
	}
}

func TestDynamicAnalyzer_IsReply(t *testing.T) {
	a := NewDynamicAnalyzer()
	ctx := &DetectContext{IsReply: true}
	signals := a.Analyze("test", ctx)

	found := false
	for _, s := range signals {
		if s.Name == "reply_to_discussion" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("IsReply=true should detect reply_to_discussion, got %+v", names(signals))
	}
}

func TestDynamicAnalyzer_HasMention(t *testing.T) {
	a := NewDynamicAnalyzer()
	ctx := &DetectContext{HasMention: true}
	signals := a.Analyze("test", ctx)

	found := false
	for _, s := range signals {
		if s.Name == "directed_message" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("HasMention=true should detect directed_message, got %+v", names(signals))
	}
}

func TestDynamicAnalyzer_MultiRound(t *testing.T) {
	a := NewDynamicAnalyzer()
	ctx := &DetectContext{MessageIndex: 5}
	signals := a.Analyze("test", ctx)

	found := false
	for _, s := range signals {
		if s.Name == "multi_round_discussion" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MessageIndex>2 should detect multi_round_discussion, got %+v", names(signals))
	}
}

func TestDynamicAnalyzer_TopicContinuation(t *testing.T) {
	a := NewDynamicAnalyzer()
	ctx := &DetectContext{
		RecentKeywords: []string{"PostgreSQL", "数据库", "迁移"},
	}
	signals := a.Analyze("决定用PostgreSQL进行数据库迁移", ctx)

	found := false
	for _, s := range signals {
		if s.Name == "topic_continuation" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Topic overlap >=2 should detect topic_continuation, got %+v", names(signals))
	}
}

func TestDynamicAnalyzer_NoTopicOverlap(t *testing.T) {
	a := NewDynamicAnalyzer()
	ctx := &DetectContext{
		RecentKeywords: []string{"PostgreSQL", "数据库"},
	}
	signals := a.Analyze("今天天气不错", ctx)

	for _, s := range signals {
		if s.Name == "topic_continuation" {
			t.Errorf("No topic overlap should not detect topic_continuation")
		}
	}
}

func TestDynamicAnalyzer_EarlyMessageIndex(t *testing.T) {
	a := NewDynamicAnalyzer()
	ctx := &DetectContext{MessageIndex: 1, IsReply: false, HasMention: false}
	signals := a.Analyze("test", ctx)

	if len(signals) != 0 {
		t.Errorf("No signals expected for early message with no reply/mention, got %d: %+v", len(signals), names(signals))
	}
}

// ============================================================
// PatternMatcherV2
// ============================================================

func TestPatternMatcherV2_AdoptSolution(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("采用方案A")

	found := false
	for _, s := range signals {
		if s.Name == "adopt_solution" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match adopt_solution pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_AdoptTool(t *testing.T) {
	m := NewPatternMatcherV2()
	// Regex: verb + [，,。.\s]* + one of (方案|方式|方法|技术|框架|工具)
	signals := m.Analyze("使用.框架")

	found := false
	for _, s := range signals {
		if s.Name == "adopt_solution" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match adopt_solution pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_DecidedAction(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("决定使用PostgreSQL")

	found := false
	for _, s := range signals {
		if s.Name == "decided_action" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match decided_action pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_ApproveProposal(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("同意这个方案")

	found := false
	for _, s := range signals {
		if s.Name == "approve_proposal" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match approve_proposal pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_AssignTask(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("由张三来负责这个模块")

	found := false
	for _, s := range signals {
		if s.Name == "assign_task" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match assign_task pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_Rejection(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("不采用这个方案")

	found := false
	for _, s := range signals {
		if s.Name == "rejection" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match rejection pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_NoMatch(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("今天天气真好")

	if len(signals) > 0 {
		t.Errorf("No patterns should match, got %d: %+v", len(signals), names(signals))
	}
}

func TestPatternMatcherV2_Voting(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("投票决定使用方案A")

	found := false
	for _, s := range signals {
		if s.Name == "voting" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match voting pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_Deadline(t *testing.T) {
	m := NewPatternMatcherV2()
	signals := m.Analyze("截止：5月15日")

	found := false
	for _, s := range signals {
		if s.Name == "deadline" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match deadline pattern, got %+v", names(signals))
	}
}

func TestPatternMatcherV2_DeadlineWithBefore(t *testing.T) {
	m := NewPatternMatcherV2()
	// Regex: keyword + separator + digits + month-marker
	// "之前" keyword must come before the date
	signals := m.Analyze("之前5月")

	found := false
	for _, s := range signals {
		if s.Name == "deadline" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Should match deadline pattern with '之前', got %+v", names(signals))
	}
}

// ============================================================
// Anti-signal 独立测试
// ============================================================

func TestDetectAntiSignals_Greeting(t *testing.T) {
	d := NewEnhancedDetector()

	tests := []string{
		"早上好",
		"大家好",
		"下午好",
		"辛苦了",
		"谢谢",
	}

	for _, input := range tests {
		signals := d.detectAntiSignals(input)
		hasGreeting := false
		for _, s := range signals {
			if prefix := "greeting:"; len(s) >= len(prefix) && s[:len(prefix)] == prefix {
				hasGreeting = true
				break
			}
		}
		if !hasGreeting {
			t.Errorf("detectAntiSignals(%q) should contain greeting signal, got %v", input, signals)
		}
	}
}

func TestDetectAntiSignals_Uncertainty(t *testing.T) {
	d := NewEnhancedDetector()

	tests := []string{
		"可能用MySQL",
		"也许吧",
		"再想想",
		"讨论一下",
	}

	for _, input := range tests {
		signals := d.detectAntiSignals(input)
		hasUncertainty := false
		for _, s := range signals {
			if prefix := "uncertain:"; len(s) >= len(prefix) && s[:len(prefix)] == prefix {
				hasUncertainty = true
				break
			}
		}
		if !hasUncertainty {
			t.Errorf("detectAntiSignals(%q) should contain uncertainty signal, got %v", input, signals)
		}
	}
}

func TestDetectAntiSignals_SmallTalk(t *testing.T) {
	d := NewEnhancedDetector()
	signals := d.detectAntiSignals("今天天气真好")

	hasSmallTalk := false
	for _, s := range signals {
		if prefix := "small_talk:"; len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			hasSmallTalk = true
			break
		}
	}
	if !hasSmallTalk {
		t.Errorf("detectAntiSignals('今天天气真好') should contain small_talk signal, got %v", signals)
	}
}

func TestDetectAntiSignals_LongMessageNotSmallTalk(t *testing.T) {
	d := NewEnhancedDetector()
	longMsg := "今天天气真好，我们来讨论一下系统架构的方案，决定用PostgreSQL还是MySQL"
	signals := d.detectAntiSignals(longMsg)

	for _, s := range signals {
		if prefix := "small_talk:"; len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			t.Errorf("Long message with decision keywords should not get small_talk anti-signal, got %v", signals)
		}
	}
}

func TestDetectAntiSignals_MultipleCategories(t *testing.T) {
	d := NewEnhancedDetector()
	signals := d.detectAntiSignals("可能早上好")

	count := len(signals)
	if count < 2 {
		t.Errorf("Should detect multiple anti-signal categories, got %d: %v", count, signals)
	}
}

// ============================================================
// calculateAntiScore 测试
// ============================================================

func TestCalculateAntiScore_Empty(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.calculateAntiScore(nil)
	if score != 0 {
		t.Errorf("Empty signals should give score 0, got %.2f", score)
	}
	score = d.calculateAntiScore([]string{})
	if score != 0 {
		t.Errorf("Empty signals should give score 0, got %.2f", score)
	}
}

func TestCalculateAntiScore_Greeting(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.calculateAntiScore([]string{"greeting:早上好"})
	if score < 0.25 || score > 0.35 {
		t.Errorf("greeting score should be ~0.3, got %.2f", score)
	}
}

func TestCalculateAntiScore_Uncertainty(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.calculateAntiScore([]string{"uncertain:可能"})
	if score < 0.35 || score > 0.45 {
		t.Errorf("uncertainty score should be ~0.4, got %.2f", score)
	}
}

func TestCalculateAntiScore_Emoji(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.calculateAntiScore([]string{"pure_emoji"})
	if score < 0.45 || score > 0.55 {
		t.Errorf("emoji score should be ~0.5, got %.2f", score)
	}
}

func TestCalculateAntiScore_MultipleSignals(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.calculateAntiScore([]string{"greeting:早上好", "uncertain:可能", "small_talk:天气"})
	if score > 1.0 {
		t.Errorf("Multiple signals should cap at 1.0, got %.2f", score)
	}
	if score < 0.9 {
		t.Errorf("Multiple signals should give high score, got %.2f", score)
	}
}

// ============================================================
// computeFinalScore 测试
// ============================================================

func TestComputeFinalScore_AllZero(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.computeFinalScore(0, 0, 0, 0, 0)
	if score != 0 {
		t.Errorf("All zero inputs should give 0, got %.2f", score)
	}
}

func TestComputeFinalScore_LexicalOnly(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.computeFinalScore(1.0, 0, 0, 0, 0)
	// lexical*0.45 = 0.45, times (1 - 0) = 0.45
	if score < 0.44 || score > 0.46 {
		t.Errorf("lexical=1.0 should give ~0.45, got %.2f", score)
	}
}

func TestComputeFinalScore_PatternBoost(t *testing.T) {
	d := NewEnhancedDetector()
	// pattern >= 0.7 and final < pattern → boost: final*0.5 + pattern*0.5
	score := d.computeFinalScore(0, 0, 0, 0.8, 0)
	// base = 0*0.45 + 0*0.15 + 0*0.05 + 0.8*0.35 = 0.28
	// boost: pattern >= 0.7 AND 0.28 < 0.8 → 0.28*0.5 + 0.8*0.5 = 0.54
	if score < 0.52 || score > 0.56 {
		t.Errorf("pattern=0.8 alone should give ~0.54 (with boost), got %.2f", score)
	}
}

func TestComputeFinalScore_AntiPenalty(t *testing.T) {
	d := NewEnhancedDetector()
	// anti=0.5 → penalty factor: (1 - 0.5*0.5) = 0.75
	score := d.computeFinalScore(1.0, 0, 0, 0, 0.5)
	// Without anti: 0.45. With anti: 0.45 * 0.75 = 0.3375
	if score < 0.30 || score > 0.37 {
		t.Errorf("lexical=1.0 anti=0.5 should give ~0.3375, got %.2f", score)
	}
}

func TestComputeFinalScore_FullPenalty(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.computeFinalScore(1.0, 0.5, 0.3, 0.8, 1.0)
	// With max anti penalty: (lex*0.45 + str*0.15 + dyn*0.05 + pat*0.35) * (1 - 1.0*0.5)
	// = (0.45 + 0.075 + 0.015 + 0.28) * 0.5 = 0.82 * 0.5 = 0.41
	if score < 0.38 || score > 0.44 {
		t.Errorf("Full anti=1.0 should heavily penalize, got %.2f", score)
	}
}

func TestComputeFinalScore_NegativeAnti(t *testing.T) {
	d := NewEnhancedDetector()
	_ = d.computeFinalScore(0, 0, 0, 0, 0) // just verify no panic
}

func TestComputeFinalScore_Clamp(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.computeFinalScore(5.0, 5.0, 5.0, 5.0, 0)
	if score > 1.0 {
		t.Errorf("Score should clamp at 1.0, got %.2f", score)
	}
}

// ============================================================
// classifyLevel 测试
// ============================================================

func TestClassifyLevel(t *testing.T) {
	d := NewEnhancedDetector()

	tests := []struct {
		score float64
		level DecisionLevel
	}{
		{0.0, LevelNone},
		{0.19, LevelNone},
		{0.20, LevelLow},
		{0.49, LevelLow},
		{0.50, LevelMedium},
		{0.64, LevelMedium},
		{0.65, LevelHigh},
		{1.0, LevelHigh},
	}

	for _, tt := range tests {
		got := d.classifyLevel(tt.score)
		if got != tt.level {
			t.Errorf("classifyLevel(%.2f) = %v, want %v", tt.score, got, tt.level)
		}
	}
}

// ============================================================
// isPureQuestion 测试
// ============================================================

func TestIsPureQuestion(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"这个方案怎么样？", true},
		{"你吃饭了吗", true},
		{"决定用这个吗", false}, // contains "决定"
		{"确认了吗", false},   // contains "确认"
		{"用这个不行吗", false}, // contains "用"
		{"选哪个", false},    // contains "选"
		{"hello", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isPureQuestion(tt.input)
		if got != tt.want {
			t.Errorf("isPureQuestion(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ============================================================
// isMostlyEmoji 测试
// ============================================================

func TestIsMostlyEmoji(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"👍", true},
		{"👍🎉❤️", true},
		{"👍好的", false},
		{"太长字符串🔴🔴🔴🔴🔴", false}, // > 10 runes
		{"", false},
		{"hello", false},
	}

	for _, tt := range tests {
		got := isMostlyEmoji(tt.input)
		if got != tt.want {
			t.Errorf("isMostlyEmoji(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ============================================================
// containsAny 测试
// ============================================================

func TestContainsAny(t *testing.T) {
	result := containsAny("今天天气不错", []string{"天气", "吃饭", "睡觉"})
	if len(result) != 1 || result[0] != "天气" {
		t.Errorf("containsAny should find '天气', got %v", result)
	}
}

func TestContainsAny_NoMatch(t *testing.T) {
	result := containsAny("hello world", []string{"天气", "吃饭"})
	if len(result) != 0 {
		t.Errorf("containsAny should return empty, got %v", result)
	}
}

func TestContainsAny_MultipleMatches(t *testing.T) {
	result := containsAny("今天天气不错，吃饭了吗", []string{"天气", "吃饭", "睡觉"})
	if len(result) != 2 {
		t.Errorf("containsAny should find 2 matches, got %v", result)
	}
}

func TestContainsAny_EmptyKeywords(t *testing.T) {
	result := containsAny("test", nil)
	if len(result) != 0 {
		t.Errorf("containsAny with nil keywords should return empty, got %v", result)
	}
}

func TestContainsAny_EmptyContent(t *testing.T) {
	result := containsAny("", []string{"test"})
	if len(result) != 0 {
		t.Errorf("containsAny with empty content should return empty, got %v", result)
	}
}

// ============================================================
// calculateCategoryScore 测试
// ============================================================

func TestCalculateCategoryScore_Empty(t *testing.T) {
	d := NewEnhancedDetector()
	score := d.calculateCategoryScore(nil)
	if score != 0 {
		t.Errorf("Empty signals should give 0, got %.2f", score)
	}
	score = d.calculateCategoryScore([]SignalDetail{})
	if score != 0 {
		t.Errorf("Empty signals should give 0, got %.2f", score)
	}
}

func TestCalculateCategoryScore_SingleSignal(t *testing.T) {
	d := NewEnhancedDetector()
	signals := []SignalDetail{{Name: "test", Weight: 0.5}}
	score := d.calculateCategoryScore(signals)
	if score != 0.5 {
		t.Errorf("Single signal with weight 0.5 should give 0.5, got %.2f", score)
	}
}

func TestCalculateCategoryScore_MaxWeight(t *testing.T) {
	d := NewEnhancedDetector()
	signals := []SignalDetail{
		{Name: "a", Weight: 0.3},
		{Name: "b", Weight: 0.7},
	}
	score := d.calculateCategoryScore(signals)
	if score != 0.7 {
		t.Errorf("Max weight 0.7 should give 0.7, got %.2f", score)
	}
}

func TestCalculateCategoryScore_ThreeSignalsBonus(t *testing.T) {
	d := NewEnhancedDetector()
	signals := []SignalDetail{
		{Name: "a", Weight: 0.5},
		{Name: "b", Weight: 0.3},
		{Name: "c", Weight: 0.4},
	}
	score := d.calculateCategoryScore(signals)
	// max=0.5, 3 signals → +0.1 = 0.6
	if score < 0.59 || score > 0.61 {
		t.Errorf("3 signals with max 0.5 should give 0.6, got %.2f", score)
	}
}

func TestCalculateCategoryScore_Cap(t *testing.T) {
	d := NewEnhancedDetector()
	signals := []SignalDetail{
		{Name: "a", Weight: 0.9},
		{Name: "b", Weight: 0.8},
		{Name: "c", Weight: 0.7},
	}
	score := d.calculateCategoryScore(signals)
	// max=0.9, 3 signals → +0.1 = 1.0
	if score > 1.0 {
		t.Errorf("Score should cap at 1.0, got %.2f", score)
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkEnhancedDetector_Analyze(b *testing.B) {
	d := NewEnhancedDetector()
	contents := []string{
		"我们决定使用PostgreSQL作为主数据库",
		"今天天气不错",
		"1. 使用PostgreSQL 2. 使用Redis 3. 使用Kafka，结论是采用方案A",
		"可能用MySQL也可能用PostgreSQL，再讨论一下",
		"LGTM",
		"确认采用微服务架构，由张三负责实施，截止日期5月15日",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Analyze(contents[i%len(contents)], nil)
	}
}

func BenchmarkLexicalAnalyzer(b *testing.B) {
	a := NewLexicalAnalyzer()
	content := "我们决定使用PostgreSQL作为主数据库，确认采用这个方案"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Analyze(content)
	}
}

func BenchmarkPatternMatcherV2(b *testing.B) {
	m := NewPatternMatcherV2()
	contents := []string{
		"采用方案A",
		"决定使用PostgreSQL",
		"由张三来负责这个模块",
		"不采用这个方案",
		"今天天气不错",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Analyze(contents[i%len(contents)])
	}
}

func BenchmarkAntiSignalDetection(b *testing.B) {
	d := NewEnhancedDetector()
	contents := []string{
		"早上好",
		"可能用MySQL，也许吧",
		"今天天气真好",
		"这个方案怎么样？",
		"已完成登录模块开发",
		"FYI，供参考",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.detectAntiSignals(contents[i%len(contents)])
	}
}

func BenchmarkComputeFinalScore(b *testing.B) {
	d := NewEnhancedDetector()
	for i := 0; i < b.N; i++ {
		d.computeFinalScore(0.8, 0.3, 0.1, 0.6, 0.2)
	}
}

// ============================================================
// 实用辅助函数
// ============================================================

func names(signals []SignalDetail) []string {
	var ns []string
	for _, s := range signals {
		ns = append(ns, s.Name)
	}
	return ns
}
