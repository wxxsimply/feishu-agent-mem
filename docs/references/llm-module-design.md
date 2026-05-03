# LLM 模块设计（多 Agent 编排版）

> 记忆系统 `internal/llm` 模块完整设计。采用多 Agent 编排模式，与 OpenClaw 集成，遵循 `docs/agent 集成设计价值观.md` 中的所有设计原则。

---

## 一、设计原则

本模块严格遵循以下设计原则：

### 1.1 专用工具原则（在万能工具里写「别用我」）

- 专用工具价值：可控边界
- 精确控制输出格式
- 文件缓存去重
- Token 预算检查

### 1.2 两级工具加载

```
模型需要工具 -> 调用 Tool Search 传入关键词 -> 匹配 searchHint + 工具名 + 说明 -> 返回完整定义 -> 模型正常调用
```

### 1.3 Agent 拆分

- Explore Agent：极端只读隔离
- Verification Agent：主动构造出错场景，用实际行动证明
- Plan Agent：工作流规划
- Guide Agent：用户引导

### 1.4 错误处理与自我纠正

```
工具调用 -> 返回错误 -> 错误处理反馈 LLM -> LLM 自我纠正 -> 修正后重试
```

### 1.5 检查点机制

```
Step1(已保存) -> Step2(已保存) -> Step3(检查点) -> Step4(已保存) -> Step5(失败)
```

### 1.6 兜底策略

1. 最大循环次数：防止死循环
2. 最大 Token 预算：控制成本
3. 超时熔断：避免无限等待
4. 人工介入：最终兜底

---

## 二、模块目录结构

```
internal/llm/
├── agent.go              # Agent 编排器（入口）
├── orchestrator.go       # 工作流编排器
├── agents/               # 多场景 Agent
│   ├── extractor_agent.go      # 决策提取 Agent
│   ├── classifier_agent.go     # 议题分类 Agent
│   ├── crosstopic_agent.go     # 跨议题检测 Agent
│   └── conflict_agent.go       # 冲突评估 Agent
├── tools/                # 专用工具集
│   ├── mcp_tools.go            # MCP 工具封装
│   ├── search_tool.go          # 搜索工具
│   └── parse_tool.go           # JSON 解析工具
├── prompts/              # 提示词管理
│   ├── manager.go              # Prompt Manager
│   ├── templates.go            # 模板定义
│   └── dynamic.go              # 动态段生成
├── error_handling/       # 错误处理与自我纠正
│   ├── checker.go              # 检查点机制
│   ├── recovery.go             # 失败恢复
│   ├── circuit_breaker.go      # 超时熔断
│   └── guardrails.go           # 护栏检查
├── budget/               # Token 预算控制
│   ├── tracker.go              # Token 追踪器
│   ├── limits.go               # 预算限制
│   └── cost_estimator.go       # 成本估算器
├── types.go              # 类型定义
├── fallback.go           # 降级策略
└── options.go            # 调用选项
```

---

## 三、核心组件设计

### 3.1 Agent 编排器（agent.go）

```go
// internal/llm/agent.go

package llm

import (
    "feishu-mem/internal/decision"
    "feishu-mem/internal/signal"
)

// MemoryAgent 记忆系统专用 Agent（向 OpenClaw 暴露的接口）
type MemoryAgent struct {
    orchestrator *Orchestrator
    tools        *ToolRegistry
    budget       *BudgetTracker
}

// NewMemoryAgent 创建 MemoryAgent
func NewMemoryAgent(tools *ToolRegistry, budget *BudgetTracker) *MemoryAgent {
    return &MemoryAgent{
        orchestrator: NewOrchestrator(tools, budget),
        tools:        tools,
        budget:       budget,
    }
}

// ========== 核心工作流 ==========

// ProcessSignal 处理信号（主入口）
func (a *MemoryAgent) ProcessSignal(sig *signal.StateChangeSignal) (*DecisionResult, error) {
    return a.orchestrator.RunSignalWorkflow(sig)
}

// ExtractDecision 提取决策
func (a *MemoryAgent) ExtractDecision(content string, topics []string) (*ExtractionResult, error) {
    return a.orchestrator.RunExtractionWorkflow(content, topics)
}

// ClassifyTopic 分类议题
func (a *MemoryAgent) ClassifyTopic(decision string) (*ClassificationResult, error) {
    return a.orchestrator.RunClassificationWorkflow(decision)
}

// DetectCrossTopic 检测跨议题
func (a *MemoryAgent) DetectCrossTopic(node *decision.DecisionNode) (*CrossTopicResult, error) {
    return a.orchestrator.RunCrossTopicWorkflow(node)
}

// ResolveConflict 解决冲突
func (a *MemoryAgent) ResolveConflict(a, b *decision.DecisionNode) (*ConflictResult, error) {
    return a.orchestrator.RunConflictWorkflow(a, b)
}

// IsAvailable 检查 LLM 是否可用
func (a *MemoryAgent) IsAvailable() bool {
    return a.tools.HasLLMBackend()
}
```

### 3.2 工作流编排器（orchestrator.go）

