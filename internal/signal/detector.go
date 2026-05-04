package signal

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ============================================================
// 增强型决策检测器 — 多因子加权评分系统
//
// 设计原则：
// 1. 不再依赖单个关键词的"命中/未命中"二元判断
// 2. 从多个维度综合评分，超过阈值才判定为决策信号
// 3. 内置反信号机制，过滤闲聊/问候/不确定讨论/进度同步/信息分享等噪声
// ============================================================

// DecisionLevel 决策信号等级
type DecisionLevel string

const (
	LevelHigh   DecisionLevel = "high"   // 明确决策信号 → 直接 LLM 处理
	LevelMedium DecisionLevel = "medium" // 可能决策信号 → LLM 处理
	LevelLow    DecisionLevel = "low"    // 弱信号 → 暂不处理
	LevelNone   DecisionLevel = "none"   // 无决策信号 → 跳过
)

// DetectionResult 综合检测结果
type DetectionResult struct {
	Score         float64         `json:"score"`
	Level         DecisionLevel   `json:"level"`
	IsDecision    bool            `json:"is_decision"`
	SignalDetails []SignalDetail  `json:"signal_details"`
	AntiSignals   []string        `json:"anti_signals"`
	Factors       *ScoreBreakdown `json:"factors"`
}

// SignalDetail 单条信号明细
type SignalDetail struct {
	Category string  `json:"category"`
	Name     string  `json:"name"`
	Weight   float64 `json:"weight"`
	Matched  string  `json:"matched"`
}

// ScoreBreakdown 各维度分数分解
type ScoreBreakdown struct {
	Lexical    float64 `json:"lexical"`
	Structural float64 `json:"structural"`
	Dynamic    float64 `json:"dynamic"`
	Pattern    float64 `json:"pattern"`
	AntiScore  float64 `json:"anti_score"`
	Final      float64 `json:"final"`
}

// EnhancedDetector 增强型检测器
type EnhancedDetector struct {
	lexical    *LexicalAnalyzer
	structural *StructuralAnalyzer
	dynamic    *DynamicAnalyzer
	pattern    *PatternMatcherV2
}

// NewEnhancedDetector 创建增强型检测器
func NewEnhancedDetector() *EnhancedDetector {
	return &EnhancedDetector{
		lexical:    NewLexicalAnalyzer(),
		structural: NewStructuralAnalyzer(),
		dynamic:    NewDynamicAnalyzer(),
		pattern:    NewPatternMatcherV2(),
	}
}

// Analyze 对消息内容进行多维度决策检测
func (d *EnhancedDetector) Analyze(content string, ctx *DetectContext) *DetectionResult {
	if strings.TrimSpace(content) == "" {
		return &DetectionResult{
			Score:      0,
			Level:      LevelNone,
			IsDecision: false,
		}
	}

	result := &DetectionResult{
		Factors: &ScoreBreakdown{},
	}

	// 1. 反信号预检 — 如果反信号过强，直接跳过
	antiSignals := d.detectAntiSignals(content)
	result.AntiSignals = antiSignals
	antiScore := d.calculateAntiScore(antiSignals)
	result.Factors.AntiScore = antiScore
	if antiScore >= 0.6 {
		result.Level = LevelNone
		result.IsDecision = false
		return result
	}

	// 2. 词汇信号分析
	lexSignals := d.lexical.Analyze(content)
	lexScore := d.calculateCategoryScore(lexSignals)
	result.Factors.Lexical = lexScore
	result.SignalDetails = append(result.SignalDetails, lexSignals...)

	// 3. 结构信号分析
	structSignals := d.structural.Analyze(content)
	structScore := d.calculateCategoryScore(structSignals)
	result.Factors.Structural = structScore
	result.SignalDetails = append(result.SignalDetails, structSignals...)

	// 4. 对话动态分析
	dynSignals := d.dynamic.Analyze(content, ctx)
	dynScore := d.calculateCategoryScore(dynSignals)
	result.Factors.Dynamic = dynScore
	result.SignalDetails = append(result.SignalDetails, dynSignals...)

	// 5. 模式信号分析
	patSignals := d.pattern.Analyze(content)
	patScore := d.calculateCategoryScore(patSignals)
	result.Factors.Pattern = patScore
	result.SignalDetails = append(result.SignalDetails, patSignals...)

	// 6. 综合加权
	finalScore := d.computeFinalScore(lexScore, structScore, dynScore, patScore, antiScore)
	result.Factors.Final = finalScore
	result.Score = finalScore

	// 7. 等级判定
	result.Level = d.classifyLevel(finalScore)
	result.IsDecision = result.Level == LevelHigh || result.Level == LevelMedium

	return result
}

