package signal

import (
	larkadapter "feishu-mem/internal/lark-adapter"
	"strings"
)

var decisionKeywords = []string{"决定", "确认", "结论", "通过", "定下来", "approve", "decided", "confirmed", "决策", "决议", "评审", "review"}

func containsDecisionKeyword(text string) bool {
	lowerText := strings.ToLower(text)
	for _, kw := range decisionKeywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// StateChangeEmitter 将 Detector 的检测结果转为标准信号
type StateChangeEmitter interface {
	AdapterType() AdapterType
	EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error)
}

// NewEmitters 创建所有发射器
func NewEmitters() map[AdapterType]StateChangeEmitter {
	return map[AdapterType]StateChangeEmitter{
		AdapterIM:       &IMEmitter{},
		AdapterVC:       &VCEmitter{},
		AdapterDocs:     &DocsEmitter{},
		AdapterCalendar: &CalendarEmitter{},
		AdapterTask:     &TaskEmitter{},
		AdapterOKR:      &OKREmitter{},
		AdapterContact:  &ContactEmitter{},
		AdapterWiki:     &WikiEmitter{},
	}
}

// IMEmitter IM 发射器
type IMEmitter struct {
	keywords []string
}

func (e *IMEmitter) AdapterType() AdapterType { return AdapterIM }

func (e *IMEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}

	signal := NewSignal(AdapterIM, result.Source+" has changes")
	strength := StrengthWeak
	keywords := []string{}
	decisionSignals := []string{}

	for _, ch := range result.Changes {
		if ch.EntityType == "pin_message" {
			strength = StrengthStrong
			decisionSignals = append(decisionSignals, "pin")
		}
		if ch.Type == "new_text" || ch.Type == "new_post" {
			matched := e.MatchKeywords(ch.Summary)
			if len(matched) >= 2 {
				strength = maxStrength(strength, StrengthMedium)
			} else if len(matched) == 1 {
				strength = maxStrength(strength, StrengthWeak)
			}
			keywords = append(keywords, matched...)
			decisionSignals = append(decisionSignals, e.matchDecisionWords(ch.Summary)...)
		}
	}

	if strength == StrengthWeak && len(decisionSignals) == 0 {
		return nil, nil
	}

	signal.Strength = strength
	signal.Context.Keywords = keywords
	signal.Context.DecisionSignals = decisionSignals
	if len(result.Changes) > 0 {
		signal.Context.ContentSnippet = result.Changes[0].Summary
	}

	return signal, nil
}

func (e *IMEmitter) MatchKeywords(text string) []string {
	decisionKeywords := []string{"决定", "确认", "结论", "通过", "定下来", "approve", "decided", "confirmed"}
	var matches []string
	lowerText := strings.ToLower(text)
	for _, kw := range decisionKeywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			matches = append(matches, kw)
		}
	}
	return matches
}

func (e *IMEmitter) matchDecisionWords(text string) []string {
	var signals []string
	decisionWords := []string{"决定", "decided", "确认", "LGTM", "lgtm", "approve", "通过", "定下来", "就这么办"}
	for _, w := range decisionWords {
		if strings.Contains(text, w) {
			signals = append(signals, "decision")
			break
		}
	}
	return signals
}

// VCEmitter VC 发射器
type VCEmitter struct{}

func (e *VCEmitter) AdapterType() AdapterType { return AdapterVC }

func (e *VCEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}
	signal := NewSignal(AdapterVC, "VC changes detected")
	strength := StrengthWeak

	for _, ch := range result.Changes {
		switch ch.Type {
		case "meeting_minutes_available":
			strength = maxStrength(strength, StrengthStrong)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "ai_summary")
		case "minutes_created", "minutes_updated", "minutes_ai_summary_ready":
			strength = maxStrength(strength, StrengthStrong)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "ai_summary")
		case "meeting_todos":
			strength = maxStrength(strength, StrengthStrong)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "action_items")
		case "meeting_ended":
			strength = maxStrength(strength, StrengthMedium)
		}
	}

	if strength == StrengthWeak {
		return nil, nil
	}

	signal.Strength = strength
	if len(result.Changes) > 0 {
		signal.Context.ContentSnippet = result.Changes[0].Summary
	}
	return signal, nil
}