```go
// internal/llm/orchestrator.go

package llm

// Orchestrator 工作流编排器
type Orchestrator struct {
    tools       *ToolRegistry
    budget      *BudgetTracker
    agents      map[string]Agent
    reflection  *SelfReflection
}

// Agent 接口
type Agent interface {
    Name() string
    Run(ctx *Context) (*AgentResult, error)
    Checkpoint() *Checkpoint
    Rollback(checkpoint *Checkpoint) error
}

// Context Agent 执行上下文
type Context struct {
    Signal      *signal.StateChangeSignal
    Content     string
    Topics      []string
    Node        *decision.DecisionNode
    OtherNode   *decision.DecisionNode
    Budget      *BudgetTracker
    History     []string
}

// AgentResult Agent 执行结果
type AgentResult struct {
    Success    bool
    Data       interface{}
    TokensUsed int
    Checkpoint *Checkpoint
    Error      error
}

// ========== 工作流 ==========

// RunSignalWorkflow 信号处理工作流
func (o *Orchestrator) RunSignalWorkflow(sig *signal.StateChangeSignal) (*DecisionResult, error) {
    ctx := &Context{
        Signal: sig,
        Budget: o.budget,
    }

    // Step 1: 提取决策
    extractResult, err := o.agents["extractor"].Run(ctx)
    if err != nil {
        return nil, err
    }

    // Step 2: 分类议题
    ctx.Content = extractResult.Data.(string)
    classResult, err := o.agents["classifier"].Run(ctx)
    if err != nil {
        return nil, err
    }

    // Step 3: 检测跨议题
    // ...

    return &DecisionResult{}, nil
}

// RunExtractionWorkflow 提取工作流
func (o *Orchestrator) RunExtractionWorkflow(content string, topics []string) (*ExtractionResult, error) {
    ctx := &Context{
        Content: content,
        Topics:  topics,
        Budget:  o.budget,
    }

    result, err := o.agents["extractor"].Run(ctx)
    if err != nil {
        return nil, err
    }

    return result.Data.(*ExtractionResult), nil
}

// ========== 自我反思机制 ==========

// SelfReflection 自我反思
type SelfReflection struct {
    reflectionInterval int  // 每 N 步暂停反思
    targetAlignment    bool // 对齐原始目标
}

// Reflect 执行反思
func (o *Orchestrator) Reflect(ctx *Context, result *AgentResult) bool {
    // 检查是否符合原始目标
    // 检查是否偏离方向
    // 返回是否需要回滚
    return false
}
```

### 3.3 场景专用 Agent（agents/）

#### 3.3.1 决策提取 Agent

```go
// internal/llm/agents/extractor_agent.go

package agents

import (
    "feishu-mem/internal/decision"
)

// ExtractorAgent 决策提取专用 Agent
// 特点：极端只读隔离、主动构造出错场景
type ExtractorAgent struct {
    name        string
    tools       []string        // 可用工具列表
    budget      *BudgetTracker
    checkpoint  *Checkpoint     // 检查点机制
    fallback    *Fallback       // 降级策略
}

func NewExtractorAgent(tools *ToolRegistry, budget *BudgetTracker) *ExtractorAgent {
    return &ExtractorAgent{
        name:   "extractor",
        tools:  []string{"mcp.search", "parse.json"},
        budget: budget,
    }
}

func (a *ExtractorAgent) Name() string { return a.name }

// Run 执行提取工作流（含自我反思）
func (a *ExtractorAgent) Run(ctx *Context) (*AgentResult, error) {
    // Step 1: 接收内容
    content := ctx.Content

    // Step 2: 快速路径 - 关键词匹配（降级策略）
    quickResult := a.tryKeywordMatch(content)
    if quickResult != nil {
        return &AgentResult{
            Success: true,
            Data:    quickResult,
        }, nil
    }

    // Step 3: 慢路径 - LLM 提取
    // 3.1: 保存检查点
    checkpoint := a.Checkpoint()

    // 3.2: 构建提示词
    prompt := a.buildPrompt(content, ctx.Topics)

    // 3.3: 调用 LLM
    llmResult, err := a.callLLM(prompt)
    if err != nil {
        // 3.4: 失败恢复 - 回滚到检查点
        a.Rollback(checkpoint)
        // 3.5: 使用降级策略
        return a.fallback.Execute(content), nil
    }

    // Step 4: 验证输出格式
    if !a.validateOutput(llmResult) {
        // 主动构造出错场景验证
        return a.verifyOutput(llmResult)
    }

    // Step 5: 保存最终检查点
    a.checkpoint = a.Checkpoint()

    return &AgentResult{
        Success:    true,
        Data:       llmResult,
        Checkpoint: a.checkpoint,
    }, nil
}

// Checkpoint 保存检查点
func (a *ExtractorAgent) Checkpoint() *Checkpoint {
    return &Checkpoint{
        StepID:    "extractor_step",
        State:     map[string]interface{}{"status": "in_progress"},
        Timestamp: time.Now(),
    }
}

// Rollback 回滚到检查点
func (a *ExtractorAgent) Rollback(checkpoint *Checkpoint) error {
    // 恢复状态
    return nil
}

// ========== 专用工具 ==========

func (a *ExtractorAgent) tryKeywordMatch(content string) *ExtractionResult {
    // 关键词匹配（快速路径）
    return nil
}

func (a *ExtractorAgent) buildPrompt(content string, topics []string) string {
    // 构建提示词（静态段 + 动态段）
    return ""
}

func (a *ExtractorAgent) callLLM(prompt string) (*ExtractionResult, error) {
    // 调用 LLM
    return nil, nil
}

func (a *ExtractorAgent) validateOutput(result *ExtractionResult) bool {
    // 验证输出格式
    return true
}

func (a *ExtractorAgent) verifyOutput(result *ExtractionResult) (*AgentResult, error) {
    // 主动构造出错场景验证
    return &AgentResult{Success: true, Data: result}, nil
}
```

