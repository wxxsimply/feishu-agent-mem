# Signal Activation & Correlation Engine

> 适配器状态变化驱动的决策生成方案

8 个独立适配器检测飞书状态变化 → 信号标准化 → 激活路由 → 上下文装配（预算约束）→ 决策状态机 → 持久化

---

## Context

8 个 lark-cli 适配器（IM/VC/Docs/Calendar/Task/OKR/Contact/Wiki）各自独立检测状态变化。核心问题：**一个适配器的状态变化可能需要关联其他适配器的历史数据来生成/更新决策，但模型无法将所有信息源的全部内容放入上下文。**

参考神经网络激活策略——不是所有神经元同时激活，而是根据输入信号的类型和强度，激活不同的权重路径。

---

## 方案概览

在现有 PipelineEngine 之前插入 **Signal Activation Engine**，形成 5 阶段流水线：

```
飞书事件 → 信号发射 → 激活路由 → 上下文装配(预算约束) → 决策状态机 → 持久化
```

现有 PipelineEngine、MemoryGraph、GitStorage/BitableStore **不改动**，新引擎作为预处理层，产出结构化的决策变更操作。

---

## 1. StateChangeSignal 协议

每个适配器在检测到状态变化时，输出标准化信号：

```go
type StateChangeSignal struct {
    SignalID    string          // UUID，用于去重
    Adapter     AdapterType     // IM/VC/Docs/Calendar/Task/OKR/Contact/Wiki
    Timestamp   time.Time
    EventType   string          // 原始飞书事件类型

    ChangeType    ChangeType    // created | updated | deleted | status_change
    ChangeSummary string        // 人类可读摘要

    PrimaryID   EntityId        // 变更的实体
    RelatedIDs  []EntityId      // 引用的其他实体

    Context     SignalContext    // 提取的关键词、人员、URL、内容片段
    Strength    SignalStrength  // strong | medium | weak
}

type SignalContext struct {
    Keywords         []string       // 决策相关关键词
    DecisionSignals  []string       // 匹配到的决策信号词（"决定"、"LGTM"等）
    MentionedIDs     []string       // @提及的用户 open_id
    ActorID          string         // 触发变更的人
    ParticipantIDs   []string       // 参会人、协作者
    EmbeddedURLs     []EmbeddedUrl  // 消息/文档中解析出的 URL
    ContentSnippet   string         // 截断的内容片段（~500字符）
    EventTime        time.Time      // 事件实际发生时间
}
```

### 信号强度规则

| 适配器 | Strong | Medium | Weak |
|--------|--------|--------|------|
| IM | Pin 消息、线程内审批回复 | 含≥2个决策关键词、@提及+上下文 | 仅1个关键词匹配 |
| VC | 会议结束+有AI待办 | 会议结束+有AI总结 | 会议结束无纪要 |
| Docs | 审批评论（/approve/同意） | 含决策关键词的评论 | 文档更新/创建 |
| Task | 任务状态→completed | 任务状态变更（其他） | 任务内容更新 |
| Calendar | — | 含"评审"/"决策"关键词的日程 | 普通日程变更 |
| OKR | — | KR 进度显著变化 | OKR 信息更新 |
| Wiki | — | 知识库新增决策相关节点 | 节点更新 |
| Contact | — | — | 人员信息变更 |

### 适配器接口

```go
// 每个适配器实现此接口
type StateChangeEmitter interface {
    // 将原始飞书事件转为标准化信号，若不相关则返回 nil
    EmitSignal(rawEvent *LarkEvent) (*StateChangeSignal, error)
    AdapterType() AdapterType
}
```

---

## 2. 激活路由器

### 2.1 核心思想：8×3×8 权重矩阵

`source_adapter × strength × target_adapter → weight (0.0-1.0)`

信号到达时，路由器查询矩阵，决定需要激活哪些其他适配器来获取关联上下文。

```
IM 强信号 → 激活 [Docs(0.9), Task(0.8), VC(0.8), Calendar(0.7), Contact(0.6), Wiki(0.4), OKR(0.3)]
Task 强信号 → 激活 [IM(0.7), Calendar(0.7), Docs(0.6), OKR(0.6), VC(0.5), Contact(0.4), Wiki(0.3)]
VC 强信号 → 激活 [IM(0.9), Task(0.9), Docs(0.8), Calendar(0.7), Contact(0.7), Wiki(0.5), OKR(0.4)]
```