// detectAntiSignals 检测反信号（降低决策置信度的因素）
func (d *EnhancedDetector) detectAntiSignals(content string) []string {
	var signals []string

	if matched := containsAny(content, []string{
		"早上好", "下午好", "晚上好", "大家好", "hello", "hi ", "hey",
		"辛苦了", "谢谢", "感谢", "拜拜", "再见", "see you",
	}); len(matched) > 0 {
		signals = append(signals, "greeting:"+matched[0])
	}

	if isMostlyEmoji(content) {
		signals = append(signals, "pure_emoji")
	}

	if matched := containsAny(content, []string{
		"可能", "也许", "大概", "或许", "不一定",
		"maybe", "perhaps", "probably", "not sure",
		"再看看", "再想想", "考虑一下", "讨论一下",
		"再说", "以后再说",
	}); len(matched) > 0 {
		signals = append(signals, "uncertain:"+matched[0])
	}

	if isPureQuestion(content) {
		signals = append(signals, "pure_question")
	}

	if matched := containsAny(content, []string{
		"吃", "喝", "玩", "天气", "八卦",
	}); len(matched) > 0 && utf8.RuneCountInString(content) < 20 {
		signals = append(signals, "small_talk:"+matched[0])
	}

	// 状态更新/进度同步
	if matched := containsAny(content, []string{
		"已完成", "正在处理", "进度", "进展", "更新一下",
		"update", "完工", "当前状态", "目前",
	}); len(matched) > 0 {
		signals = append(signals, "status_update:"+matched[0])
	}

	// 信息分享
	if matched := containsAny(content, []string{
		"分享", "通知", "告知", "FYI", "fyi",
		"仅供参考", "参考",
	}); len(matched) > 0 {
		signals = append(signals, "information_sharing:"+matched[0])
	}

	// 计划性表述（未定）
	if matched := containsAny(content, []string{
		"打算", "想试试", "准备做", "计划做", "考虑使用",
		"考虑采用",
	}); len(matched) > 0 {
		signals = append(signals, "aspirational:"+matched[0])
	}

	// 转述他人
	if matched := containsAny(content, []string{
		"他说", "她说", "反馈说", "提到", "提及",
	}); len(matched) > 0 {
		signals = append(signals, "reporting_others:"+matched[0])
	}

	return signals
}

