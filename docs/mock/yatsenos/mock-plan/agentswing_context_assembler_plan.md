# AgentSwing 并行上下文组装方案

## 背景与目标

### 当前问题
当前 `ContextAssembler` 使用**单一贪婪策略**，对所有适配器类型都一样：
- 按固定顺序插入决策和适配器结果，直到预算耗尽
- 没有区分 IM、Doc、Wiki 的特性差异
- 没有多策略并行探索
- 没有前瞻性评估哪个上下文能产生更好的决策提取

### 各适配器特性差异

| 特性 | IM (群聊) | Doc (文档) | Wiki (知识库) |
|------|-----------|------------|--------------|
| **内容形态** | 流式对话，多轮交互 | 单个长文档，结构化 | 层级结构，多节点 |
| **时间依赖** | 强顺序依赖，前序消息很重要 | 版本历史，增量更新 | 节点变更，树结构 |
| **上下文连贯性** | 需要完整对话历史来理解决策 | 需要文档前后文 | 需要节点层级关系 |
| **决策密度** | 分散在多轮对话中 | 集中在文档关键段落 | 分布在多个节点 |
| **信号来源** | 消息、回复、提及、情绪 | 文档内容、评论、版本diff | 节点内容、移动、创建 |

### 借鉴 AgentSwing 的核心思想
1. **并行上下文管理**：同时生成多种策略的上下文变体
2. **Lookahead 轻量评估**：评估每个上下文的"决策提取友好度"
3. **自适应选择**：选择最有可能提取到准确决策的上下文

---

## 方案设计

### 核心架构

#### 1. 适配器特定策略映射

```go
// 适配器策略映射
var AdapterStrategyMap = map[AdapterType][]ContextStrategy {
    AdapterIM: {
        StrategyIMConversation,      // 完整对话流优先
        StrategyIMRecentMessages,    // 最近消息优先
        StrategyIMThreadView,        // 线索/回复视图
        StrategyIMTopicRelevant,     // 话题相关消息
    },
    AdapterDocs: {
        StrategyDocFullContent,      // 完整文档内容
        StrategyDocDiffFocused,      // 差异部分聚焦
        StrategyDocStructure,        // 结构化（标题+段落）
        StrategyDocCommentPriority,  // 评论优先
    },
    AdapterWiki: {
        StrategyWikiTreeContext,     // 树结构上下文
        StrategyWikiNodeFocused,     // 当前节点聚焦
        StrategyWikiRecentUpdated,   // 最近更新优先
        StrategyWikiPathRelevance,   // 路径相关节点
    },
    AdapterCalendar: {
        StrategyCalendarEventChain,  // 事件链
        StrategyCalendarMeetings,    // 会议记录优先
    },
    AdapterVC: {
        StrategyVCTranscriptFlow,    // 转录流
        StrategyVCActionItems,       // 行动项优先
    },
    AdapterTask: {
        StrategyTaskProgression,     // 任务进度
    },
}
```

#### 2. ContextVariant 上下文变体

```go
type ContextVariant struct {
    // 策略标识
    Strategy ContextStrategy
    AdapterType AdapterType

    // 实际内容
    Context *AssembledContext

    // 评估分数
    ExtractabilityScore float64 // 0.0-1.0，"决策提取友好度"
    CoherenceScore float64      // 0.0-1.0，上下文连贯性
    TokenEstimate      int

    // 轻量 lookahead 结果
    Preview *DecisionExtract // 基于这个上下文的决策预提取结果
}
```

#### 3. ContextStrategy 策略接口

```go
type ContextStrategy interface {
    Name() string
    SupportedAdapters() []AdapterType
    Assemble(
        signal *StateChangeSignal,
        queryResults map[AdapterType]any,
        existingDecisions []*decision.DecisionNode,
        budget int,
    ) (*AssembledContext, error)
    EstimateTokens(
        decisions []*decision.DecisionNode,
        queryResults map[AdapterType]any,
    ) int
}
```

---

### IM (群聊) 专用策略

#### StrategyIMConversation 完整对话流
```go
type StrategyIMConversation struct {
    LookbackMessages int // 回溯消息数，默认 50
}

func (s *StrategyIMConversation) Assemble(...) (*AssembledContext, error) {
    // 1. 获取当前消息所在的对话线索
    // 2. 按时间顺序回溯 N 条消息
    // 3. 保持消息的上下文连续性
    // 4. 优先保留回复链
}
```

**适用场景**：决策是多轮对话的结果，需要上下文