### 2.2 权重矩阵（完整）

```
                    ┌─────────────────── 目标适配器 ───────────────────┐
                    │ IM    VC    Docs  Cal   Task  OKR   Contact Wiki │
┌──────────────────┼──────────────────────────────────────────────────┤
│ 源: IM            │                                                │
│   Strong          │ -     0.8   0.9   0.7   0.8   0.3   0.6   0.4 │
│   Medium          │ -     0.5   0.7   0.4   0.6   0.2   0.4   0.3 │
│   Weak            │ -     0.2   0.3   0.1   0.3   0.0   0.2   0.1 │
├──────────────────┼──────────────────────────────────────────────────┤
│ 源: VC            │                                                │
│   Strong          │ 0.9   -     0.8   0.7   0.9   0.4   0.7   0.5 │
│   Medium          │ 0.6   -     0.5   0.4   0.7   0.2   0.5   0.3 │
│   Weak            │ 0.2   -     0.2   0.1   0.3   0.0   0.3   0.1 │
├──────────────────┼──────────────────────────────────────────────────┤
│ 源: Docs          │                                                │
│   Strong          │ 0.8   0.6   -     0.5   0.7   0.3   0.5   0.7 │
│   Medium          │ 0.5   0.3   -     0.3   0.5   0.2   0.3   0.5 │
│   Weak            │ 0.2   0.1   -     0.1   0.2   0.0   0.1   0.2 │
├──────────────────┼──────────────────────────────────────────────────┤
│ 源: Calendar      │                                                │
│   Strong          │ 0.7   0.8   0.6   -     0.7   0.4   0.6   0.3 │
│   Medium          │ 0.4   0.5   0.3   -     0.5   0.2   0.4   0.2 │
│   Weak            │ 0.1   0.2   0.1   -     0.2   0.0   0.2   0.0 │
├──────────────────┼──────────────────────────────────────────────────┤
│ 源: Task          │                                                │
│   Strong          │ 0.7   0.5   0.6   0.7   -     0.6   0.4   0.3 │
│   Medium          │ 0.4   0.3   0.3   0.4   -     0.4   0.3   0.2 │
│   Weak            │ 0.1   0.1   0.1   0.1   -     0.1   0.1   0.0 │
├──────────────────┼──────────────────────────────────────────────────┤
│ 源: OKR           │                                                │
│   Strong          │ 0.5   0.4   0.5   0.6   0.8   -     0.3   0.3 │
│   Medium          │ 0.3   0.2   0.3   0.3   0.5   -     0.2   0.2 │
│   Weak            │ 0.1   0.0   0.1   0.1   0.2   -     0.1   0.0 │
├──────────────────┼──────────────────────────────────────────────────┤
│ 源: Contact       │                                                │
│   Strong          │ 0.6   0.4   0.3   0.4   0.3   0.2   -     0.2 │
│   Medium          │ 0.3   0.2   0.2   0.2   0.2   0.1   -     0.1 │
│   Weak            │ 0.1   0.0   0.0   0.0   0.0   0.0   -     0.0 │
├──────────────────┼──────────────────────────────────────────────────┤
│ 源: Wiki          │                                                │
│   Strong          │ 0.6   0.5   0.8   0.3   0.5   0.3   0.4   -   │
│   Medium          │ 0.3   0.3   0.5   0.2   0.3   0.2   0.2   -   │
│   Weak            │ 0.1   0.1   0.2   0.0   0.1   0.0   0.1   -   │
└──────────────────┴──────────────────────────────────────────────────┘

权重阈值:
  ≥ 0.7  = MUST（必须查询）
  0.4-0.7 = SHOULD（应该查询）
  0.1-0.4 = MAY（预算允许时查询）
  < 0.1  = SKIP（跳过）
```

### 2.3 数据结构

```go
type ContextQueryPlan struct {
    SignalID        string
    TotalTokenBudget int
    Queries         []ContextQuery
}

type ContextQuery struct {
    QueryID      string
    TargetAdapter AdapterType
    QueryType    QueryType       // SearchByKeyword | SearchByPerson | SearchByEntity | SearchByTimeRange | GetEntityDetail | SearchDecisionsByTopic ...
    Params       QueryParams
    TokenBudget  int
    Priority     int             // 0 = 最高优先级
    Purpose      string          // 人类可读说明
}

type ActivationTarget struct {
    Adapter  AdapterType
    Weight   float64
    Priority Priority          // Must | Should | May
}
```