// DocsEmitter Docs 发射器
type DocsEmitter struct{}

func (e *DocsEmitter) AdapterType() AdapterType { return AdapterDocs }

func (e *DocsEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}
	signal := NewSignal(AdapterDocs, "Doc changes detected")
	strength := StrengthWeak

	for _, ch := range result.Changes {
		switch ch.Type {
		case "doc_decision":
			strength = maxStrength(strength, StrengthStrong)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "decision_doc")
		case "doc_comment_approval":
			strength = maxStrength(strength, StrengthStrong)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "approval_comment")
		case "doc_created", "doc_content_updated":
			// 检查标题是否含决策关键词
			if containsDecisionKeyword(ch.Summary) {
				strength = maxStrength(strength, StrengthStrong)
				signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "decision_doc")
			} else {
				strength = maxStrength(strength, StrengthWeak)
			}
		}
	}

	if strength == StrengthWeak {
		return nil, nil
	}

	signal.Strength = strength
	if len(result.Changes) > 0 {
		signal.Context.ContentSnippet = result.Changes[0].Summary
	}
	return signal, nil
}

// CalendarEmitter Calendar 发射器
type CalendarEmitter struct{}

func (e *CalendarEmitter) AdapterType() AdapterType { return AdapterCalendar }

func (e *CalendarEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}
	signal := NewSignal(AdapterCalendar, "Calendar changes detected")
	strength := StrengthWeak

	for _, ch := range result.Changes {
		if ch.Type == "decision_meeting" || containsDecisionKeyword(ch.Summary) {
			strength = maxStrength(strength, StrengthMedium)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "review_meeting")
		}
	}

	if strength == StrengthWeak {
		return nil, nil
	}

	signal.Strength = strength
	if len(result.Changes) > 0 {
		signal.Context.ContentSnippet = result.Changes[0].Summary
	}
	return signal, nil
}

// TaskEmitter Task 发射器
type TaskEmitter struct{}

func (e *TaskEmitter) AdapterType() AdapterType { return AdapterTask }

func (e *TaskEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}
	signal := NewSignal(AdapterTask, "Task changes detected")
	strength := StrengthWeak

	for _, ch := range result.Changes {
		switch ch.Type {
		case "task_completed":
			strength = maxStrength(strength, StrengthMedium)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "task_done")
		case "task_created", "task_updated":
			strength = maxStrength(strength, StrengthWeak)
		}
	}

	if strength == StrengthWeak {
		return nil, nil
	}

	signal.Strength = strength
	if len(result.Changes) > 0 {
		signal.Context.ContentSnippet = result.Changes[0].Summary
	}
	return signal, nil
}

// WikiEmitter Wiki 发射器
type WikiEmitter struct{}

func (e *WikiEmitter) AdapterType() AdapterType { return AdapterWiki }

func (e *WikiEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}
	signal := NewSignal(AdapterWiki, "Wiki changes detected")
	strength := StrengthWeak

	for _, ch := range result.Changes {
		if containsDecisionKeyword(ch.Summary) {
			strength = maxStrength(strength, StrengthMedium)
			signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "decision_node")
		}
	}

	if strength == StrengthWeak {
		return nil, nil
	}

	signal.Strength = strength
	if len(result.Changes) > 0 {
		signal.Context.ContentSnippet = result.Changes[0].Summary
	}
	return signal, nil
}

// OKREmitter OKR 发射器
type OKREmitter struct{}

func (e *OKREmitter) AdapterType() AdapterType { return AdapterOKR }

func (e *OKREmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}
	signal := NewSignal(AdapterOKR, "OKR changes detected")
	signal.Strength = StrengthMedium
	return signal, nil
}

// ContactEmitter Contact 发射器
type ContactEmitter struct{}

func (e *ContactEmitter) AdapterType() AdapterType { return AdapterContact }

func (e *ContactEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
	if !result.HasChanges {
		return nil, nil
	}
	signal := NewSignal(AdapterContact, "Contact changes detected")
	signal.Strength = StrengthWeak
	return signal, nil
}

func maxStrength(a, b SignalStrength) SignalStrength {
	order := map[SignalStrength]int{
		StrengthWeak:   1,
		StrengthMedium: 2,
		StrengthStrong: 3,
	}
	if order[a] > order[b] {
		return a
	}
	return b
}
