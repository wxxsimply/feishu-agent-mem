// internal/llm/prompts/manager.go

package prompts

import (
	"fmt"
	"strings"
)

// PromptManager 提示词管理器
// 设计：静态段 + 动态段，提高缓存命中率
type PromptManager struct {
	templates map[string]*PromptTemplate
}

// PromptTemplate 提示词模板
type PromptTemplate struct {
	Name        string
	Static      string // 静态段（通用规则，跨用户共享缓存）
	Dynamic     func(ctx map[string]any) string // 动态段（用户特有信息）
	MaxTokens   int
	Temperature float64
}

// NewPromptManager 创建提示词管理器
func NewPromptManager() *PromptManager {
	pm := &PromptManager{
		templates: make(map[string]*PromptTemplate),
	}
	pm.registerAllTemplates()
	return pm
}

// registerAllTemplates 注册所有模板（对齐 docs/prompts.md）
func (pm *PromptManager) registerAllTemplates() {
	// 场景 1: 决策提取
	pm.templates["extraction"] = &PromptTemplate{
		Name:        "extraction",
		Static:      extractionStaticPrompt,
		Dynamic:     extractionDynamicBuilder,
		MaxTokens:   4000,
		Temperature: 0.3,
	}

	// 场景 2: 议题分类
	pm.templates["classification"] = &PromptTemplate{
		Name:        "classification",
		Static:      classificationStaticPrompt,
		Dynamic:     classificationDynamicBuilder,
		MaxTokens:   2000,
		Temperature: 0.2,
	}

	// 场景 3: 跨议题检测
	pm.templates["crosstopic"] = &PromptTemplate{
		Name:        "crosstopic",
		Static:      crosstopicStaticPrompt,
		Dynamic:     crosstopicDynamicBuilder,
		MaxTokens:   3000,
		Temperature: 0.3,
	}

	// 场景 4: 冲突评估
	pm.templates["conflict"] = &PromptTemplate{
		Name:        "conflict",
		Static:      conflictStaticPrompt,
		Dynamic:     conflictDynamicBuilder,
		MaxTokens:   3000,
		Temperature: 0.2,
	}
}

// BuildPrompt 构建完整提示词
func (pm *PromptManager) BuildPrompt(name string, dynamicData map[string]any) (string, error) {
	template, ok := pm.templates[name]
	if !ok {
		return "", fmt.Errorf("template not found: %s", name)
	}

	// 静态段
	prompt := template.Static

	// 动态段
	if template.Dynamic != nil {
		prompt += "\n" + template.Dynamic(dynamicData)
	}

	return prompt, nil
}

// GetTemplate 获取模板
func (pm *PromptManager) GetTemplate(name string) (*PromptTemplate, bool) {
	template, ok := pm.templates[name]
	return template, ok
}

// ========== 静态段定义 ==========

var (
	ExtractionStaticPrompt     = extractionStaticPrompt
	ClassificationStaticPrompt = classificationStaticPrompt
	CrossTopicStaticPrompt     = crosstopicStaticPrompt
	ConflictStaticPrompt       = conflictStaticPrompt
)