#### StrategyIMThreadView 线索视图
```go
type StrategyIMThreadView struct {
    IncludeReplies bool  // 包含回复
    IncludeRoot bool     // 包含根消息
}

func (s *StrategyIMThreadView) Assemble(...) (*AssembledContext, error) {
    // 1. 如果是回复消息，回溯到根消息
    // 2. 构建完整的回复树
    // 3. 只展示相关线索
}
```

**适用场景**：决策在回复线程中产生

#### StrategyIMTopicRelevant 话题相关
```go
type StrategyIMTopicRelevant struct {
    TopicKeywords []string
}

func (s *StrategyIMTopicRelevant) Assemble(...) (*AssembledContext, error) {
    // 1. 提取当前消息的关键词
    // 2. 搜索历史中相关话题的消息
    // 3. 按相关度排序
}
```

**适用场景**：决策与特定话题相关

---

### Doc (文档) 专用策略

#### StrategyDocFullContent 完整内容
```go
type StrategyDocFullContent struct {
    MaxSections int // 最大章节数
}

func (s *StrategyDocFullContent) Assemble(...) (*AssembledContext, error) {
    // 1. 获取完整文档 Markdown
    // 2. 按标题分段
    // 3. 智能截断保留重要部分
}
```

**适用场景**：决策遍布整个文档

#### StrategyDocDiffFocused 差异聚焦
```go
type StrategyDocDiffFocused struct {
    ShowContext int // 前后上下文行数
}

func (s *StrategyDocDiffFocused) Assemble(...) (*AssembledContext, error) {
    // 1. 获取上次保存的版本
    // 2. 计算 diff
    // 3. 聚焦变化部分 + 上下文
}
```

**适用场景**：决策在最新更新中

#### StrategyDocCommentPriority 评论优先
```go
type StrategyDocCommentPriority struct {
    IncludeDecisionComments bool // 包含决策相关评论
}

func (s *StrategyDocCommentPriority) Assemble(...) (*AssembledContext, error) {
    // 1. 先放入文档评论（含审批）
    // 2. 再放入相关文档段落
}
```

**适用场景**：决策在评论中达成

---

### Wiki (知识库) 专用策略

#### StrategyWikiTreeContext 树结构上下文
```go
type StrategyWikiTreeContext struct {
    IncludeParent bool
    IncludeSiblings bool
    IncludeChildren bool
}

func (s *StrategyWikiTreeContext) Assemble(...) (*AssembledContext, error) {
    // 1. 构建节点树路径
    // 2. 包含父节点、兄弟节点、子节点的摘要
    // 3. 突出当前节点内容
}
```

**适用场景**：知识库节点的层级关系很重要

#### StrategyWikiNodeFocused 节点聚焦
```go
type StrategyWikiNodeFocused struct {
    ExpandDepth int // 展开深度
}

func (s *StrategyWikiNodeFocused) Assemble(...) (*AssembledContext, error) {
    // 1. 深度聚焦当前节点内容
    // 2. 只包含直接相关的子节点
}
```

**适用场景**：决策只在单个节点内

#### StrategyWikiRecentUpdated 最近更新优先
```go
type StrategyWikiRecentUpdated struct {
    RecentHours int // 最近 N 小时
}

func (s *StrategyWikiRecentUpdated) Assemble(...) (*AssembledContext, error) {
    // 1. 查看同一空间内最近更新的节点
    // 2. 构建变更时间线
}
```

**适用场景**：跨节点的项目决策

---

### ParallelContextAssembler 并行组装器

```go
type ParallelContextAssembler struct {
    strategies map[AdapterType][]ContextStrategy
    llmAgent   *llm.MemoryAgent // 用于轻量 lookahead
    memoryGraph *core.MemoryGraph
}

// 主入口函数
func (a *ParallelContextAssembler) AssembleParallel(
    signal *StateChangeSignal,
    queryResults map[AdapterType]any,
    existingDecisions []*decision.DecisionNode,
    budget int,
) (*AssembledContext, []ContextVariant, error) {
    // 1. 获取适配器特定的策略集
    adapterType := signal.AdapterType
    strategies := a.getStrategiesForAdapter(adapterType)

    // 2. 并行生成所有策略变体
    variants := a.generateVariantsParallel(
        signal, queryResults, existingDecisions, budget, strategies)

    // 3. 轻量 lookahead 评估
    variants = a.evaluateVariantsLookahead(variants, signal)

    // 4. 选择最佳上下文
    bestVariant := a.selectBestVariant(variants)

    return bestVariant.Context, variants, nil
}
```

---

### Lookahead 轻量评估机制

#### 如何评估"决策提取友好度"

不做完整的 30s LLM 调用，做**轻量提示评估**：