### 2.4 路由算法

```go
func (r *ActivationRouter) Route(signal *StateChangeSignal) ContextQueryPlan {
    // 1. 根据信号强度确定 token 预算
    //    Strong → 6000, Medium → 2000, Weak → 500
    budget := ComputeBudget(signal.Strength)

    // 2. 始终优先搜索 MemoryGraph 中的已有决策
    //    基于 signal.Context.Keywords + MentionedIDs 生成查询
    queries := DecisionLookupQueries(signal, budget)

    // 3. 查权重矩阵，获取需要激活的目标适配器（按权重降序）
    activations := r.Matrix.Activate(signal.Adapter, signal.Strength)

    // 4. 为每个激活的适配器生成具体查询
    for _, act := range activations {
        queries = append(queries, GenerateQueries(signal, act)...)
    }

    // 5. 信号中的 embedded_urls → 直接查询对应实体
    for _, url := range signal.Context.EmbeddedURLs {
        if url.ExtractedToken != "" {
            queries = append(queries, URLEntityQuery(url))
        }
    }

    // 6. @提及的人员 → 查询人员关联的决策/任务
    for _, openID := range signal.Context.MentionedIDs {
        queries = append(queries, PersonContextQuery(openID, signal))
    }

    // 7. 按 Priority 排序，裁剪到总预算
    sort.Slice(queries, func(i, j int) bool { return queries[i].Priority < queries[j].Priority })
    TrimToBudget(queries, budget)

    return ContextQueryPlan{SignalID: signal.SignalID, TotalTokenBudget: budget, Queries: queries}
}
```

### 2.5 动态权重调整

权重矩阵不是静态的，根据实际关联效果反馈调整：

```go
// 成功关联后，增加提供有用上下文的适配器权重
// 逻辑：指数移动平均 α=0.1
// new_weight = 0.9 * old_weight + 0.1 * usefulness_score
func (m *ActivationMatrix) Reinforce(source, strength, usefulAdapter, score)

// 定期衰减不常用的权重，防止权重固化
// 逻辑：weight *= (1 - decay_rate)
func (m *ActivationMatrix) Decay(decayRate float64)
```

---

## 3. 上下文装配（预算约束）

### 3.1 装配策略

给定信号和查询结果，在 token 预算内贪心选择最相关的上下文：

| 信号强度 | Token 预算 | 包含决策 | 适配器结果 | 时间窗口 |
|---------|-----------|---------|-----------|---------|
| Strong | ~6K | 话题内全部 active + 跨话题 | 4+适配器各 Top 10 | 30天 |
| Medium | ~2K | 话题内 Top 5 + 跨话题引用 | 2-3适配器各 Top 5 | 14天 |
| Weak | ~500 | 话题内 Top 3 | 1-2适配器各 Top 2 | 7天 |

### 3.2 相关性评分

每个候选上下文项按三个维度评分，贪心填充预算：

```
score = recency * 0.3 + person_overlap * 0.3 + keyword_overlap * 0.4

recency       = e^(-0.1 × age_days)          // 时间衰减，半衰期≈7天
person_overlap = |A ∩ B| / |A ∪ B|            // Jaccard(信号人员, 条目人员)
keyword_overlap = |A ∩ B| / |A ∪ B|           // Jaccard(信号关键词, 条目关键词)
```

### 3.3 装配流程

```go
func (a *ContextAssembler) Assemble(signal, queryResults, existingDecisions) AssembledContext {
    // 1. 对已有决策和适配器结果分别评分
    // 2. 按分数降序排列
    // 3. 贪心选择：优先放入高分决策，再放入高分适配器结果，直到预算耗尽
    // 4. 计算关联提示（CorrelationHints）
}
```

### 3.4 关联提示

装配器预先计算关联提示，辅助状态机判断：