#### 3.3.2 议题分类 Agent

```go
// internal/llm/agents/classifier_agent.go

package agents

// ClassifierAgent 议题分类专用 Agent
type ClassifierAgent struct {
    name       string
    tools      []string
    budget     *BudgetTracker
    checkpoint *Checkpoint
}

func NewClassifierAgent(tools *ToolRegistry, budget *BudgetTracker) *ClassifierAgent {
    return &ClassifierAgent{
        name:  "classifier",
        tools: []string{"mcp.topic", "parse.json"},
        budget: budget,
    }
}

func (a *ClassifierAgent) Name() string { return a.name }

func (a *ClassifierAgent) Run(ctx *Context) (*AgentResult, error) {
    // Step 1: 获取候选议题列表
    topics := a.getCandidateTopics()

    // Step 2: 构建提示词
    prompt := a.buildPrompt(ctx.Content, topics)

    // Step 3: 调用 LLM
    result, err := a.callLLM(prompt)
    if err != nil {
        return nil, err
    }

    // Step 4: 验证结果
    if result.Confidence < 0.5 {
        return &AgentResult{
            Success: false,
            Data:    nil,
        }, nil
    }

    return &AgentResult{
        Success: true,
        Data:    result,
    }, nil
}
```

#### 3.3.3 跨议题检测 Agent

```go
// internal/llm/agents/crosstopic_agent.go

package agents

// CrossTopicAgent 跨议题检测专用 Agent
type CrossTopicAgent struct {
    name       string
    tools      []string
    budget     *BudgetTracker
    checkpoint *Checkpoint
}

func NewCrossTopicAgent(tools *ToolRegistry, budget *BudgetTracker) *CrossTopicAgent {
    return &CrossTopicAgent{
        name:  "crosstopic",
        tools: []string{"mcp.topic", "parse.json"},
        budget: budget,
    }
}

func (a *CrossTopicAgent) Name() string { return a.name }

func (a *CrossTopicAgent) Run(ctx *Context) (*AgentResult, error) {
    // Step 1: 获取候选议题（排除自身）
    candidates := a.getCandidateTopics(ctx.Node.Topic)

    // Step 2: 判定维度分析
    dimensions := a.analyzeDimensions(ctx.Node)

    // Step 3: 仅当 2 个以上维度指向 Cross 时才标记
    if len(dimensions) < 2 {
        return &AgentResult{
            Success: true,
            Data:    &CrossTopicResult{IsCrossTopic: false},
        }, nil
    }

    // Step 4: LLM 判定
    result, err := a.callLLM(ctx.Node, candidates)
    if err != nil {
        return nil, err
    }

    return &AgentResult{
        Success: true,
        Data:    result,
    }, nil
}
```

#### 3.3.4 冲突评估 Agent

```go
// internal/llm/agents/conflict_agent.go

package agents

// ConflictAgent 冲突评估专用 Agent
type ConflictAgent struct {
    name       string
    tools      []string
    budget     *BudgetTracker
    checkpoint *Checkpoint
}

func NewConflictAgent(tools *ToolRegistry, budget *BudgetTracker) *ConflictAgent {
    return &ConflictAgent{
        name:  "conflict",
        tools: []string{"mcp.decision", "parse.json"},
        budget: budget,
    }
}

func (a *ConflictAgent) Name() string { return a.name }

func (a *ConflictAgent) Run(ctx *Context) (*AgentResult, error) {
    // Step 1: 权重比较
    newWeight := a.impactWeight(ctx.Node.ImpactLevel)
    oldWeight := a.impactWeight(ctx.OtherNode.ImpactLevel)

    // Step 2: LLM 语义矛盾评估
    score, err := a.callLLM(ctx.Node, ctx.OtherNode)
    if err != nil {
        return nil, err
    }

    // Step 3: 决策（对齐 decision-tree.md §5.3）
    var result *ConflictResult
    switch {
    case score < 0.3:
        result = &ConflictResult{Action: "no_conflict"}
    case score < 0.6:
        result = &ConflictResult{Action: "relates"}
    case newWeight > oldWeight:
        result = &ConflictResult{Action: "supersedes"}
    case newWeight == oldWeight:
        result = &ConflictResult{Action: "conflict", NeedsUser: true}
    default:
        result = &ConflictResult{Action: "blocked"}
    }

    return &AgentResult{
        Success: true,
        Data:    result,
    }, nil
}
```

---

### 3.4 工具设计（tools/）