func (d *EnhancedDetector) calculateAntiScore(signals []string) float64 {
	if len(signals) == 0 {
		return 0
	}
	score := 0.0
	for _, s := range signals {
		switch {
		case strings.HasPrefix(s, "greeting:"):
			score += 0.3
		case s == "pure_emoji":
			score += 0.5
		case strings.HasPrefix(s, "uncertain:"):
			score += 0.4
		case s == "pure_question":
			score += 0.3
		case strings.HasPrefix(s, "small_talk:"):
			score += 0.5
		case strings.HasPrefix(s, "status_update:"):
			score += 0.5
		case strings.HasPrefix(s, "information_sharing:"):
			score += 0.4
		case strings.HasPrefix(s, "aspirational:"):
			score += 0.3
		case strings.HasPrefix(s, "reporting_others:"):
			score += 0.35
		default:
			score += 0.2
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

func (d *EnhancedDetector) calculateCategoryScore(signals []SignalDetail) float64 {
	if len(signals) == 0 {
		return 0
	}
	maxWeight := 0.0
	for _, s := range signals {
		if s.Weight > maxWeight {
			maxWeight = s.Weight
		}
	}
	if len(signals) >= 3 {
		maxWeight += 0.1
	}
	if maxWeight > 1.0 {
		maxWeight = 1.0
	}
	return maxWeight
}

// computeFinalScore 综合加权计算
func (d *EnhancedDetector) computeFinalScore(lexical, structural, dynamic, pattern, anti float64) float64 {
	final := lexical*0.45 + structural*0.15 + dynamic*0.05 + pattern*0.35

	if pattern >= 0.7 && final < pattern {
		final = final*0.5 + pattern*0.5
	}

	final *= (1.0 - anti*0.5)

	if final < 0 {
		final = 0
	}
	if final > 1.0 {
		final = 1.0
	}
	return final
}

func (d *EnhancedDetector) classifyLevel(score float64) DecisionLevel {
	switch {
	case score >= 0.65:
		return LevelHigh
	case score >= 0.50:
		return LevelMedium
	case score >= 0.20:
		return LevelLow
	default:
		return LevelNone
	}
}

// ============================================================
// DetectContext 检测上下文
// ============================================================

type DetectContext struct {
	IsReply        bool
	HasMention     bool
	RecentKeywords []string
	SenderName     string
	MessageIndex   int
}

// ============================================================
// 词汇分析器 — 多层级关键词系统
// ============================================================

type LexicalAnalyzer struct {
	highWeight   []weightedPattern
	mediumWeight []weightedPattern
	lowWeight    []weightedPattern
}

type weightedPattern struct {
	keywords []string
	weight   float64
	name     string
}

func NewLexicalAnalyzer() *LexicalAnalyzer {
	return &LexicalAnalyzer{
		highWeight: []weightedPattern{
			{keywords: []string{"决定", "已决定", "决定了", "最终决定"}, weight: 1.0, name: "explicit_decision"},
			{keywords: []string{"确认", "已确认", "确认了", "确认一下"}, weight: 0.9, name: "explicit_confirm"},
			{keywords: []string{"通过", "已通过", "通过了", "审批通过"}, weight: 0.9, name: "explicit_approve"},
			{keywords: []string{"定下来", "就这样", "就这么办", "定了"}, weight: 0.95, name: "finalize"},
			{keywords: []string{"结论", "最终结论", "结论是"}, weight: 0.9, name: "conclusion"},
			{keywords: []string{"不再讨论", "不讨论", "到此为止"}, weight: 0.85, name: "close_discussion"},
		},
		mediumWeight: []weightedPattern{
			{keywords: []string{"采用", "选用", "使用", "用这个"}, weight: 0.65, name: "adopt"},
			{keywords: []string{"选", "选择", "选型", "方案"}, weight: 0.55, name: "selection"},
			{keywords: []string{"建议", "推荐", "提议"}, weight: 0.50, name: "proposal"},
			{keywords: []string{"approve", "lgtm", "LGTM", "approved", "confirmed", "agreed"}, weight: 0.70, name: "eng_approve"},
			{keywords: []string{"decided", "decision", "finalize"}, weight: 0.75, name: "eng_decision"},
			{keywords: []string{"需要", "必须", "务必", "一定要", "尽快"}, weight: 0.50, name: "requirement"},
			{keywords: []string{"暂不", "不采用", "否决", "驳回", "不同意"}, weight: 0.60, name: "rejection"},
			{keywords: []string{"todo", "to do", "action item", "待办"}, weight: 0.55, name: "action_item"},
			{keywords: []string{"责任人", "负责", "执行人", "owner"}, weight: 0.50, name: "assign_owner"},
		},
		lowWeight: []weightedPattern{
			{keywords: []string{"可以", "没问题", "ok", "OK", "好的"}, weight: 0.20, name: "agreement"},
			{keywords: []string{"对比", "比较", "vs", "还是", "或者", "alternative"}, weight: 0.35, name: "comparison"},
			{keywords: []string{"我觉", "我认为", "个人认为", "观点"}, weight: 0.25, name: "opinion"},
			{keywords: []string{"计划", "安排", "排期", "时间"}, weight: 0.30, name: "planning"},
			{keywords: []string{"问题", "bug", "issue", "修复"}, weight: 0.30, name: "problem"},
		},
	}
}

func (a *LexicalAnalyzer) Analyze(content string) []SignalDetail {
	var signals []SignalDetail
	lower := strings.ToLower(content)

	for _, p := range a.highWeight {
		if matched := matchWeighted(lower, p); matched != "" {
			signals = append(signals, SignalDetail{
				Category: "lexical", Name: p.name, Weight: p.weight, Matched: matched,
			})
		}
	}
	for _, p := range a.mediumWeight {
		if matched := matchWeighted(lower, p); matched != "" {
			signals = append(signals, SignalDetail{
				Category: "lexical", Name: p.name, Weight: p.weight, Matched: matched,
			})
		}
	}
	for _, p := range a.lowWeight {
		if matched := matchWeighted(lower, p); matched != "" {
			signals = append(signals, SignalDetail{
				Category: "lexical", Name: p.name, Weight: p.weight, Matched: matched,
			})
		}
	}

	return signals
}

func matchWeighted(content string, p weightedPattern) string {
	for _, kw := range p.keywords {
		if strings.Contains(content, strings.ToLower(kw)) {
			return kw
		}
	}
	return ""
}

// ============================================================
// 结构分析器 — 消息格式特征
// ============================================================

type StructuralAnalyzer struct{}

func NewStructuralAnalyzer() *StructuralAnalyzer {
	return &StructuralAnalyzer{}
}

func (a *StructuralAnalyzer) Analyze(content string) []SignalDetail {
	var signals []SignalDetail

	if matched := containsAny(content, []string{
		"1.", "2.", "3.", "第一", "第二", "第三",
		"首先", "其次", "最后",
	}); len(matched) > 0 {
		signals = append(signals, SignalDetail{
			Category: "structural", Name: "numbered_list", Weight: 0.40,
			Matched: matched[0],
		})
	}

	if matched := containsAny(content, []string{"/", "vs", "VS", "对比"}); len(matched) > 0 {
		signals = append(signals, SignalDetail{
			Category: "structural", Name: "option_comparison", Weight: 0.35,
			Matched: matched[0],
		})
	}

	if strings.Contains(content, "\"") || strings.Contains(content, "「") || strings.Contains(content, "『") {
		signals = append(signals, SignalDetail{
			Category: "structural", Name: "quoted_content", Weight: 0.20,
			Matched: "quotes_found",
		})
	}

	runeCount := utf8.RuneCountInString(content)
	hasTechTerms := containsAny(content, []string{
		"接口", "API", "数据库", "服务", "部署", "架构",
		"前端", "后端", "客户端", "协议", "格式", "版本",
	})
	if runeCount > 50 && len(hasTechTerms) > 0 {
		signals = append(signals, SignalDetail{
			Category: "structural", Name: "technical_detail", Weight: 0.35,
			Matched: "long_tech_message",
		})
	}

	if matched := containsAny(content, []string{
		"来负责", "来处", "来做", "由你", "由我", "由他",
		"assign", "负责", "owner",
	}); len(matched) > 0 {
		signals = append(signals, SignalDetail{
			Category: "structural", Name: "task_assignment", Weight: 0.45,
			Matched: matched[0],
		})
	}

	return signals
}

// ============================================================
// 对话动态分析器
// ============================================================

type DynamicAnalyzer struct{}

func NewDynamicAnalyzer() *DynamicAnalyzer {
	return &DynamicAnalyzer{}
}

func (a *DynamicAnalyzer) Analyze(content string, ctx *DetectContext) []SignalDetail {
	if ctx == nil {
		return nil
	}
	var signals []SignalDetail

	if ctx.IsReply {
		signals = append(signals, SignalDetail{
			Category: "dynamic", Name: "reply_to_discussion", Weight: 0.35,
			Matched: "is_reply",
		})
	}

	if ctx.HasMention {
		signals = append(signals, SignalDetail{
			Category: "dynamic", Name: "directed_message", Weight: 0.40,
			Matched: "has_mention",
		})
	}

	if ctx.MessageIndex > 2 {
		signals = append(signals, SignalDetail{
			Category: "dynamic", Name: "multi_round_discussion", Weight: 0.25,
			Matched: "message_index",
		})
	}

	lower := strings.ToLower(content)
	if len(ctx.RecentKeywords) > 0 {
		overlapCount := 0
		for _, kw := range ctx.RecentKeywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				overlapCount++
			}
		}
		if overlapCount >= 2 {
			signals = append(signals, SignalDetail{
				Category: "dynamic", Name: "topic_continuation", Weight: 0.30,
				Matched: "topic_overlap",
			})
		}
	}

	return signals
}

// ============================================================
// 模式匹配器 v2 — 常见决策句式模式
// ============================================================

type PatternMatcherV2 struct {
	patterns []*decisionPattern
}

type decisionPattern struct {
	name   string
	regex  *regexp.Regexp
	weight float64
}

func NewPatternMatcherV2() *PatternMatcherV2 {
	return &PatternMatcherV2{
		patterns: compilePatterns(),
	}
}

func compilePatterns() []*decisionPattern {
	rawPatterns := []struct {
		name   string
		regex  string
		weight float64
	}{
		{name: "adopt_solution", regex: `(采用|选用|使用|用)\s*[，,。.\s]*(方案|方式|方法|技术|框架|工具)`, weight: 0.75},
		{name: "decide_solution", regex: `就[用定选]`, weight: 0.70},
		{name: "decided_action", regex: `(决定|确认|同意)\s*(使用|采用|用|选|选择)`, weight: 0.80},
		{name: "final_plan", regex: `(最终|最后)[的决定的方案]`, weight: 0.50},
		{name: "confirm_plan", regex: `(可以|没问题|ok|好的|行)[，,。.．！!\s]*(就|按|照|这样)`, weight: 0.45},
		{name: "approve_proposal", regex: `(同意|批准|通过|approve)\s*(这个|该|此)`, weight: 0.80},
		{name: "preference", regex: `(倾向于|偏向|建议|推荐)\s*(使用|采用|选|用)`, weight: 0.60},
		{name: "recommend", regex: `我\s*(建议|推荐|觉得)\s*[我们]?\s*[采用使用选用]`, weight: 0.55},
		{name: "conclusion_therefore", regex: `(因此|所以|那[就么]|那就)`, weight: 0.30},
		{name: "conclusion_statement", regex: `结论[是就]`, weight: 0.75},
		{name: "conclusion_finally", regex: `总结[一下]?[：:,，]`, weight: 0.65},
		{name: "assign_task", regex: `(由|让|请)\s*[@]?\S{1,10}[来去]\s*(负责|处理|跟进|做|完成)`, weight: 0.55},
		{name: "owner_assign", regex: `([\p{Han}\w]{2,10})\s*(负责|owner|owner是)`, weight: 0.50},
		{name: "ab_selection", regex: `(用|选|采用)\s*\S{1,10}\s*(还是|或|or|vs)\s*\S{1,10}`, weight: 0.45},
		{name: "rejection", regex: `(不|别|不用|不需要|没必要)\s*(考虑|使用|采用|选)`, weight: 0.55},
		{name: "reject_proposal", regex: `(否决|驳回|不同意|反对)\s*(这个|该|此)`, weight: 0.75},
		{name: "deadline", regex: `(截止|之前|前|ddl|deadline)\s*[：:为]?\s*\d{1,2}[月/.]`, weight: 0.40},
		{name: "voting", regex: `(投票|表决|举手表决|投票决定|投票结果)`, weight: 0.70},
	}

	var compiled []*decisionPattern
	for _, rp := range rawPatterns {
		re, err := regexp.Compile(rp.regex)
		if err == nil {
			compiled = append(compiled, &decisionPattern{
				name: rp.name, regex: re, weight: rp.weight,
			})
		}
	}
	return compiled
}

func (m *PatternMatcherV2) Analyze(content string) []SignalDetail {
	var signals []SignalDetail

	for _, p := range m.patterns {
		if loc := p.regex.FindStringSubmatch(content); len(loc) > 0 {
			signals = append(signals, SignalDetail{
				Category: "pattern",
				Name:     p.name,
				Weight:   p.weight,
				Matched:  loc[0],
			})
		}
	}

	return signals
}

// ============================================================
// 工具函数
// ============================================================

func containsAny(s string, keywords []string) []string {
	var matched []string
	lower := strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			matched = append(matched, kw)
		}
	}
	return matched
}

func isMostlyEmoji(s string) bool {
	if utf8.RuneCountInString(s) > 10 {
		return false
	}
	emojiCount := 0
	totalRunes := 0
	for _, r := range s {
		totalRunes++
		if (r >= 0x1F300 && r <= 0x1F9FF) ||
			(r >= 0x2600 && r <= 0x27BF) ||
			(r >= 0xFE00 && r <= 0xFE0F) ||
			r == 0x2764 || r == 0x2763 || r == 0x2615 {
			emojiCount++
		}
	}
	if totalRunes == 0 {
		return false
	}
	return float64(emojiCount)/float64(totalRunes) > 0.5
}

func isPureQuestion(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "?") && !strings.HasSuffix(s, "？") &&
		!strings.HasSuffix(s, "吗") && !strings.HasSuffix(s, "吧") &&
		!strings.HasSuffix(s, "么") {
		return false
	}
	if len(containsAny(s, []string{"决定", "确认", "通过", "用", "选"})) > 0 {
		return false
	}
	return true
}