| 提示类型 | 触发条件 | 置信度 |
|---------|---------|--------|
| DocDecisionLink | 信号中的文档 URL 与某决策关联同一文档 | 0.8 |
| TaskDecisionProgress | 任务完成信号 + 执行中的决策 | 0.7 |
| MeetingDecisionLink | 会议结束 + 话题内决策 | 0.7 |
| PersonDecisionLink | 人员重叠 | 0.5 |

---

## 4. 决策状态机

### 4.1 状态流转

```
待决策 ──→ 决策中 ──→ 已决策 ──→ 执行中 ──→ 已完成
  │          │          │          │
  └──→ 搁置 ←┘          └──→ 否决  └──→ 搁置
```

### 4.2 转换规则（信号驱动）

| 当前状态 | 触发适配器 | 信号强度 | 条件 | 新状态 |
|---------|-----------|---------|------|--------|
| 待决策 | IM | Medium+ | 决策关键词讨论 | 决策中 |
| 待决策 | Calendar | Medium+ | 创建决策评审会 | 决策中 |
| 决策中 | Docs | Strong | 文档评论含审批 | 已决策 |
| 决策中 | VC | Strong | 会议纪要确认结论 | 已决策 |
| 决策中 | IM | Strong | IM 审批（LGTM等） | 已决策 |
| 已决策 | Task | Medium+ | 创建关联任务 | 执行中 |
| 执行中 | Task | Strong | 所有相关任务完成 | 已完成 |
| 执行中 | Calendar | Strong | 里程碑到达 | 已完成 |

### 4.3 状态机实现

```go
func (sm *DecisionStateMachine) EvaluateTransition(current *DecisionNode, signal *StateChangeSignal, ctx *AssembledContext) *StateTransition {
    // 按 (当前状态, 信号适配器, 信号强度) 三元组匹配转换规则
    // 关键判定：allRelatedTasksCompleted
    //   → 查询 ctx 中 Task 适配器结果，检查决策关联的所有任务是否全部完成
}
```

### 4.4 关键判定：allRelatedTasksCompleted

当任务完成信号到达时，查询 MemoryGraph 中该决策关联的所有任务状态。这是跨适配器关联的典型场景——Task 适配器的状态变化触发对 MemoryGraph 中其他任务记录的检查。

---

## 5. 跨适配器关联模式（5 种）

```go
type CorrelationPattern struct {
    PatternID          string
    Name               string
    TriggerSignals     []SignalPattern       // 触发条件
    RequiredContext    []ContextRequirement  // 需要从哪些适配器拉取上下文
    Action             PatternAction         // 匹配后执行的动作
    ConfidenceThreshold float64
}
```

| 模式 | 触发信号 | 需要的上下文 | 执行动作 |
|------|---------|------------|---------|
| IM + 会议 → 决策 | IM 消息含决策关键词 | VC 会议纪要（±2小时） | 创建新决策 |
| 任务 + 里程碑 → 进度 | 任务完成 | Calendar 里程碑（±1天） | 推进决策状态 |
| 文档审批 + IM → 确认 | 文档审批评论 | IM 讨论（近期） | 确认决策 |
| 会议结束 → 创建任务 | 会议结束+待办 | AI 纪要待办列表 | 创建执行任务 |
| OKR + 任务 → 对齐 | 任务完成 | OKR KR 进度 | 关联任务到 KR |

---

## 6. SignalActivationEngine 编排器

```go
type SignalActivationEngine struct {
    Emitters     map[AdapterType]StateChangeEmitter
    Router       *ActivationRouter
    Assembler    *ContextAssembler
    StateMachine *DecisionStateMachine
    Patterns     *PatternMatcher
    Pipeline     *PipelineEngine   // 复用现有
    Memory       *MemoryGraph      // 复用现有
}

func (e *SignalActivationEngine) OnEvent(event *LarkEvent) (*ProcessingReport, error) {
    // Step 1: 识别适配器，发射信号
    adapter := IdentifyAdapter(event)
    signal := e.Emitters[adapter].EmitSignal(event)
    if signal == nil {
        return nil, nil  // 不相关事件，跳过
    }

    // Step 2: 激活路由（决定查哪些适配器）
    plan := e.Router.Route(signal)

    // Step 3: 执行查询 + 上下文装配
    context := e.ExecuteAndAssemble(plan, signal)

    // Step 4: 匹配关联模式 + 评估状态转换
    transitions := e.Evaluate(signal, context)

    // Step 5: 通过现有 PipelineEngine 执行变更
    for _, t := range transitions {
        e.ApplyTransition(t, context)
    }
}
```