#### 3.4.1 工具注册表

```go
// internal/llm/tools/mcp_tools.go

package tools

// ToolRegistry 工具注册表（两级加载）
type ToolRegistry struct {
    // 第一级：轻量描述（searchHint）
    hints map[string]*ToolHint

    // 第二级：完整定义（按需加载）
    fullTools map[string]*ToolDefinition
}

// ToolHint 轻量描述
type ToolHint struct {
    Name        string
    SearchHint  string  // 搜索关键词
    Description string  // 一句话描述
}

// ToolDefinition 完整定义
type ToolDefinition struct {
    Name          string
    Description   string
    InputSchema   map[string]interface{}
    OutputBudget  int  // Token 预算
    Handler       func(input interface{}) (interface{}, error)
}

// NewToolRegistry 创建工具注册表
func NewToolRegistry() *ToolRegistry {
    r := &ToolRegistry{
        hints:     make(map[string]*ToolHint),
        fullTools: make(map[string]*ToolDefinition),
    }
    r.registerAllTools()
    return r
}

// registerAllTools 注册所有工具
func (r *ToolRegistry) registerAllTools() {
    // MCP 工具
    r.RegisterMCPTools()

    // 专用工具
    r.RegisterSearchTool()
    r.RegisterParseTool()
}

// RegisterMCPTools 注册 MCP 工具（两级加载）
func (r *ToolRegistry) RegisterMCPTools() {
    // 第一级：轻量描述
    r.hints["mcp.search"] = &ToolHint{
        Name:        "mcp.search",
        SearchHint:  "search, 搜索, 决策",
        Description: "搜索记忆系统中的决策记录",
    }

    r.hints["mcp.topic"] = &ToolHint{
        Name:        "mcp.topic",
        SearchHint:  "topic, 议题, 分类",
        Description: "查看某议题的决策图谱",
    }

    r.hints["mcp.decision"] = &ToolHint{
        Name:        "mcp.decision",
        SearchHint:  "decision, 决策, 详情",
        Description: "查看某决策详情",
    }

    // 第二级：完整定义（按需加载）
    // ...
}

// SearchTool 搜索工具
type SearchTool struct {
    outputBudget int  // 搜索结果 ≤ 2w字符
    dedup        bool // 文件缓存去重
}

// RegisterSearchTool 注册搜索工具
func (r *ToolRegistry) RegisterSearchTool() {
    r.hints["search"] = &ToolHint{
        Name:        "search",
        SearchHint:  "search, 查找, 搜索",
        Description: "搜索决策记录（专用工具，预算 2w 字符）",
    }

    r.fullTools["search"] = &ToolDefinition{
        Name:         "search",
        Description:  "搜索决策记录",
        OutputBudget: 20000, // ≤ 2w字符
        Handler:      r.handleSearch,
    }
}

// handleSearch 处理搜索请求
func (r *ToolRegistry) handleSearch(input interface{}) (interface{}, error) {
    // 实现搜索逻辑
    return nil, nil
}
```

#### 3.4.2 搜索工具

```go
// internal/llm/tools/search_tool.go

package tools

// SearchTool 专用搜索工具
// 价值：可控边界、精确输出格式、Token 预算检查
type SearchTool struct {
    outputBudget int      // 搜索结果 ≤ 2w字符
    dedup        bool     // 文件缓存去重
    cache        *Cache   // 缓存
}

func NewSearchTool() *SearchTool {
    return &SearchTool{
        outputBudget: 20000,
        dedup:        true,
        cache:        NewCache(),
    }
}

// Search 执行搜索
func (t *SearchTool) Search(query string, opts *SearchOptions) (*SearchResult, error) {
    // 1. 检查缓存
    if cached := t.cache.Get(query); cached != nil {
        return cached, nil
    }

    // 2. 执行搜索
    results, err := t.doSearch(query, opts)
    if err != nil {
        return nil, err
    }

    // 3. 去重
    if t.dedup {
        results = t.deduplicate(results)
    }

    // 4. 截断到预算
    results = t.truncateToBudget(results, t.outputBudget)

    // 5. 缓存结果
    t.cache.Set(query, results)

    return results, nil
}
```

#### 3.4.3 JSON 解析工具

```go
// internal/llm/tools/parse_tool.go

package tools

// ParseTool JSON 解析工具（容错处理）
type ParseTool struct{}

func NewParseTool() *ParseTool {
    return &ParseTool{}
}

// ParseJSON 解析 JSON（容错）
func (t *ParseTool) ParseJSON(input string) (map[string]interface{}, error) {
    // 1. 尝试直接解析
    var result map[string]interface{}
    if err := json.Unmarshal([]byte(input), &result); err == nil {
        return result, nil
    }

    // 2. 尝试提取 JSON 块
    jsonBlock := t.extractJSONBlock(input)
    if jsonBlock != "" {
        if err := json.Unmarshal([]byte(jsonBlock), &result); err == nil {
            return result, nil
        }
    }

    // 3. 尝试修复常见格式问题
    fixed := t.fixCommonIssues(input)
    if err := json.Unmarshal([]byte(fixed), &result); err == nil {
        return result, nil
    }

    return nil, fmt.Errorf("JSON parse failed: %s", input)
}

// extractJSONBlock 提取 JSON 块
func (t *ParseTool) extractJSONBlock(input string) string {
    // 从 Markdown 代码块中提取 JSON
    // ...
    return ""
}

// fixCommonIssues 修复常见格式问题
func (t *ParseTool) fixCommonIssues(input string) string {
    // 修复：单引号 -> 双引号
    // 修复：缺少引号的键
    // ...
    return input
}
```