```go
func (a *ParallelContextAssembler) evaluateVariant(
    variant ContextVariant,
    signal *StateChangeSignal,
) float64 {
    // 评估维度：

    // 1. 上下文密度
    densityScore := a.calculateDensityScore(variant.Context)

    // 2. 话题连贯性
    coherenceScore := a.calculateCoherenceScore(variant.Context, signal)

    // 3. 信号匹配度
    matchScore := a.calculateSignalMatchScore(variant.Context, signal)

    // 4. 轻量启发式预提取（可选）
    if a.llmAgent.IsAvailable() {
        previewScore := a.lightweightExtractPreview(variant.Context, signal)
        return (densityScore + coherenceScore + matchScore + previewScore) / 4
    }

    return (densityScore + coherenceScore + matchScore) / 3
}

// 轻量预提取：用更小的提示模板，更短的超时（5s）
func (a *ParallelContextAssembler) lightweightExtractPreview(...) float64 {
    prompt := a.buildLightweightPrompt(ctx, signal)
    // 用更低的 max tokens，更短的 timeout
    // 只返回置信度作为分数
}
```

---

## 集成到当前 Pipeline

### 修改位置

| 文件 | 修改内容 |
|------|---------|
| `internal/signal/context.go` | 添加新的 ParallelContextAssembler |
| `internal/signal/worker.go` | 在 processJob 中调用 ParallelContextAssembler |
| `internal/signal/engine.go` | 更新 SignalActivationEngine 用新的组装器 |

### 集成方式

在 `WorkerPool.processIMJob()` 中：

```go
// 旧代码：单一策略
ctx := &AssembledContext
if engine.assembler != nil {
    ctx = engine.assembler.Assemble(signal, queryResults, existingDecisions, budget)
}

// 新代码：并行策略
if engine.parallelAssembler != nil {
    bestCtx, allVariants, err := engine.parallelAssembler.AssembleParallel(
        signal, queryResults, existingDecisions, budget)
    // 可选：把变体信息也存下来用于分析
    logDecisionVariants(allVariants)
}
```

---

## 可观测性与调试

### VariantLogger 记录变体表现

```go
type VariantLogger struct {
    logs []VariantLogEntry
}

type VariantLogEntry struct {
    SignalContent string
    SelectedStrategy string
    AllVariants []VariantScoreLog
    ActualDecisionExtracted bool
    ActualDecision *DecisionExtract
    Timestamp time.Time
}

// 记录用于分析哪个策略在哪些场景表现最好
func (l *VariantLogger) Log(...)
```

### 指标输出

- 各策略被选中的频率（按适配器类型分组）
- 各策略的平均提取成功率
- Lookahead 预测 vs 实际结果的吻合度

---

## 迁移路径（渐进式，不破坏现有代码）

### Phase 1: 基础搭建（只添加，不修改）

```
新建文件：
├── internal/signal/context_parallel.go    # 新并行组装器
├── internal/signal/context_strategies_im.go # IM 策略
├── internal/signal/context_strategies_doc.go # Doc 策略
├── internal/signal/context_strategies_wiki.go # Wiki 策略
└── internal/signal/context_evaluate.go     # 评估逻辑
```

### Phase 2: 集成但默认关闭

在 `config/openclaw.yaml` 中添加：

```yaml
context:
  use_parallel: false  # 默认用旧的单一策略
  im_strategies:
    - conversation
    - recent_messages
    - thread_view
    - topic_relevant
  doc_strategies:
    - full_content
    - diff_focused
    - structure
    - comment_priority
  wiki_strategies:
    - tree_context
    - node_focused
    - recent_updated
    - path_relevance
  lookahead_enabled: true
  lookahead_timeout_seconds: 5
```

### Phase 3: A/B 测试与验证

- 同时运行两种模式，记录表现
- 按适配器类型分析效果
- 如果更好，把默认设为 `use_parallel: true`

---

## 预期收益

| 维度 | 预期提升 |
|------|---------|
| IM 决策提取准确率 | +20-30%（对话上下文更连贯） |
| Doc 决策提取准确率 | +15-25%（文档内容更聚焦） |
| Wiki 决策提取准确率 | +20-30%（层级结构更清晰） |
| 上下文 token 利用率 | +10-20%（更智能的筛选） |
| 可观测性 | 显著提升（知道为什么选这个上下文） |
| 扩展性 | 轻松添加新策略（只实现 Strategy 接口） |

---

## 相关文件与参考

- 当前代码：`internal/signal/context.go`
- 论文：`docs/2603.27490v1_AgentSwing.pdf`
- 相关组件：`internal/llm/agent.go`, `internal/core/memory_graph.go`
- 各适配器实现：
  - `internal/lark-adapter/lark_im.go`
  - `internal/lark-adapter/lark_doc.go`
  - `internal/lark-adapter/lark_wiki.go`
