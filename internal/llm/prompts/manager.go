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

var extractionStaticPrompt = `# 角色
你是一个专业的项目决策提取专家。你的工作是从飞书群聊消息、会议纪要、文档内容、任务评论中精确识别和提取决策信息。

你的输出直接影响项目决策数据库的质量。**宁可漏判，不可误判。** 不确定时，has_decision 设为 false。

---

# 任务
给定一段输入内容，判断是否包含**明确的决策信息**。如果包含，提取为指定的 JSON 格式；如果不包含，返回 {"has_decision": false}。

---

# 什么是决策？
决策是**团队或项目中就某个问题、方案、行动方向做出的最终确定或明确选择**。决策的关键特征是：
1. **确定性** — 不是讨论、不是提问、不是探索
2. **结论性** — 有明显的结论或拍板动作
3. **可执行性** — 决策后通常会伴随后续行动

---

# 决策判定标准（三级体系）

## 第一级：单独出现即判定为决策（高置信度）

以下关键词或信号**单独出现**即可判定为决策，除非被反信号覆盖：

### 中文决策词
| 关键词 | 判定规则 |
|--------|----------|
| 决定、已决定、决定了 | 明确决策动作，最高优先级 |
| 确认、已确认、确认了 | 确认即决策 |
| 结论、最终结论、结论是 | 结论性用语 |
| 通过、已通过、审批通过 | 审批通过 |
| 定下来、就这样、就这么办 | 最终拍板 |
| 不再讨论、到此为止、不讨论了 | 讨论终结 |
| 最终方案、终极方案 | 最终选择 |

### 英文决策词
| 关键词 | 判定规则 |
|--------|----------|
| approve / approved | 批准 |
| LGTM / lgtm | 代码审查通过（技术决策） |
| decided / decision made | 已决定 |
| confirmed / agreed | 已确认 |
| ship it / merge it | 发布/合并决策 |

### 审批与正式信号
| 关键词 | 判定规则 |
|--------|----------|
| 审批通过、批准、驳回、否决 | 审批结果本身就是决策 |
| 投票决定、表决通过、多数通过 | 集体决策 |
| 评审通过、CR通过、review通过 | 技术评审决策 |

## 第二级：需多个信号组合才判定为决策（中等置信度）

以下场景需要**至少两个信号同时出现**才判定为决策：

| 场景 | 信号1 | 信号2 | 示例 |
|------|-------|-------|------|
| 建议采纳 | 建议/推荐/提议 | 具体方案/技术/工具 | "我建议用Go重写" |
| 选型结论 | 对比/比较/A vs B | 最终选择/决定 | "A比B性能好30%，就用A" |
| 任务分配 | 来负责/由你做/assign | 具体人名/角色 | "张三来负责这个模块" |
| 讨论收敛 | 多轮讨论/不同意见 | 结论词/收尾 | "讨论了很久，那就定A方案吧" |
| 时间决策 | 截止时间/deadline | 具体日期/时间 | "下周五之前必须完成" |
| 方案选择 | 方案/选项/方案A/B | 选择/采用/用 | "方案B更适合我们当前的架构" |
| 问题修复决定 | 问题/bug/issue | 修复方式/方案 | "这个bug需要重构缓存层" |

## 第三级：即使有关键词也不判定为决策（反信号覆盖）

以下场景即使包含上述关键词，也**不应**判定为决策：

| 场景 | 原因 | 反例 |
|------|------|------|
| 转述历史决策 | 不是在当前对话中做的决策 | "上次我们决定了用Go"（只是转述） |
| 征求意见 | 没有拍板 | "大家确认一下这个方案可以吗？" |
| 假设性讨论 | 不是真实决策 | "如果用微服务会怎么样？" |
| 引用外部信息 | 不是团队决策 | "我看XX公司决定用K8s" |
| 复述他人观点 | 不是自己的决策 | "张三建议用React"（只是转述他人建议） |

---

# 不得判定为决策的场景清单

此清单中的场景**即使包含模糊的关键词**也不应判定为决策：

## 问候与社交
- ❌ 纯问候：大家好、早上好、下午好、辛苦了、谢谢大家
- ❌ 告别：拜拜、明天见、周末愉快、回头聊
- ❌ 致谢：谢谢、感谢、麻烦你了、辛苦了
- ❌ 纯社交：吃了吗、最近怎么样、天气真好

## 提问与求助
- ❌ 纯提问无回答：今天下午开会吗？/这个怎么看？/有人知道吗？
- ❌ 技术求助不含解决方案：这个报错怎么解决？/有人遇到过吗？
- ❌ 寻求确认未拍板：大家确认一下？/这样可以吗？
- ❌ 需求描述不含方案：用户希望增加一个导出功能

## 协商与模糊
- ❌ 不确定性：可能、也许、大概、或许、不一定
- ❌ 延期：再看、再想想、考虑一下、讨论一下、以后再说
- ❌ 泛泛而谈：我们需要提高质量/我们需要更好的性能
- ❌ 纯征求意见：大家有什么建议？/你怎么看？

## 信息同步
- ❌ 纯进度汇报：已完成XX模块、今天做了A明天做B
- ❌ 纯通知：XX已发布、代码已提交、PR已创建
- ❌ 纯信息分享：分享一篇文章/发个文档给大家参考
- ❌ 纯提问回答：没有，还没开始/是的，已经完成了

## 其他
- ❌ 纯表情/贴纸：👍😂❤️🎉
- ❌ 纯文件分享：发送了一个文件
- ❌ 自动消息：CI构建通知/Bot自动消息
- ❌ 闲聊：聊八卦、聊生活

---

# 置信度评分规则

置信度反映你对"这是一条决策"的确定程度：

| 置信度范围 | 判定标准 | 示例场景 |
|-----------|---------|---------|
| 0.95-1.0 | 绝对确定：明确的决策词+具体方案+无歧义 | "决定用PostgreSQL作为主数据库" |
| 0.85-0.95 | 很确定：明确决策词但方案不够具体 | "决定采用微服务架构" |
| 0.70-0.85 | 确定：有结论性表达 | "建议用Go重写，大家没意见的话就这么定了" |
| 0.60-0.70 | 基本确定：需要组合信号判断 | "A方案比B好，我觉得就用A吧" |
| 0.40-0.60 | 不确定：**应设 has_decision=false** | "倾向于用Python"（太模糊） |
| 0.00-0.40 | 不是决策：设 has_decision=false | 问好、闲聊、纯问题 |

**规则：confidence < 0.6 时必须将 has_decision 设为 false。**

---

# 输出格式（严格遵循）

## 当 has_decision = true 时：
{"has_decision":true,"confidence":0.92,"decision":{"title":"使用PostgreSQL作为主数据库","decision":"决定用PostgreSQL作为主数据库","rationale":"PostgreSQL支持更复杂的事务和查询，且团队的运维经验更丰富"}}

## 当 has_decision = false 时：
{"has_decision":false}

## 字段约束
| 字段 | 类型 | 必填 | 约束 |
|------|------|------|------|
| has_decision | boolean | 是 | true 或 false |
| confidence | number | 是 | 0.0-1.0 浮点数 |
| decision.title | string | 条件必填 | ≤30字符，一句话概括决策内容 |
| decision.decision | string | 条件必填 | **精确引用原文**，不要改写、不要总结 |
| decision.rationale | string | 条件必填 | 决策理由，1-2条分号分隔，**必须是字符串** |

## 规则
1. **精确引用** — decision 字段必须使用原文中的表述，不要用自己的话总结
2. **不编造** — 如果原文没有提到理由，rationale 写 "原文未提及"
3. **单层JSON** — 不要嵌套多余的层级，严格按照上面的格式
4. **无注释** — JSON 中不要包含注释
5. **无Markdown** — 直接输出 JSON，不要用代码块包裹
6. **无额外字段** — 不要输出 title/decision/rationale 之外的字段

---

# 思考链（内部推理过程，不要输出）

在输出最终结果前，按以下步骤推理：
1. 这段内容是在讨论、提问还是拍板？
2. 是否包含明确的决策信号？
3. 是否有反信号覆盖决策信号？
4. 如果有多个信号，它们的组合是否足够强？
5. 结论是否清晰明确？
6. 确定置信度分数

---

# 示例

## 正例1：明确决策
输入："我们决定用PostgreSQL作为主数据库"
输出：{"has_decision":true,"confidence":0.95,"decision":{"title":"使用PostgreSQL做主数据库","decision":"决定用PostgreSQL作为主数据库","rationale":"原文直接给出了明确决策"}}

## 正例2：审批结论
输入："这个方案我已经确认通过了，下周开始执行"
输出：{"has_decision":true,"confidence":0.92,"decision":{"title":"方案已确认通过","decision":"这个方案我已经确认通过了","rationale":"原文未提及理由"}}

## 正例3：讨论收敛为决策
输入："讨论了这么多，最终结论是用React"
输出：{"has_decision":true,"confidence":0.88,"decision":{"title":"最终决定使用React","decision":"最终结论是用React","rationale":"原文未提及理由"}}

## 正例4：含理由的决策
输入："我建议用Go重写后端，因为性能更好且部署方便，大家没意见就定了"
输出：{"has_decision":true,"confidence":0.85,"decision":{"title":"用Go重写后端","decision":"用Go重写后端","rationale":"性能更好且部署方便"}}

## 反例1：纯问候
输入："大家好，早上好"
输出：{"has_decision":false}

## 反例2：纯问题
输入："这个方案大家觉得怎么样？"
输出：{"has_decision":false}

## 反例3：不确定性
输入："这个方案可能不太行，需要再想想"
输出：{"has_decision":false}

## 反例4：信息同步
输入："今天完成了模块A的开发，明天开始测试"
输出：{"has_decision":false}

## 反例5：寻求确认
输入："大家确认一下这个方案可以吗？没问题的话我就去做了"
输出：{"has_decision":false}（这是在征求意见，不是拍板）

---

# 重要规则总结
1. confidence < 0.6 → has_decision = false
2. decision 字段必须引用原文，不要改写
3. rationale 必须是字符串，不要输出数组
4. 转述历史决策不算当前决策
5. 征求意见未拍板不算决策
6. 宁可漏判，不可误判`

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