---

### 3.5 提示词管理（prompts/）

```go
// internal/llm/prompts/manager.go

package prompts

// PromptManager 提示词管理器
// 设计：静态段 + 动态段，提高缓存命中率
type PromptManager struct {
    templates map[string]*PromptTemplate
}

// PromptTemplate 提示词模板
type PromptTemplate struct {
    Name        string
    Static      string  // 静态段（通用规则，跨用户共享缓存）
    Dynamic     func(ctx map[string]interface{}) string  // 动态段（用户特有信息）
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
        MaxTokens:   2000,
        Temperature: 0.3,
    }

    // 场景 2: 议题分类
    pm.templates["classification"] = &PromptTemplate{
        Name:        "classification",
        Static:      classificationStaticPrompt,
        Dynamic:     classificationDynamicBuilder,
        MaxTokens:   1000,
        Temperature: 0.2,
    }

    // 场景 3: 跨议题检测
    pm.templates["crosstopic"] = &PromptTemplate{
        Name:        "crosstopic",
        Static:      crosstopicStaticPrompt,
        Dynamic:     crosstopicDynamicBuilder,
        MaxTokens:   1500,
        Temperature: 0.3,
    }

    // 场景 4: 冲突评估
    pm.templates["conflict"] = &PromptTemplate{
        Name:        "conflict",
        Static:      conflictStaticPrompt,
        Dynamic:     conflictDynamicBuilder,
        MaxTokens:   1500,
        Temperature: 0.2,
    }
}

// BuildPrompt 构建完整提示词
func (pm *PromptManager) BuildPrompt(name string, dynamicData map[string]interface{}) (string, error) {
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

// ========== 静态段定义（对齐 docs/prompts.md） ==========

var extractionStaticPrompt = `
# 系统提示词：决策提取器

## 角色
你是一个项目决策提取专家。从飞书群聊消息、会议纪要、文档内容中识别和提取决策性信息。

## 决策识别信号
- 中文: "决定"、"确认"、"结论"、"通过"、"定下来"、"就这么办"、"不再讨论"、"最终方案"
- 英文: "approve"、"LGTM"、"decided"、"confirmed"、"agreed"
- Pin 消息自动视为高价值锚点
- 会议纪要中的"待办"/"Action Items"自动提取
- 文档评论中的 "/approve" 或"同意"视为审批信号

## 输出格式
仅当 confidence >= 0.6 时输出，否则返回 {"has_decision": false}。
不要编造原文没有的内容。
`
```

---

### 3.6 错误处理（error_handling/）

```go
// internal/llm/error_handling/checker.go

package error_handling

import "time"

// Checkpoint 检查点机制
type Checkpoint struct {
    StepID    string
    State     map[string]interface{}
    Timestamp time.Time
}

// CheckpointManager 检查点管理器
type CheckpointManager struct {
    checkpoints map[string]*Checkpoint
}

func NewCheckpointManager() *CheckpointManager {
    return &CheckpointManager{
        checkpoints: make(map[string]*Checkpoint),
    }
}

// Save 保存检查点
func (m *CheckpointManager) Save(stepID string, state map[string]interface{}) {
    m.checkpoints[stepID] = &Checkpoint{
        StepID:    stepID,
        State:     state,
        Timestamp: time.Now(),
    }
}

// Load 加载检查点
func (m *CheckpointManager) Load(stepID string) *Checkpoint {
    return m.checkpoints[stepID]
}

// Rollback 回滚到检查点
func (m *CheckpointManager) Rollback(stepID string) error {
    checkpoint := m.Load(stepID)
    if checkpoint == nil {
        return fmt.Errorf("checkpoint not found: %s", stepID)
    }
    // 恢复状态
    return nil
}
```

```go
// internal/llm/error_handling/recovery.go

package error_handling

// Recovery 失败恢复机制
type Recovery struct {
    maxRetries    int           // 最大循环次数
    maxTokens     int           // 最大Token预算
    timeout       time.Duration // 超时熔断
    humanFallback bool          // 人工介入
}

// RecoveryStrategy 恢复策略
type RecoveryStrategy struct {
    strategy string // "retry" | "rollback" | "fallback" | "human"
}

// Recover 执行恢复
func (r *Recovery) Recover(err error, checkpoint *Checkpoint) *RecoveryStrategy {
    // 1. 失败检测
    failureType := r.classifyFailure(err)

    // 2. 显性/隐性分类
    switch failureType {
    case "explicit":
        // 显性失败：直接重试
        return &RecoveryStrategy{strategy: "retry"}
    case "implicit_loop":
        // 隐性失败：死循环
        return &RecoveryStrategy{strategy: "rollback"}
    case "implicit_drift":
        // 隐性失败：方向偏离
        return &RecoveryStrategy{strategy: "rollback"}
    case "implicit_overflow":
        // 隐性失败：上下文溢出
        return &RecoveryStrategy{strategy: "fallback"}
    }

    // 3. 最终兜底
    return &RecoveryStrategy{strategy: "human"}
}
```

