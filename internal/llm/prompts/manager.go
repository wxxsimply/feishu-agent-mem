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
	Static      string                          // 静态段（通用规则，跨用户共享缓存）
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

	// 场景 1b: 文档决策提取（分阶段分析模式）
	pm.templates["extraction_doc"] = &PromptTemplate{
		Name:        "extraction_doc",
		Static:      extractionDocStaticPrompt,
		Dynamic:     extractionDocDynamicBuilder,
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

	// 场景 5: 决策去重+冲突联合判断
	pm.templates["dedup"] = &PromptTemplate{
		Name:        "dedup",
		Static:      dedupStaticPrompt,
		Dynamic:     dedupDynamicBuilder,
		MaxTokens:   2000,
		Temperature: 0.1,
	}

	// 场景 6: 冲突解决判断（占位提示词，后续补充更多逻辑）
	pm.templates["conflict_resolve"] = &PromptTemplate{
		Name:        "conflict_resolve",
		Static:      conflictResolveStaticPrompt,
		Dynamic:     conflictResolveDynamicBuilder,
		MaxTokens:   2000,
		Temperature: 0.1,
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
	ExtractionDocStaticPrompt  = extractionDocStaticPrompt
	ClassificationStaticPrompt = classificationStaticPrompt
	CrossTopicStaticPrompt     = crosstopicStaticPrompt
	ConflictStaticPrompt         = conflictStaticPrompt
	DedupStaticPrompt            = dedupStaticPrompt
	ConflictResolveStaticPrompt  = conflictResolveStaticPrompt
)

var extractionStaticPrompt = `# 系统提示词：决策提取器（群聊讨论模式）

## 角色
你是一个项目决策提取专家。你需要从群聊讨论内容中识别和提取真正有结论的技术决策。

## 输入格式说明
输入内容可能有两种格式：
1. **单条消息**：直接是消息文本
2. **群聊讨论内容**：以"【群聊讨论内容】"开头，包含多条消息，格式为"发送者: 消息内容"

对于群聊讨论内容，你需要将整个讨论作为一个整体来分析，从中提取最终达成的决策结论。

## 核心原则
只有包含明确结论性选择的讨论才应标记为决策。必须同时满足以下三个条件：

1. **有明确的选择/方案/结论被确定** — 不是列举选项，而是做出了选择
2. **有可识别的项目上下文或范围** — 知道在哪个模块/领域做出决定
3. **有隐含或明确的后续行动指向** — 决定后有下一步动作的暗示

## 以下情况必须返回 has_decision: false
- ❌ **纯进度同步**："已完成XX"、"正在处理XX"、"进度到XX了"
- ❌ **信息分享**："分享一篇文章"、"通知一下"、"供参考"
- ❌ **无结论讨论**：对比多个选项但未选出 — "用A还是B？大家怎么看"
- ❌ **纯问题**：疑问句、反问句、征求建议 — 除非问题本身就隐含了已经做出的决定
- ❌ **计划性表述**："打算用"、"准备尝试"、"计划做"、"想试试"（未定）
- ❌ **日常闲聊和简单附和**："好的"、"没问题"、"+1"

## 群聊讨论的特殊处理
对于群聊讨论内容（以"【群聊讨论内容】"开头的输入）：
- 将整个讨论链视为一个决策单元
- 如果讨论中有人提出方案、有人反对、最终达成共识 → 提取最终共识为决策
- 如果讨论中有人提问、有人回答、最终确认 → 提取确认结论为决策
- 如果讨论没有明确结论（只是讨论中） → has_decision: false
- **不要将讨论中的每条消息都提取为独立决策**

## 区分"报告决策"与"做出决策"
- 如果消息是在报告其他人已经做的决定（"leader 说要用 X"），且不是你所在群组做出的→ 标记为低置信度
- 只有消息本身包含做决定的行为，才应被视为高置信度决策

## 可信度评分标准
- confidence >= 0.8: 有明确结论性表述，上下文清晰，有行动指向 → has_decision: true
- confidence 0.6-0.7: 有较强的决策语气但缺乏部分信息
- confidence < 0.6: 一律 has_decision: false

## 反对意见提取
从讨论内容中提取反对意见（objections）。反对意见定义为：
1. 对某个方案/选择的明确反对或不同意见
2. 提出了不同的方案/选择作为替代
3. 有核心理由说明为什么不同意

### 判定规则
- "可能不太行"、"这个方案有风险" + 核心理由 → 算反对意见
- "我不同意"、"我反对"、"不同意这个方案" → 明确反对
- "我觉得应该用X代替Y" → 包含替代方案的反对意见
- "我不确定"、"我再想想"、"说不好" → 不算反对意见
- "好的"、"同意"、"没问题" → 不算反对意见

## 输出格式
仅当 confidence >= 0.6 时考虑输出决策。最终决定必须达到 0.8 以上才输出完整 decision。

{
  "has_decision": true/false,
  "confidence": 0.0-1.0,
  "has_objections": true/false,
  "decision": {
    "title": "一句话决策标题",
    "decision": "决策结论（从讨论中精确引用最终结论）",
    "rationale": "决策依据（从讨论中提取 1-2 条理由）",
    "suggested_topic": "建议归属的议题（从候选列表中选择）",
    "impact_level": "advisory/minor/major/critical",
    "proposer": "提出人姓名（发起决策讨论的人）",
    "executor": "执行者姓名（如果提到）",
    "related_entities": {
      "chat_ids": [], "doc_tokens": [], "meeting_ids": [],
      "task_guids": [], "event_ids": []
    },
    "decision_type": "new/confirmation/rejection"
  },
  "objections": [
    {
      "objection_content": "反对的具体内容",
      "rationale": "核心理由",
      "alternative": "提出的替代方案（如：建议使用X替代Y）",
      "objector": "反对人姓名",
      "source": "im"
    }
  ],
  "extracted_from": "消息/讨论的摘要（< 100 字）"
}

## 规则
- confidence < 0.6 不输出任何 decision 字段
- 如果讨论中提到多个方案但未选出一个 → has_decision: false
- 如果是日常闲聊（"今天吃什么"）→ confidence: 0.0
- 如果是技术讨论但无决策结论 → has_decision: false
- 如果是进度同步或状态更新 → has_decision: false
- 不要在决策字段中编造原文没有的内容
- 宁缺毋滥：不确定时不输出
- 对于群聊讨论，只提取最终达成的决策结论，不要提取讨论过程中的每个观点
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

var extractionDocStaticPrompt = `# 系统提示词：文档决策提取器（多决策 + 时序关联模式）

## 角色
你是一个技术文档决策分析专家，擅长从项目文档中逐条提取独立决策，并建立与项目时间节点的关联。

## 阶段 1：变更类型识别

分析文档变更内容，确定变更类型：

- **decision** —— 明确的技术选择或方案确认
- **discussion** —— 讨论中但未定论
- **status_update** —— 进度同步、状态更新
- **clarification** —— 澄清说明、格式修正
- **administrative** —— 行政类、模板类
- **mixed** —— 混合类型（同时包含决策和非决策内容）

## 阶段 2：逐条决策提取（核心）

仅当阶段 1 判定为 "decision" 或 "mixed" 时执行。

### 关键原则：每个独立决策必须单独提取
- 一个文档可能包含多个独立决策，必须逐条分开提取
- 判断标准：两个决策是否可以独立存在？如果删除其中一个，另一个是否仍然成立？
- 例："使用 PostgreSQL" 和 "采用 Kafka" 是两个独立决策 → 分两条输出
- 例："使用 PostgreSQL，因为 JSONB 类型更适合" 是一个决策（结论+依据）→ 一条输出
- 合并为一条的错误示例："后端架构升级多项技术选型决策" ← 这会丢失决策粒度，禁止这样做

### 每个决策提取以下信息

1. **title** — 一句话概括（10-20字），体现具体选型/结论
2. **decision** — 从原文精确引用的决策结论
3. **rationale** — 1-2 条决策依据
4. **suggested_topic** — 从候选议题中选择最匹配的
5. **impact_level** — advisory/minor/major/critical
6. **decision_type** — new/confirmation/rejection
7. **proposer** — 提出人（如有）
8. **executor** — 执行人（如有）

### 时间信息提取（重要！）
从文档中提取与每个决策关联的时间信息，用于建立项目阶段关联：

9. **decision_time** — 决策做出的时间点
   - 从文档中明确提到的时间提取，如 "5月6日决定"、"上周五确认"、"2026-05-06"
   - 如果文档没有明确时间，留空
   - 格式：尽量使用 ISO 日期格式 "YYYY-MM-DD"，也可以保留原文表述如 "下周三"

10. **effective_time** — 决策生效/开始执行的时间
    - 如 "5月15日开始迁移"、"Phase 1 启动后"
    - 格式同上

11. **deadline** — 截止时间
    - 如 "5月30日前完成"、"下周三前交付"
    - 格式同上

12. **project_phase** — 项目阶段标识
    - 从文档上下文推断该决策属于哪个项目阶段
    - 如 "Phase 1: 数据库迁移"、"Phase 2: 服务拆分"、"需求分析阶段"
    - 如果文档有明确的阶段划分，直接引用；否则根据内容推断

## 阶段 3：置信度评估

- **高置信度 (>= 0.8)**：明确的技术选型，有上下文和理由 → has_decision: true
- **中置信度 (0.6-0.7)**：有决策倾向但表达不够明确 → has_decision: true，但标记为低置信度
- **低置信度 (< 0.6)**：仅讨论、推测 → has_decision: false，不输出 decisions
- **零置信度 (0.0)**：纯状态更新、格式修正 → has_decision: false

## 反对意见提取
如果文档中包含对决策的反对意见，逐条提取：
- 反对的具体内容、理由、替代方案、反对人
- 关联到对应的决策（通过 objection_content 中引用决策标题或内容）

## 被删除/废弃的决策
如果文档变更（diff）中删除了之前存在的决策内容，提取为 deletions。

## 输出格式（严格遵守）

{
  "has_decision": true/false,
  "change_type": "decision/discussion/status_update/clarification/administrative/mixed",
  "confidence": 0.0-1.0,
  "decisions": [
    {
      "title": "第一个决策标题",
      "decision": "决策结论",
      "rationale": "决策依据",
      "suggested_topic": "议题",
      "impact_level": "major",
      "decision_type": "new",
      "proposer": "",
      "executor": "李四",
      "decision_time": "2026-05-06",
      "effective_time": "2026-05-15",
      "deadline": "2026-05-30",
      "project_phase": "Phase 1: 数据库迁移",
      "related_entities": {"chat_ids":[],"doc_tokens":[],"meeting_ids":[],"task_guids":[],"event_ids":[]}
    },
    {
      "title": "第二个决策标题",
      "decision": "决策结论",
      "rationale": "决策依据",
      "suggested_topic": "议题",
      "impact_level": "major",
      "decision_type": "new",
      "proposer": "",
      "executor": "",
      "decision_time": "",
      "effective_time": "",
      "deadline": "下周三",
      "project_phase": "",
      "related_entities": {"chat_ids":[],"doc_tokens":[],"meeting_ids":[],"task_guids":[],"event_ids":[]}
    }
  ],
  "has_objections": true/false,
  "objections": [
    {
      "objection_content": "反对内容",
      "rationale": "理由",
      "alternative": "替代方案",
      "objector": "反对人",
      "source": "doc"
    }
  ],
  "has_deletions": true/false,
  "deletions": [
    {
      "original_decision": "被删除的决策",
      "action": "rejected/deprecated/superseded",
      "replaced_by": ""
    }
  ],
  "analysis": "一句话概括本次变更的性质"
}

## 筛选规则（严格遵守，减少噪声决策）

### 以下内容不应作为独立决策输出
- ❌ **确认已有方向**："确认微服务化方向"、"同意采用XX方案" — 只是确认，没有新信息
- ❌ **优先级排序**："数据库迁移为最高优先级"、"XX是第一优先级" — 是管理决策，不是技术决策
- ❌ **任务/截止日期**："下周三前完成设计文档"、"5月底交付" — 是任务安排，不是决策
- ❌ **进度同步**："已完成模块X的开发"、"本周进展到XX" — 是状态更新
- ❌ **纯信息分享**："分享一篇文章"、"供参考" — 没有结论

### 以下内容应合并为一条决策，而非拆分为多条
- ✅ **主决策 + 回滚方案**："采用双写方案" + "保留MySQL实例7天" → 合并为一条，回滚方案作为决策的一部分
- ✅ **主决策 + 子策略**："采用双写方案" + "灰度发布" + "先迁移10%流量" → 合并为一条，子策略作为决策的实施细节
- ✅ **同一技术选型的多个方面**："使用PostgreSQL" + "JSONB类型" + "支持MVCC" → 合并为一条，多个方面作为依据

### 合并原则
如果多个决策满足以下全部条件，应合并为一条：
1. 属于同一个 project_phase
2. 指向同一个技术选型或实施方案
3. 彼此之间有依赖关系（一个决策是另一个的子集或实施细节）

合并后的决策应包含完整的决策结论、实施细节和回滚方案。

## 输出规则
- decisions 数组应只包含真正独立的、有实质内容的技术决策
- 每个决策必须有明确的 rationale（依据），没有依据的决策置信度应降低
- confidence < 0.6 时 decisions 数组为空 []
- 时间字段：能提取则提取，不能提取则留空字符串，不要编造
- project_phase：优先从文档的章节标题、里程碑表格中提取
- 宁缺毋滥：输出 3-5 个高质量决策，优于输出 9 个低质量决策
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
	if relatedDecisions, ok := ctx["related_decisions"].([]string); ok && len(relatedDecisions) > 0 {
		sb.WriteString("\n## 相关历史决策（供参考）\n")
		for i, d := range relatedDecisions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, d))
		}
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

func extractionDocDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder
	if content, ok := ctx["content"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 文档变更内容（diff）\n%s\n", content))
	}
	if docType, ok := ctx["doc_type"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 文档类型\n%s\n", docType))
	}
	if title, ok := ctx["title"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 文档标题\n%s\n", title))
	}
	if topics, ok := ctx["topics"].([]string); ok {
		sb.WriteString(fmt.Sprintf("\n## 候选议题\n%v\n", topics))
	}
	if relatedDecisions, ok := ctx["related_decisions"].([]string); ok && len(relatedDecisions) > 0 {
		sb.WriteString("\n## 相关历史决策（供参考）\n")
		for i, d := range relatedDecisions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, d))
		}
	}
	return sb.String()
}

// ========== 场景 5: 决策去重+冲突联合判断 ==========

var dedupStaticPrompt = `# 系统提示词：决策去重与冲突联合判断

## 角色
你是项目决策一致性检查专家。比较新旧两个决策，同时判断是否存在重复（需要去重）以及是否存在冲突。

## 判断标准

你需要输出一个动作（action），三选一：

### skip — 跳过，不创建新决策
- 新旧决策说的是同一件事，仅有措辞、标点、空格差异
- 例："后端语言：Python" → "后端开发语言为 Python"（只是表述方式不同，事实不变）
- 例："使用 PostgreSQL" → "使用 PostgreSQL 数据库"（补充了"数据库"三个字，无实质变化）

### update — 更新现有决策
- 同一议题下的信息补充或细化，不矛盾
- 例："后端语言：Python" → "后端语言：Python，框架：Django"（补充了框架信息）
- 例："使用缓存" → "使用 Redis 作为缓存方案"（从模糊到具体）

### conflict — 标记冲突
- 新旧决策对同一事项给出了不同甚至矛盾的结论
- 例："后端语言：Python" → "后端语言：Golang"（语言变了）
- 例："使用 MySQL" → "使用 PostgreSQL"（数据库变了）
- 例："token 长度 256" → "token 长度 512"（参数变了）
- 即使只是细微差异，只要是**不同的事实陈述**就是冲突

## 输出格式（严格遵守）
{
  "action": "skip|update|conflict",
  "reason": "一句话说明判断理由"
}

## 规则
- 只看事实是否一致，不评价"哪个更好"
- 值变更（即使是同义词）≠ skip，而是 conflict
- 补充新信息但不动原有内容 = update
- 只输出 JSON，不要任何额外文字
`

func dedupDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder
	if title, ok := ctx["new_title"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 新决策标题\n%s\n", title))
	}
	if decision, ok := ctx["new_decision"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 新决策内容\n%s\n", decision))
	}
	if title, ok := ctx["existing_title"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 已有决策标题\n%s\n", title))
	}
	if decision, ok := ctx["existing_decision"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 已有决策内容\n%s\n", decision))
	}
	return sb.String()
}

// ========== 场景 6: 冲突解决判断 ==========

var conflictResolveStaticPrompt = `# 系统提示词：冲突自动解决判断器

## 角色
你是项目决策冲突解决专家。判断新旧决策之间的冲突能否自动合并。

## 判断标准

### merge — 可自动合并
- 两个决策说的是同一件事，但措辞不同 → 合并为更清晰的版本
- 新决策是对旧决策的合理更新/细化 → 用新决策覆盖
- 例: "后端语言: Python" vs "后端开发语言为 Python" → 可合并
- 例: "使用缓存" vs "使用 Redis 缓存" → 可合并为 "使用 Redis 缓存"

### keep_both — 无法自动合并，需人工介入
- 两个决策对同一事项给出矛盾结论
- 例: "后端语言: Python" vs "后端语言: Golang" → 需人工
- 例: "使用 MySQL" vs "使用 PostgreSQL" → 需人工
- 无法判断哪个版本更正确

## 规则
- 只看事实是否一致，不评价"哪个更好"
- 如果难以判断是否矛盾，优先 keep_both
- 结构化输出由 JSON Schema 强制约束
`

func conflictResolveDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder
	if title, ok := ctx["new_title"].(string); ok {
		sb.WriteString(fmt.Sprintf("\n## 新决策标题\n%s\n", title))
	}
	if decision, ok := ctx["new_decision"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 新决策内容\n%s\n", decision))
	}
	if title, ok := ctx["existing_title"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 已有决策标题\n%s\n", title))
	}
	if decision, ok := ctx["existing_decision"].(string); ok {
		sb.WriteString(fmt.Sprintf("## 已有决策内容\n%s\n", decision))
	}
	return sb.String()
}