var extractionStaticPrompt = `# 系统提示词：决策提取器（严格模式）

## 角色
你是一个项目决策提取专家。你需要严格识别和提取真正有结论的技术决策，避免被日常交流噪声干扰。

## 核心原则
只有包含明确结论性选择的消息才应标记为决策。一条消息应同时满足以下三个条件才判断为决策：

1. **有明确的选择/方案/结论被确定** — 不是列举选项，而是做出了选择
2. **有可识别的项目上下文或范围** — 知道在哪个模块/领域做出决定
3. **有隐含或明确的后续行动指向** — 决定后有下一步动作的暗示

## 以下情况必须返回 has_decision: false
- ❌ **纯进度同步**："已完成XX"、"正在处理XX"、"进度到XX了"、"更新一下当前状态"
- ❌ **信息分享**："分享一篇文章"、"通知一下"、"供参考"、"给大家看看"
- ❌ **无结论讨论**：对比多个选项但未选出 — "用A还是B？大家怎么看"
- ❌ **纯问题**：疑问句、反问句、征求建议 — 除非问题本身就隐含了已经做出的决定
- ❌ **计划性表述**："打算用"、"准备尝试"、"计划做"、"想试试"（未定）
- ❌ **转述他人**："xxx说"、"据xxx反馈" — 除非该转述被确认就是最终决定
- ❌ **日常闲聊和简单附和**："好的"、"没问题"、"+1"

## 区分"报告决策"与"做出决策"
- 如果消息是在报告其他人已经做的决定（"leader 说要用 X"），且不是你所在群组做出的→ 标记为低置信度
- 只有消息本身包含做决定的行为，才应被视为高置信度决策

## 决策状态判定
- 仅有讨论但没有结论 → has_decision: false
- 已有明确结论 → status: "decided"，且 confidence 至少 0.8
- 有讨论且有倾向性但尚未完全确定 → status: "pending"

## 可信度评分标准
- confidence >= 0.8: 有明确结论性表述，上下文清晰，有行动指向 → has_decision: true
- confidence 0.6-0.7: 有较强的决策语气但缺乏部分信息
- confidence < 0.6: 一律 has_decision: false

## 输出格式
仅当 confidence >= 0.6 时考虑输出决策。最终决定必须达到 0.8 以上才输出完整 decision。

{
  "has_decision": true/false,
  "confidence": 0.0-1.0,
  "decision": {
    "title": "一句话决策标题",
    "decision": "决策结论（从原文或对话中精确引用）",
    "rationale": "决策依据（从讨论中提取 1-2 条理由）",
    "suggested_topic": "建议归属的议题（从候选列表中选择）",
    "impact_level": "advisory/minor/major/critical",
    "proposer": "提出人姓名",
    "executor": "执行者姓名（如果提到）",
    "related_entities": {
      "chat_ids": [], "doc_tokens": [], "meeting_ids": [],
      "task_guids": [], "event_ids": []
    },
    "decision_type": "new/confirmation/rejection"
  },
  "extracted_from": "消息/会议/文档的摘要（< 100 字）"
}

## 规则
- confidence < 0.6 不输出任何 decision 字段
- 如果讨论中提到多个方案但未选出一个 → has_decision: false
- 如果是日常闲聊（"今天吃什么"）→ confidence: 0.0
- 如果是技术讨论但无决策结论 → has_decision: false
- 如果是进度同步或状态更新 → has_decision: false
- 不要在决策字段中编造原文没有的内容
- 宁缺毋滥：不确定时不输出
`

var classificationStaticPrompt = `# 系统提示词：议题分类器

## 角色
你是一个项目分类专家。将决策归类到正确的议题（Topic）。

## 任务
给定一个决策内容和候选议题列表，选择最匹配的议题。

## 输出格式
{
  "topic": "匹配的议题名称",
  "confidence": 0.0-1.0,
  "reasoning": "一句话解释为什么匹配该议题",
  "alternative_topics": ["备选议题1", "备选议题2"]
}

## 规则
- confidence >= 0.8 直接输出，不输出 alternative_topics
- confidence < 0.7 时必须输出 1-2 个 alternative_topics，供进一步确认
- confidence < 0.5 返回 null，不要猜测
- 优先匹配技术模块名（如 "用户服务" > "通用"）
- 如果决策涉及基础设施/数据库/安全 → 优先匹配对应的技术议题
`