```go
// internal/llm/error_handling/circuit_breaker.go

package error_handling

// CircuitBreaker 超时熔断器
type CircuitBreaker struct {
    timeout    time.Duration
    maxRetries int
    state      string // "closed" | "open" | "half-open"
}

func NewCircuitBreaker(timeout time.Duration, maxRetries int) *CircuitBreaker {
    return &CircuitBreaker{
        timeout:    timeout,
        maxRetries: maxRetries,
        state:      "closed",
    }
}

// Execute 执行操作（带熔断）
func (cb *CircuitBreaker) Execute(operation func() error) error {
    if cb.state == "open" {
        return fmt.Errorf("circuit breaker is open")
    }

    // 带超时执行
    done := make(chan error, 1)
    go func() {
        done <- operation()
    }()

    select {
    case err := <-done:
        if err != nil {
            cb.recordFailure()
            return err
        }
        cb.recordSuccess()
        return nil
    case <-time.After(cb.timeout):
        cb.recordTimeout()
        return fmt.Errorf("operation timeout")
    }
}
```

```go
// internal/llm/error_handling/guardrails.go

package error_handling

// Guardrails 护栏检查
type Guardrails struct {
    maxLoops  int  // 最大循环次数
    maxTokens int  // 最大Token预算
}

// Check 检查护栏
func (g *Guardrails) Check(ctx *Context) error {
    // 检查循环次数
    if ctx.LoopCount > g.maxLoops {
        return fmt.Errorf("max loops exceeded: %d", g.maxLoops)
    }

    // 检查Token预算
    if ctx.TokensUsed > g.maxTokens {
        return fmt.Errorf("max tokens exceeded: %d", g.maxTokens)
    }

    return nil
}
```

---

### 3.7 Token 预算控制（budget/）

```go
// internal/llm/budget/tracker.go

package budget

// TokenBudget 各层预算限制
// 对齐 docs/agent 集成设计价值观.md
type TokenBudget struct {
    SearchTool    int // 搜索工具 ≤ 2w字符
    FileRead      int // 文件读取特殊处理
    TotalPerAgent int // 单个Agent预算
    MaxTotal      int // 全局预算上限
}

// DefaultBudget 默认预算
func DefaultBudget() TokenBudget {
    return TokenBudget{
        SearchTool:    20000, // 2w字符
        FileRead:      10000, // 特殊处理
        TotalPerAgent: 8000,  // 单个Agent
        MaxTotal:      50000, // 全局上限
    }
}

// BudgetTracker 预算追踪器
type BudgetTracker struct {
    used    int
    limits  TokenBudget
    history []string
}

func NewBudgetTracker(limits TokenBudget) *BudgetTracker {
    return &BudgetTracker{
        limits: limits,
    }
}

// CanUse 检查是否有足够的预算
func (bt *BudgetTracker) CanUse(tool string, estimatedTokens int) bool {
    return bt.used+estimatedTokens <= bt.limits.MaxTotal
}

// Consume 消耗预算
func (bt *BudgetTracker) Consume(tool string, tokens int) {
    bt.used += tokens
    bt.history = append(bt.history, tool)
}

// GetUsed 获取已使用预算
func (bt *BudgetTracker) GetUsed() int {
    return bt.used
}

// GetRemaining 获取剩余预算
func (bt *BudgetTracker) GetRemaining() int {
    return bt.limits.MaxTotal - bt.used
}
```

```go
// internal/llm/budget/cost_estimator.go

package budget

// CostEstimator 成本估算器
type CostEstimator struct {
    // 不同工具的成本估算
    toolCosts map[string]int
}

func NewCostEstimator() *CostEstimator {
    return &CostEstimator{
        toolCosts: map[string]int{
            "mcp.search":    500,
            "mcp.topic":     1000,
            "mcp.decision":  300,
            "mcp.timeline":  600,
            "mcp.conflict":  400,
            "mcp.signal":    300,
            "llm.call":      2000,
        },
    }
}

// Estimate 估算成本
func (ce *CostEstimator) Estimate(tool string) int {
    if cost, ok := ce.toolCosts[tool]; ok {
        return cost
    }
    return 0
}
```

---

### 3.8 类型定义（types.go）