集成点：
```
现有: EventManager → PipelineEngine.on_event(event)
新增: EventManager → SignalActivationEngine.on_event(event) → PipelineEngine.on_decision(mutation)
```

---

## 7. 典型场景走读

### 场景 A：IM 消息触发新决策

```
1. 用户发消息 "决定了，数据库用 PostgreSQL"
2. IM 适配器发射信号: adapter=IM, strength=Medium, keywords=["数据库","PostgreSQL"]
3. 激活路由: 查权重矩阵 IM/Medium → {Docs:0.7, Task:0.6, VC:0.5, Calendar:0.4, ...}
4. 上下文装配(2K预算):
   - MemoryGraph: "数据库架构"话题下2条已有决策
   - Task: 1个相关任务"数据库选型"
   - Docs: 1篇相关文档"数据库技术方案"
5. 状态机: 无匹配的已有决策 → 创建新 DecisionNode, status=待决策
```

### 场景 B：任务完成推进决策到已完成

```
1. 飞书任务 "PostgreSQL 部署" 标记完成
2. Task 适配器发射信号: adapter=Task, strength=Strong, primary_id=task_xxx
3. 激活路由: 查矩阵 Task/Strong → {IM:0.7, Calendar:0.7, Docs:0.6, OKR:0.6, ...}
4. 上下文装配(6K预算):
   - MemoryGraph: 通过 task_xxx 找到决策 dec_20260428_001 (执行中)
   - Calendar: 检查里程碑
   - Task: 检查所有相关任务状态
5. 状态机: 执行中 + Task完成 + all_related_tasks_completed → 已完成
   → 写 Git + 同步 Bitable
```

### 场景 C：一个月前的 IM 消息 + 当前任务完成

```
1. 一个月前: 群聊讨论"数据库字段从 INT 改为 BIGINT"，已存为决策(执行中)
2. 今天: 任务 "数据库迁移脚本" 完成
3. Task 适配器信号触发 → 路由器查 MemoryGraph
4. MemoryGraph 通过 keywords=["数据库","迁移"] 找到一个月前的决策
5. 检查所有关联任务 → 全部完成 → 推进状态
关键点: IM 适配器不需要检测到状态变化，Task 信号通过 MemoryGraph 关联到了历史决策
```

---

## 8. 文件结构

```
src/
├── core/
│   ├── signal.go              // StateChangeSignal、SignalContext、SignalStrength
│   ├── activation_router.go   // ActivationRouter、ActivationMatrix、ContextQueryPlan
│   ├── context_assembler.go   // ContextAssembler、评分算法、预算裁剪
│   ├── state_machine.go       // DecisionStateMachine、状态转换规则
│   ├── correlation_patterns.go // 5 种跨适配器关联模式
│   └── signal_engine.go       // SignalActivationEngine 编排器
├── adapters/
│   ├── emitter.go             // StateChangeEmitter 接口
│   ├── im_emitter.go          // IM 信号发射器
│   ├── vc_emitter.go          // VC 信号发射器
│   ├── docs_emitter.go        // Docs 信号发射器
│   ├── calendar_emitter.go    // Calendar 信号发射器
│   ├── task_emitter.go        // Task 信号发射器
│   ├── okr_emitter.go         // OKR 信号发射器
│   ├── contact_emitter.go     // Contact 信号发射器
│   └── wiki_emitter.go        // Wiki 信号发射器
└── main.go                    // 初始化 SignalActivationEngine，替换事件入口
```

---

## 9. 验证方案

1. **单元测试**: 每个适配器的信号强度判定、权重矩阵查询、评分算法
2. **集成测试**: 模拟 IM 消息 → 验证路由到正确的适配器查询 → 验证上下文装配在预算内
3. **端到端测试**: 使用 simulated_chats.json + simulated_sdrs.json 模拟场景 A/B/C
4. **验证命令**:
   - `lark-cli im +messages-search` → 触发 IM 信号 → 检查路由目标
   - `lark-cli task +get-my-tasks` → 触发 Task 信号 → 检查决策状态推进
   - `lark-cli vc +notes` → 触发 VC 信号 → 检查会议→决策关联