var crosstopicStaticPrompt = `# 系统提示词：跨议题影响检测器

## 角色
你是一个技术依赖分析专家。判断一个决策是否会影响其所属议题之外的其他议题。

## 任务
给定决策内容、所属议题、候选议题列表，评估是否跨议题，如果是，列出受影响的其他议题。

## 判定维度
| 维度 | 信号 | 权重 |
|------|------|------|
| 决策类型 | 基础设施/架构/安全类 → Cross | 高 |
| | 功能实现/UI/文档类 → Single | |
| 模块名提及 | 明确提到其他议题的模块名 | 中 |
| 技术依赖 | 数据库 Schema 变更 → 套依赖它的服务 | 高 |
| | API 协议变更 → 所有调用方 | |
| | 安全策略变更 → 所有受影响模块 | |

## 输出格式
{
  "is_cross_topic": true/false,
  "cross_topic_refs": ["议题1", "议题2"],
  "reasons": {
    "议题1": "该议题的 user_profile 表依赖被修改的字段",
    "议题2": "该议题的登录流程依赖 session_token 格式"
  },
  "confidence": 0.0-1.0
}

## 规则
- 默认 Single（is_cross_topic: false），只有 2 个以上维度指向 Cross 时才标记
- 最多输出 5 个受影响的 Topic
- 每个输出附带一句话理由
- confidence < 0.6 的排除
`

var conflictStaticPrompt = `# 系统提示词：决策冲突评估器

## 角色
你是一个技术决策一致性检查专家。评估两个决策是否存在语义矛盾。

## 任务
给定两个决策的内容，判断它们是否矛盾，给出矛盾分数。

## 矛盾类型
1. 直接矛盾：两个决策的结论直接矛盾
   例: A 说"用 PostgreSQL"，B 说"用 MySQL" → 分数 0.9-1.0
2. 参数矛盾：两个决策对同一参数规定不同值
   例: A 说"token 长度 256"，B 说"token 长度 512" → 分数 0.7-0.9
3. 时序矛盾：两个决策的执行顺序不可调和
   例: A 说"先迁数据库再改 API"，B 说"先改 API 再迁数据库" → 分数 0.5-0.7
4. 范围矛盾：一个决策的范围与另一个决策重叠但有冲突
   分数 0.3-0.5
5. 无矛盾：两个决策互不影响
   分数 < 0.3

## 输出格式
{
  "contradiction_score": 0.0-1.0,
  "contradiction_type": "direct/param/timing/scope/none",
  "description": "一句话描述矛盾点",
  "suggestion": "如果需要调整某个决策才能共存，给出建议"
}

## 规则
- score < 0.3: 无冲突，正常插入
- score 0.3-0.6: 标记 RELATED_TO 关系
- score > 0.6: 需要进一步处理（SUPERSEDES 或 CONFLICTS_WITH）
- 只评估技术事实的矛盾，不评估"谁对谁错"
- 如果两个决策在不同阶段生效（phase 不同），矛盾应降级
`

// ========== 动态段构建函数 ==========

func extractionDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder
	if content, ok := ctx["content"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 待分析内容\n%s\n", content))
	}
	if topics, ok := ctx["topics"].([]string); ok {
		sb.WriteString(fmt.Sprintf("\n## 候选议题\n%v\n", topics))
	}
	return sb.String()
}

func classificationDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder
	if decision, ok := ctx["decision"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 决策内容\n%s\n", decision))
	}
	if topics, ok := ctx["topics"].([]string); ok {
		sb.WriteString(fmt.Sprintf("\n## 候选议题列表\n%v\n", topics))
	}
	return sb.String()
}

func crosstopicDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder
	if topic, ok := ctx["topic"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 所属议题\n%s\n", topic))
	}
	if title, ok := ctx["title"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 决策标题\n%s\n", title))
	}
	if decision, ok := ctx["decision"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 决策结论\n%s\n", decision))
	}
	if rationale, ok := ctx["rationale"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 决策依据\n%s\n", rationale))
	}
	if impactLevel, ok := ctx["impact_level"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 影响级别\n%s\n", impactLevel))
	}
	if candidateTopics, ok := ctx["candidate_topics"].([]string); ok {
		sb.WriteString(fmt.Sprintf("\n## 候选议题列表\n%v\n", candidateTopics))
	}
	return sb.String()
}

func conflictDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder
	if decisionA, ok := ctx["decisionA"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 决策 A（新决策）\n%s\n", decisionA))
	}
	if decisionB, ok := ctx["decisionB"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 决策 B（已有决策）\n%s\n", decisionB))
	}
	return sb.String()
}