```go
// internal/llm/types.go

package llm

import (
    "feishu-mem/internal/decision"
    "time"
)

// ========== 请求/响应类型 ==========

// ExtractionResult 决策提取结果
type ExtractionResult struct {
    HasDecision bool                `json:"has_decision"`
    Confidence  float64             `json:"confidence"`
    Decision    *DecisionExtract    `json:"decision,omitempty"`
    ExtractedFrom string            `json:"extracted_from"`
}

// DecisionExtract 提取的决策
type DecisionExtract struct {
    Title       string `json:"title"`
    Decision    string `json:"decision"`
    Rationale   string `json:"rationale"`
    Topic       string `json:"suggested_topic"`
    ImpactLevel string `json:"impact_level"`
    PhaseScope  string `json:"phase_scope"`
    Proposer    string `json:"proposer"`
    Executor    string `json:"executor"`
}

// ClassificationResult 议题分类结果
type ClassificationResult struct {
    Topic              string   `json:"topic"`
    Confidence         float64  `json:"confidence"`
    Reasoning          string   `json:"reasoning"`
    AlternativeTopics  []string `json:"alternative_topics,omitempty"`
}

// CrossTopicResult 跨议题检测结果
type CrossTopicResult struct {
    IsCrossTopic bool              `json:"is_cross_topic"`
    CrossTopicRefs []string        `json:"cross_topic_refs,omitempty"`
    Reasons      map[string]string `json:"reasons,omitempty"`
    Confidence   float64           `json:"confidence"`
}

// ConflictResult 冲突评估结果
type ConflictResult struct {
    ContradictionScore float64 `json:"contradiction_score"`
    ContradictionType  string  `json:"contradiction_type"`
    Description        string  `json:"description"`
    Suggestion         string  `json:"suggestion,omitempty"`
    Action             string  `json:"action"`
    NeedsUser          bool    `json:"needs_user"`
}

// DecisionResult 决策处理结果
type DecisionResult struct {
    Decision    *decision.DecisionNode
    Topic       *ClassificationResult
    CrossTopic  *CrossTopicResult
    Conflicts   []ConflictResult
    CreatedAt   time.Time
}

// ========== Agent 类型 ==========

// AgentConfig Agent 配置
type AgentConfig struct {
    MaxRetries     int
    Timeout        time.Duration
    MaxTokens      int
    Temperature    float64
}

// AgentState Agent 状态
type AgentState struct {
    Name        string
    Status      string // "idle" | "running" | "completed" | "failed"
    Checkpoint  *Checkpoint
    TokensUsed  int
    LoopCount   int
}
```

---

### 3.9 降级策略（fallback.go）

```go
// internal/llm/fallback.go

package llm

import (
    "strings"
)

// Fallback 降级策略（纯规则实现）
// 用途：LLM 不可用时使用
type Fallback struct {
    decisionKeywords []string
}

func NewFallback() *Fallback {
    return &Fallback{
        decisionKeywords: []string{
            "决定", "decided", "确认", "LGTM", "lgtm",
            "approve", "通过", "定下来", "结论", "最终方案",
        },
    }
}

// ExtractDecision 降级：关键词匹配提取决策
func (f *Fallback) ExtractDecision(content string) *ExtractionResult {
    result := &ExtractionResult{
        HasDecision:   false,
        Confidence:    0.0,
        ExtractedFrom: content,
    }

    for _, kw := range f.decisionKeywords {
        if strings.Contains(content, kw) {
            result.HasDecision = true
            result.Confidence = 0.7 // 降级置信度较低
            result.Decision = &DecisionExtract{
                Title:    "Auto-extracted decision",
                Decision: content,
                Proposer: "keyword_match",
            }
            break
        }
    }

    return result
}

// ClassifyTopic 降级：关键词匹配分类
func (f *Fallback) ClassifyTopic(content string, topics []string) *ClassificationResult {
    // 简单的关键词匹配
    for _, topic := range topics {
        if strings.Contains(content, topic) {
            return &ClassificationResult{
                Topic:      topic,
                Confidence: 0.6,
                Reasoning:  "keyword_match",
            }
        }
    }

    return &ClassificationResult{
        Topic:      "",
        Confidence: 0.0,
    }
}

// DetectCrossTopic 降级：简单规则
func (f *Fallback) DetectCrossTopic(node *decision.DecisionNode) *CrossTopicResult {
    // 基于 impact_level 的简单规则
    if node.ImpactLevel == "critical" || node.ImpactLevel == "major" {
        return &CrossTopicResult{
            IsCrossTopic: true,
            Confidence:   0.5,
        }
    }

    return &CrossTopicResult{
        IsCrossTopic: false,
        Confidence:   0.5,
    }
}
```

---

### 3.10 调用选项（options.go）

```go
// internal/llm/options.go

package llm

import "time"

// CallOptions 调用选项
type CallOptions struct {
    Timeout      time.Duration
    MaxRetries   int
    Temperature  float64
    MaxTokens    int
    Budget       *BudgetTracker
}

// DefaultCallOptions 默认调用选项
func DefaultCallOptions() *CallOptions {
    return &CallOptions{
        Timeout:     30 * time.Second,
        MaxRetries:  3,
        Temperature: 0.3,
        MaxTokens:   2000,
    }
}

// WithTimeout 设置超时
func WithTimeout(timeout time.Duration) func(*CallOptions) {
    return func(opts *CallOptions) {
        opts.Timeout = timeout
    }
}

// WithMaxRetries 设置最大重试次数
func WithMaxRetries(retries int) func(*CallOptions) {
    return func(opts *CallOptions) {
        opts.MaxRetries = retries
    }
}

// WithTemperature 设置温度
func WithTemperature(temp float64) func(*CallOptions) {
    return func(opts *CallOptions) {
        opts.Temperature = temp
    }
}

// WithBudget 设置预算
func WithBudget(budget *BudgetTracker) func(*CallOptions) {
    return func(opts *CallOptions) {
        opts.Budget = budget
    }
}
```

---

## 四、与 OpenClaw 的集成

### 4.1 MCP 工具暴露

记忆系统通过 MCP Server（端口 37777）向 OpenClaw 暴露工具：

```json
{
  "mcpServers": {
    "memory": {
      "command": "node",
      "args": ["./mcp-proxy.js"],
      "env": {
        "MEMORY_SERVICE_URL": "http://localhost:37777"
      }
    }
  }
}
```

### 4.2 Agent 调用方式

```go
// OpenClaw 通过 MCP 工具调用记忆系统的 Agent

// 方式 1: 直接调用 Agent
result, err := memoryAgent.ExtractDecision(content, topics)

// 方式 2: 通过 MCP 工具调用
// OpenClaw 调用 memory.search -> 触发 ExtractorAgent
```

### 4.3 错误恢复流程

```
失败检测 -> 显性/隐性分类 -> 自我纠正/回滚 -> 护栏检查 -> 降级/人工介入
```

---

## 五、配置扩展

### 5.1 internal/config/settings.go 扩展

```go
// LLMConfig LLM 配置
type LLMConfig struct {
    // OpenClaw LLM（预留）
    OpenClaw OpenClawConfig `yaml:"openclaw"`

    // Agent 配置
    Agent AgentConfig `yaml:"agent"`

    // 预算配置
    Budget BudgetConfig `yaml:"budget"`
}

// OpenClawConfig OpenClaw 配置
type OpenClawConfig struct {
    BaseURL string `yaml:"base_url"`
    Token   string `yaml:"token"`
    Timeout int    `yaml:"timeout"`
}

// AgentConfig Agent 配置
type AgentConfig struct {
    MaxRetries     int     `yaml:"max_retries"`
    Timeout        int     `yaml:"timeout"`
    Temperature    float64 `yaml:"temperature"`
}

// BudgetConfig 预算配置
type BudgetConfig struct {
    SearchTool    int `yaml:"search_tool"`
    FileRead      int `yaml:"file_read"`
    TotalPerAgent int `yaml:"total_per_agent"`
    MaxTotal      int `yaml:"max_total"`
}
```

---

## 六、对齐设计文档

| 本文档章节 | 对齐的设计文档 | 对齐点 |
|-----------|--------------|-------|
| §1.1 专用工具 | agent 集成设计价值观.md | 在万能工具里写「别用我」、Token 预算检查 |
| §1.2 两级工具加载 | agent 集成设计价值观.md | 模型需要工具 -> 调用 Tool Search -> 返回完整定义 |
| §1.3 Agent 拆分 | agent 集成设计价值观.md | Explore Agent、Verification Agent |
| §1.4 错误处理 | agent 集成设计价值观.md | 自我反思、检查点、兜底策略 |
| §3.3 静态段+动态段 | agent 集成设计价值观.md | 服务端复用计算结果、提高缓存命中率 |
| §3.6 检查点 | agent 集成设计价值观.md | Step1(已保存) -> Step2(已保存) -> Step3(检查点) |
| §3.6 护栏检查 | agent 集成设计价值观.md | 最大循环次数、最大Token预算、超时熔断 |
| §3.7 预算控制 | agent 集成设计价值观.md | 搜索工具 ≤ 2w字符、文件读取特殊处理 |
| §3.9 降级策略 | agent 集成设计价值观.md | LLM 不可用时使用纯规则实现 |
| §4 MCP 集成 | memory-openclaw-integration.md | 通过 MCP Server 暴露工具给 OpenClaw |

---

## 七、实施优先级

| Phase | 模块 | 说明 | 依赖 |
|-------|------|------|------|
| **P0** | `types.go` | 所有类型定义 | — |
| **P0** | `budget/` | Token 预算控制 | — |
| **P1** | `prompts/` | 提示词管理 | — |
| **P1** | `tools/` | 工具注册表 | — |
| **P1** | `error_handling/` | 错误处理 | — |
| **P2** | `agents/` | 场景专用 Agent | P0, P1 |
| **P2** | `fallback.go` | 降级策略 | — |
| **P2** | `orchestrator.go` | 工作流编排 | P2 |
| **P3** | `agent.go` | Agent 编排器入口 | P2 |
| **P3** | `options.go` | 调用选项 | — |

---

## 八、总结

### 核心设计原则

1. **专用工具原则** - 每个场景一个专用 Agent，精确控制输出格式和 Token 预算
2. **两级工具加载** - 轻量描述（searchHint）+ 完整定义（按需加载）
3. **错误处理** - 自我反思、检查点、兜底策略
4. **预算控制** - 搜索工具 ≤ 2w字符、全局预算上限
5. **降级策略** - LLM 不可用时使用纯规则实现

### 一句话总结

**LLM 模块采用多 Agent 编排模式，每个场景一个专用 Agent，内置错误处理和预算控制，通过 MCP 工具与 OpenClaw 集成，确保记忆系统的稳定性和可控性。**
