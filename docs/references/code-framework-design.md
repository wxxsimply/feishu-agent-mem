# feishu-agent-mem 代码框架设计

> OpenClaw 记忆系统完整代码框架。Golang 实现，部署在 openclaw-zh Docker 容器中。覆盖 8 个适配器检测器、信号激活引擎、Git 权威存储、Bitable 查询接口、MCP Server、OpenClaw Core 协作的全部代码模块。

---

## 一、总体架构与模块关系

### 1.1 分层架构

```
┌──────────────────────────────────────────────────────────────────┐
│                    feishu-agent-mem (Go binary)                    │
│                                                                    │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                      cmd/mem-service/                         │ │
│  │                        (服务入口)                              │ │
│  └────────────────────────────┬────────────────────────────────┘ │
│                               │                                    │
│  ┌────────────────────────────┼────────────────────────────────┐ │
│  │                      OpenClaw Core                            │ │
│  │                                                               │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │ │
│  │  │ PipelineEngine│  │ MemoryGraph  │  │ConflictResolver│      │ │
│  │  │ (流程编排)     │  │ (内存决策图)  │  │ (冲突仲裁)     │      │ │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │ │
│  └─────────┼──────────────────┼──────────────────┼──────────────┘ │
│            │                  │                  │                 │
│  ┌─────────┼──────────────────┼──────────────────┼──────────────┐ │
│  │         ▼                  ▼                  ▼               │ │
│  │              Signal Activation Engine (信号引擎)               │ │
│  │                                                               │ │
│  │  Detector → Emitter → ActivationRouter → ContextAssembler    │ │
│  │                                          → StateMachine       │ │
│  └──────────────────────────┬───────────────────────────────────┘ │
│                             │                                      │
│  ┌──────────────────────────┼───────────────────────────────────┐ │
│  │                    Storage Layer (存储层)                      │ │
│  │                          │                                    │ │
│  │  ┌────────────┐  ┌───────┴──────┐  ┌──────────────────┐     │ │
│  │  │ GitStorage │  │ BitableStore │  │ LarkAdapter       │     │ │
│  │  │ (决策文件R/W)│ │ (结构化查询)  │  │ (lark-cli exec封装)│    │ │
│  │  └────────────┘  └──────────────┘  └──────┬───────────┘     │ │
│  └───────────────────────────────────────────┼──────────────────┘ │
│                                              │                    │
│  ┌───────────────────────────────────────────┼──────────────────┐ │
│  │                    MCP Server (端口 37777)                     │ │
│  │                                                               │ │
│  │  memory.search / memory.topic / memory.decision               │ │
│  │  memory.timeline / memory.conflict / memory.signal            │ │
│  └───────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

### 1.2 对齐设计文档

| 代码模块 | 对应设计文档 | 覆盖内容 |
|---------|------------|---------|
| `internal/decision/` | decision-tree.md §2.1 | DecisionNode 结构体 |
| `internal/lark-adapter/` | decision-extraction.md §1-4 | 8 个飞书适配器 |
| `internal/core/` | openclaw-architecture.md §2.1 | PipelineEngine, MemoryGraph |
| `internal/signal/` | signal-activation-engine.md | SignalActivationEngine |
| `internal/storage/git/` | git-operations-design.md | GitStorage |
| `internal/storage/bitable/` | openclaw-architecture.md §2.4 | BitableStore |
| `internal/mcp/` | memory-openclaw-integration.md §6.3 | MCP Server 6 个工具 |

---

## 二、项目目录结构

```
feishu-agent-mem/
│
├── cmd/
│   └── mem-service/
│       └── main.go                    # 服务入口（长期运行）
│
├── internal/
│   │
│   ├── decision/                      # === 决策数据模型 ===
│   │   ├── node.go                    # DecisionNode 定义
│   │   ├── relation.go                # RelationType 枚举
│   │   ├── status.go                  # DecisionStatus 枚举
│   │   ├── impact.go                  # ImpactLevel 枚举
│   │   └── feishu_links.go            # FeishuLinks 结构体
│   │
│   ├── lark-adapter/                  # === 飞书适配器层 === (已有)
│   │   ├── lark_cli.go               # LarkCLI exec 封装 (已有)
│   │   ├── config.go                  # Config (已有)
│   │   ├── types.go                   # Extractor/Detector 接口 (已有)
│   │   ├── lark_im.go                 # IM 适配器 (已有)
│   │   ├── lark_calendar.go           # Calendar 适配器 (已有)
│   │   ├── lark_doc.go                # Doc 适配器 (已有)
│   │   ├── lark_wiki.go               # Wiki 适配器 (已有)
│   │   ├── lark_vc.go                 # VC 适配器 (已有)
│   │   ├── lark_minutes.go            # Minutes 适配器 (已有)
│   │   ├── lark_task.go               # Task 适配器 (已有)
│   │   ├── lark_okr.go                # OKR 适配器 (已有)
│   │   └── lark_contact.go            # Contact 适配器 (已有)
│   │
│   ├── signal/                        # === 信号激活引擎 ===
│   │   ├── signal.go                  # StateChangeSignal 协议
│   │   ├── emitter.go                 # StateChangeEmitter 接口 + 9 个实现
│   │   ├── activation_router.go       # 8×3×8 权重矩阵 + 路由算法
│   │   ├── context_assembler.go       # 上下文装配 + 预算约束
│   │   ├── state_machine.go           # 决策状态机
│   │   ├── correlation_patterns.go    # 5 种跨适配器关联模式
│   │   └── engine.go                  # SignalActivationEngine 编排器
│   │
│   ├── core/                          # === OpenClaw Core ===
│   │   ├── pipeline.go                # PipelineEngine
│   │   ├── memory_graph.go            # MemoryGraph（内存索引）
│   │   ├── conflict_resolver.go       # ConflictResolver
│   │   ├── topic_manager.go           # Topic 管理
│   │   └── event_manager.go           # 事件订阅管理
│   │
│   ├── storage/                       # === 持久化层 ===
│   │   ├── git/
│   │   │   ├── git_storage.go         # GitStorage（决策文件 R/W）
│   │   │   ├── commit.go              # Commit 管理
│   │   │   ├── diff.go                # Diff/Blame 封装
│   │   │   ├── branch.go              # 分支管理
│   │   │   └── hooks.go               # Git Hooks 安装
│   │   │
│   │   └── bitable/
│   │       ├── bitable_store.go       # BitableStore（CRUD + 查询）
│   │       ├── schema.go              # 表结构定义 + 字段映射
│   │       └── sync.go                # git_commit_hash 同步
│   │
│   ├── mcp/                           # === MCP Server ===
│   │   ├── server.go                  # MCP Server 启动 + 路由
│   │   ├── tools.go                   # 6 个工具注册 + handler
│   │   ├── search_tool.go             # memory.search 实现
│   │   ├── topic_tool.go              # memory.topic 实现
│   │   ├── decision_tool.go           # memory.decision 实现
│   │   ├── timeline_tool.go           # memory.timeline 实现
│   │   ├── conflict_tool.go           # memory.conflict 实现
│   │   └── signal_tool.go             # memory.signal 实现
│   │
│   ├── context/                       # === 渐进式披露 ===
│   │   ├── index.go                   # 索引层（~800 tokens）
│   │   ├── assembler.go               # 上下文装配器
│   │   └── budget.go                  # Token 预算控制
│   │
│   └── config/                        # === 配置 ===
│       ├── settings.go                # 完整配置结构体
│       └── defaults.go                # 默认值
│
├── test/
│   ├── detector/                      # 检测器测试 (已有)
│   │   ├── detector_im_test.go
│   │   └── ...
│   └── integration/                   # 集成测试 (新增)
│       └── pipeline_test.go
│
├── config/                            # === 配置文件 ===
│   ├── openclaw.yaml                  # 主配置
│   └── mcp.json                       # MCP 注册配置
│
├── data/                              # === Git 工作目录（运行时生成） ===
│   ├── decisions/
│   │   └── {project}/{topic}/{sdr_id}.md
│   ├── conflicts/
│   ├── archive/
│   └── L0_RULES.md
│
├── outputs/                           # === 提取输出 (已有) ===
│   ├── *.json
│   └── detect_state.json
│
├── scripts/                           # === 运维脚本 ===
│   ├── start.sh                       # Docker 内启动
│   └── health_check.sh                # 健康检查
│
├── Dockerfile                         # 集成到 openclaw-zh 镜像
├── go.mod
├── go.sum
├── Makefile
├── .env
└── CLAUDE.md
```

---

## 三、核心数据结构

### 3.1 DecisionNode（对齐 decision-tree.md §2.1）

```go
// internal/decision/node.go

package decision

import "time"

// DecisionNode 决策节点 — 与 decision-tree.md §2.1 严格对齐
type DecisionNode struct {
    // === 标识与内容 ===
    SDRID          string `json:"sdr_id" yaml:"sdr_id"`
    GitCommitHash  string `json:"git_commit_hash" yaml:"git_commit_hash"`
    Title          string `json:"title" yaml:"title"`
    Decision       string `json:"decision" yaml:"decision"`
    Rationale      string `json:"rationale" yaml:"rationale"`

    // === 树位置 ===
    Project string `json:"project" yaml:"project"`
    Topic   string `json:"topic" yaml:"topic"` // 唯一位置锚点

    // === 时态标签 ===
    Phase      string      `json:"phase" yaml:"phase"`
    PhaseScope PhaseScope  `json:"phase_scope" yaml:"phase_scope"`
    VersionRange VersionRange `json:"version_range" yaml:"version_range"`

    // === 影响级别 ===
    ImpactLevel ImpactLevel `json:"impact_level" yaml:"impact_level"`

    // === 跨议题 ===
    CrossTopicRefs []string `json:"cross_topic_refs" yaml:"cross_topic_refs"`

    // === 树内父子 ===
    ParentDecision string `json:"parent_decision" yaml:"parent_decision"`
    ChildrenCount  int    `json:"children_count" yaml:"children_count"`

    // === 人员 ===
    Proposer      string   `json:"proposer" yaml:"proposer"`
    Executor      string   `json:"executor" yaml:"executor"`
    Stakeholders  []string `json:"stakeholders" yaml:"stakeholders"`

    // === 关系图谱 ===
    Relations []Relation `json:"relations" yaml:"relations"`

    // === 飞书关联 ===
    FeishuLinks FeishuLinks `json:"feishu_links" yaml:"feishu_links"`

    // === 状态 ===
    Status    DecisionStatus `json:"status" yaml:"status"`
    CreatedAt time.Time      `json:"created_at" yaml:"created_at"`
    DecidedAt *time.Time     `json:"decided_at" yaml:"decided_at"`
}

// PhaseScope 阶段范围
type PhaseScope string
const (
    PhaseScopePoint       PhaseScope = "Point"
    PhaseScopeSpan        PhaseScope = "Span"
    PhaseScopeRetroactive PhaseScope = "Retroactive"
)

// ImpactLevel 影响级别（取代 tree_level）
type ImpactLevel string
const (
    ImpactAdvisory ImpactLevel = "advisory"
    ImpactMinor    ImpactLevel = "minor"
    ImpactMajor    ImpactLevel = "major"
    ImpactCritical ImpactLevel = "critical"
)

// DecisionStatus 决策状态
type DecisionStatus string
const (
    StatusPending       DecisionStatus = "pending"
    StatusInDiscussion  DecisionStatus = "in_discussion"
    StatusDecided       DecisionStatus = "decided"
    StatusExecuting     DecisionStatus = "executing"
    StatusCompleted     DecisionStatus = "completed"
    StatusShelved       DecisionStatus = "shelved"
    StatusRejected      DecisionStatus = "rejected"
    StatusSuperseded    DecisionStatus = "superseded"
    StatusDeprecated    DecisionStatus = "deprecated"
)

// VersionRange 版本范围
type VersionRange struct {
    From string `json:"from" yaml:"from"`
    To   string `json:"to" yaml:"to"` // null = 当前生效
}

// FeishuLinks 飞书实体关联
type FeishuLinks struct {
    RelatedChatIDs       []string `json:"related_chat_ids" yaml:"related_chat_ids"`
    RelatedMessageIDs    []string `json:"related_message_ids" yaml:"related_message_ids"`
    RelatedDocTokens     []string `json:"related_doc_tokens" yaml:"related_doc_tokens"`
    RelatedEventIDs      []string `json:"related_event_ids" yaml:"related_event_ids"`
    RelatedMeetingIDs    []string `json:"related_meeting_ids" yaml:"related_meeting_ids"`
    RelatedTaskGUIDs     []string `json:"related_task_guids" yaml:"related_task_guids"`
    RelatedMinuteTokens  []string `json:"related_minute_tokens" yaml:"related_minute_tokens"`
}
```

### 3.2 Relation（关系图边）

```go
// internal/decision/relation.go

package decision

type RelationType string
const (
    RelationDependsOn      RelationType = "DEPENDS_ON"
    RelationSupersedes     RelationType = "SUPERSEDES"
    RelationRefines        RelationType = "REFINES"
    RelationConflictsWith  RelationType = "CONFLICTS_WITH"
    RelationRelatesTo      RelationType = "RELATES_TO"
)

type Relation struct {
    Type          RelationType `json:"type" yaml:"type"`
    TargetSDRID   string       `json:"target_sdr_id" yaml:"target_sdr_id"`
    Description   string       `json:"description,omitempty" yaml:"description,omitempty"`
}
```

### 3.3 StateChangeSignal（对齐 signal-activation-engine.md §1）

```go
// internal/signal/signal.go

package signal

import "time"

type AdapterType string
const (
    AdapterIM       AdapterType = "IM"
    AdapterVC       AdapterType = "VC"
    AdapterDocs     AdapterType = "Docs"
    AdapterCalendar AdapterType = "Calendar"
    AdapterTask     AdapterType = "Task"
    AdapterOKR      AdapterType = "OKR"
    AdapterContact  AdapterType = "Contact"
    AdapterWiki     AdapterType = "Wiki"
)

type ChangeType string
const (
    ChangeCreated      ChangeType = "created"
    ChangeUpdated      ChangeType = "updated"
    ChangeDeleted      ChangeType = "deleted"
    ChangeStatusChange ChangeType = "status_change"
)

type SignalStrength string
const (
    StrengthStrong SignalStrength = "strong"
    StrengthMedium SignalStrength = "medium"
    StrengthWeak   SignalStrength = "weak"
)

// StateChangeSignal 每个适配器检测到变化时输出的标准信号
type StateChangeSignal struct {
    SignalID      string         `json:"signal_id"`
    Adapter       AdapterType    `json:"adapter"`
    Timestamp     time.Time      `json:"timestamp"`
    EventType     string         `json:"event_type"`
    ChangeType    ChangeType     `json:"change_type"`
    ChangeSummary string         `json:"change_summary"`
    PrimaryID     string         `json:"primary_id"`
    RelatedIDs    []string       `json:"related_ids"`
    Context       SignalContext  `json:"context"`
    Strength      SignalStrength `json:"strength"`
}

type SignalContext struct {
    Keywords        []string      `json:"keywords"`
    DecisionSignals []string      `json:"decision_signals"`
    MentionedIDs    []string      `json:"mentioned_ids"`
    ActorID         string        `json:"actor_id"`
    ParticipantIDs  []string      `json:"participant_ids"`
    EmbeddedURLs    []EmbeddedURL `json:"embedded_urls"`
    ContentSnippet  string        `json:"content_snippet"`
    EventTime       time.Time     `json:"event_time"`
}

type EmbeddedURL struct {
    RawURL         string `json:"raw_url"`
    ExtractedToken string `json:"extracted_token"`
    URLType        string `json:"url_type"` // doc | wiki | meeting | minute | sheet | bitable
}
```

### 3.4 MemoryGraph（对齐 openclaw-architecture.md §2.1.2）

```go
// internal/core/memory_graph.go

package core

import (
    "feishu-mem/internal/decision"
    "sync"
)

// MemoryGraph 内存决策图 — 运行时加速
type MemoryGraph struct {
    mu sync.RWMutex

    // 决策索引: sdr_id → *DecisionNode
    decisions map[string]*decision.DecisionNode

    // 议题索引: topic_name → []sdr_id
    topics map[string][]string

    // 关系索引: sdr_id → []Relation
    relations map[string][]decision.Relation

    // 跨议题引用索引: topic → []related_topic
    crossTopicRefs map[string][]string

    // 项目索引
    projects map[string][]string // project → []topic_name

    // 脏标记
    dirtyDecisions map[string]struct{}
}

// NewMemoryGraph 创建空图
func NewMemoryGraph() *MemoryGraph {
    return &MemoryGraph{
        decisions:      make(map[string]*decision.DecisionNode),
        topics:         make(map[string][]string),
        relations:      make(map[string][]decision.Relation),
        crossTopicRefs: make(map[string][]string),
        projects:       make(map[string][]string),
        dirtyDecisions: make(map[string]struct{}),
    }
}

// LoadFromGit 启动时从 Git 全量加载决策
func (mg *MemoryGraph) LoadFromGit(gitStorage GitReader) error

// QueryByTopic 按议题检索 active 决策
func (mg *MemoryGraph) QueryByTopic(topic string) []*decision.DecisionNode

// QueryCrossTopic 跨议题检索（含 cross_topic_refs）
func (mg *MemoryGraph) QueryCrossTopic(topic string) []*decision.DecisionNode

// DetectConflicts 检测新决策与现有决策的冲突
func (mg *MemoryGraph) DetectConflicts(new *decision.DecisionNode) []Conflict
```

---

## 四、飞书适配器层（已有代码 + 扩展）

### 4.1 适配器接口（扩展 Detector → Emitter）

已有 `Detector` 接口保持不变，新增 `StateChangeEmitter` 接口桥接：

```go
// internal/signal/emitter.go

package signal

import (
    larkadapter "feishu-mem/internal/lark-adapter"
)

// StateChangeEmitter 将 Detector 的检测结果转为标准信号
type StateChangeEmitter interface {
    // AdapterType 返回适配器类型
    AdapterType() AdapterType

    // EmitSignal 将 LarkEvent (DetectResult) 转为标准信号
    // 若变化不产生决策信号（如噪音变化），返回 nil
    EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error)
}
```

### 4.2 已有适配器的改造点

每个适配器需要增加一个 Emitter 实现。以 IM 为例：

```go
// internal/signal/emitter.go — IM 发射器

type IMEmitter struct {
    keywords []string // 决策信号关键词
}

func (e *IMEmitter) AdapterType() AdapterType { return AdapterIM }

func (e *IMEmitter) EmitSignal(result *larkadapter.DetectResult) (*StateChangeSignal, error) {
    if !result.HasChanges {
        return nil, nil
    }

    signal := &StateChangeSignal{
        SignalID:  generateSignalID(AdapterIM),
        Adapter:   AdapterIM,
        Timestamp: result.DetectedAt,
    }

    strength := StrengthWeak
    keywords := []string{}
    decisionSignals := []string{}

    for _, ch := range result.Changes {
        // 分类信号强度
        if ch.EntityType == "pin_message" {
            strength = StrengthStrong
            decisionSignals = append(decisionSignals, "pin")
        }
        if ch.Type == "new_text" || ch.Type == "new_post" {
            // 关键词匹配
            matched := e.matchKeywords(ch.Summary)
            if len(matched) >= 2 {
                strength = maxStrength(strength, StrengthMedium)
            } else if len(matched) == 1 {
                strength = maxStrength(strength, StrengthWeak)
            }
            keywords = append(keywords, matched...)
            decisionSignals = append(decisionSignals, e.matchDecisionWords(ch.Summary)...)
        }
    }

    // 弱信号且无决策关键词 → 不发射（噪音）
    if strength == StrengthWeak && len(decisionSignals) == 0 {
        return nil, nil
    }

    signal.Strength = strength
    signal.Context.Keywords = keywords
    signal.Context.DecisionSignals = decisionSignals
    signal.Context.ContentSnippet = result.Changes[0].Summary // 取第一条摘要
    signal.ChangeSummary = fmt.Sprintf("IM: %d changes, %d decision signals",
        len(result.Changes), len(decisionSignals))

    return signal, nil
}
```

---

## 五、信号激活引擎

### 5.1 激活路由器（对齐 signal-activation-engine.md §2）

```go
// internal/signal/activation_router.go

package signal

// ActivationMatrix 8×3×8 权重矩阵
type ActivationMatrix struct {
    // source_adapter × strength × target_adapter → weight (0.0-1.0)
    weights map[AdapterType]map[SignalStrength]map[AdapterType]float64
}

// ActivationPriority 激活优先级
type ActivationPriority int
const (
    PrioritySkip   ActivationPriority = iota // weight < 0.1
    PriorityMay                               // 0.1-0.4
    PriorityShould                            // 0.4-0.7
    PriorityMust                              // >= 0.7
)

// ActivationTarget 激活目标
type ActivationTarget struct {
    Adapter  AdapterType
    Weight   float64
    Priority ActivationPriority
}

// ContextQuery 上下文查询
type ContextQuery struct {
    QueryID       string
    TargetAdapter AdapterType
    TokenBudget   int
    Priority      int // 0 = 最高优先级
    Purpose       string
}

// ContextQueryPlan 查询计划
type ContextQueryPlan struct {
    SignalID         string
    TotalTokenBudget int
    Queries          []ContextQuery
}

// ActivationRouter 激活路由器
type ActivationRouter struct {
    Matrix *ActivationMatrix
}

// Route 根据信号决定需要查询哪些适配器
func (r *ActivationRouter) Route(signal *StateChangeSignal) *ContextQueryPlan
```

### 5.2 上下文装配器（对齐 signal-activation-engine.md §3）

```go
// internal/signal/context_assembler.go

package signal

import "feishu-mem/internal/decision"

// AssembledContext 装配后的上下文包
type AssembledContext struct {
    Decisions        []*decision.DecisionNode // 相关决策
    AdapterResults   map[AdapterType]any      // 各适配器查询结果
    CorrelationHints []CorrelationHint        // 关联提示
    TotalTokens      int                      // 消耗的 token 数
}

// CorrelationHint 关联提示
type CorrelationHint struct {
    Type       string  // DocDecisionLink | TaskDecisionProgress | MeetingDecisionLink | PersonDecisionLink
    Confidence float64
    TargetSDR  string
    Description string
}

// ContextAssembler 上下文装配器
type ContextAssembler struct{}

// Assemble 在 token 预算内贪心装配上下文
func (a *ContextAssembler) Assemble(
    signal *StateChangeSignal,
    queryResults map[AdapterType][]any,
    existingDecisions []*decision.DecisionNode,
    budget int,
) *AssembledContext
```

### 5.3 决策状态机（对齐 signal-activation-engine.md §4）

```go
// internal/signal/state_machine.go

package signal

import "feishu-mem/internal/decision"

// StateTransition 状态转换
type StateTransition struct {
    SDRID        string
    FromStatus   decision.DecisionStatus
    ToStatus     decision.DecisionStatus
    Reason       string
    TriggerSignal string
}

// DecisionMutation 决策变更操作（传给 PipelineEngine）
type DecisionMutation struct {
    Type          MutationType              // create | update | status_change | conflict
    SDRID         string
    Node          *decision.DecisionNode    // create 时填充
    FieldChanges  map[string]any            // update 时填充
    NewStatus     decision.DecisionStatus   // status_change 时填充
    CommitMessage string
}

type MutationType string
const (
    MutationCreate       MutationType = "create"
    MutationUpdate       MutationType = "update"
    MutationStatusChange MutationType = "status_change"
    MutationConflict     MutationType = "conflict"
)

// DecisionStateMachine 决策状态机
type DecisionStateMachine struct{}

// EvaluateTransition 评估状态转换
func (sm *DecisionStateMachine) EvaluateTransition(
    current *decision.DecisionNode,
    signal *StateChangeSignal,
    ctx *AssembledContext,
) []*StateTransition
```

### 5.4 引擎编排器（对齐 signal-activation-engine.md §6）

```go
// internal/signal/engine.go

package signal

import (
    larkadapter "feishu-mem/internal/lark-adapter"
    "feishu-mem/internal/core"
)

// SignalActivationEngine 信号激活引擎编排器
type SignalActivationEngine struct {
    Emitters    map[AdapterType]StateChangeEmitter
    Router      *ActivationRouter
    Assembler   *ContextAssembler
    StateMachine *DecisionStateMachine
    Patterns    *PatternMatcher

    // 复用现有模块
    Pipeline  *core.PipelineEngine
    Memory    *core.MemoryGraph
}

// OnDetectResult 处理检测器返回的结果
func (e *SignalActivationEngine) OnDetectResult(
    adapter AdapterType,
    result *larkadapter.DetectResult,
) (*ProcessingReport, error) {
    // Step 1: 发射信号
    signal, err := e.Emitters[adapter].EmitSignal(result)
    if err != nil || signal == nil {
        return nil, nil // 不相关事件
    }

    // Step 2: 激活路由
    plan := e.Router.Route(signal)

    // Step 3: 执行查询 + 上下文装配
    ctx := e.executeAndAssemble(plan, signal)

    // Step 4: 匹配关联模式 + 评估状态转换
    mutations := e.evaluate(signal, ctx)

    // Step 5: 通过 PipelineEngine 执行变更
    return e.applyMutations(mutations, ctx), nil
}

// ProcessingReport 处理报告
type ProcessingReport struct {
    SignalID      string
    Mutations     []*DecisionMutation
    Conflicts     []Conflict
    TokenUsage    int
    DecisionCount int
}

// Conflict 冲突
type Conflict struct {
    ConflictID        string
    DecisionA         string
    DecisionB         string
    ContradictionScore float64
    Description       string
}
```

---

## 六、OpenClaw Core

### 6.1 PipelineEngine（流程引擎）

```go
// internal/core/pipeline.go

package core

import "feishu-mem/internal/signal"

// PipelineEngine 流程引擎 — 执行决策变更
type PipelineEngine struct {
    GitStorage     GitStorageInterface
    BitableStore   BitableStoreInterface
    MemoryGraph    *MemoryGraph
}

// GitStorageInterface Git 存储接口
type GitStorageInterface interface {
    WriteDecision(node *decision.DecisionNode) (commitHash string, err error)
    ReadDecision(project, topic, sdrID string) (*decision.DecisionNode, error)
    ListDecisions(project, topic string) ([]*decision.DecisionNode, error)
    // ...
}

// BitableStoreInterface Bitable 存储接口
type BitableStoreInterface interface {
    UpsertDecision(node *decision.DecisionNode) error
    QueryByTopic(topic, status string) ([]*decision.DecisionNode, error)
    QueryCrossTopic(topic string) ([]*decision.DecisionNode, error)
    // ...
}

// ApplyMutation 执行决策变更
func (pe *PipelineEngine) ApplyMutation(mutation *signal.DecisionMutation) error {
    switch mutation.Type {
    case signal.MutationCreate:
        // 1. 写入 Git
        hash, err := pe.GitStorage.WriteDecision(mutation.Node)
        if err != nil {
            return fmt.Errorf("git write failed: %w", err)
        }
        mutation.Node.GitCommitHash = hash

        // 2. 同步 Bitable
        if err := pe.BitableStore.UpsertDecision(mutation.Node); err != nil {
            return fmt.Errorf("bitable sync failed: %w", err)
        }

        // 3. 更新 MemoryGraph
        pe.MemoryGraph.UpsertDecision(mutation.Node)

    case signal.MutationUpdate:
        // 读取现有 → 合并字段 → 写回
        // ...

    case signal.MutationStatusChange:
        // 仅修改 status 字段
        // ...
    }
    return nil
}
```

### 6.2 ConflictResolver（冲突仲裁）

```go
// internal/core/conflict_resolver.go

package core

// ConflictResolver 冲突仲裁器
type ConflictResolver struct {
    LLMClient LLMClientInterface
}

// LLMClientInterface LLM 客户端接口
type LLMClientInterface interface {
    // EvaluateContradiction 语义矛盾评估
    EvaluateContradiction(a, b *decision.DecisionNode) (float64, error)

    // ClassifyTopic LLM 从内容判定议题归属
    ClassifyTopic(content string, topics []string) (string, float64, error)

    // JudgeCrossTopic 判定跨议题影响
    JudgeCrossTopic(node *decision.DecisionNode, candidateTopics []string) ([]string, error)
}

// ResolveResult 冲突解决结果
type ResolveResult struct {
    Action     string // no_conflict | relates | supersedes | conflict | blocked
    OldSDRID   string
    NewSDRID   string
    Reason     string
    NeedsUser  bool
}

// Resolve 解决新决策与现有决策的冲突（对齐 decision-tree.md §5.3）
func (cr *ConflictResolver) Resolve(
    newDecision *decision.DecisionNode,
    existingDecision *decision.DecisionNode,
) (*ResolveResult, error) {
    // Step 1: 权重比较
    newWeight := impactLevelWeight(newDecision.ImpactLevel)
    oldWeight := impactLevelWeight(existingDecision.ImpactLevel)

    // Step 2: LLM 语义矛盾评估
    score, err := cr.LLMClient.EvaluateContradiction(newDecision, existingDecision)
    if err != nil {
        return nil, err
    }

    // Step 3: 决策（对齐 decision-tree.md §5.3 表格）
    switch {
    case score < 0.3:
        return &ResolveResult{Action: "no_conflict"}, nil
    case score < 0.6:
        return &ResolveResult{Action: "relates"}, nil
    case newWeight > oldWeight && score > 0.6:
        return &ResolveResult{
            Action:   "supersedes",
            OldSDRID: existingDecision.SDRID,
            NewSDRID: newDecision.SDRID,
        }, nil
    case newWeight == oldWeight && score > 0.6:
        return &ResolveResult{
            Action:    "conflict",
            NeedsUser: true,
        }, nil
    }
    return nil, nil
}
```

---

## 七、存储层

### 7.1 GitStorage（对齐 git-operations-design.md）

```go
// internal/storage/git/git_storage.go

package git

import (
    "feishu-mem/internal/decision"
    "path/filepath"
)

// GitStorage Git 权威存储
type GitStorage struct {
    workDir string          // data/ 目录
    remote  string
    cli     *GitCLI         // 封装 git 命令
}

// GitCLI git 命令执行封装
type GitCLI struct{}

func (g *GitCLI) Run(args ...string) (string, error)

// === 完整接口（对齐 git-operations-design.md） ===

// WriteDecision 写入决策文件 + commit
func (gs *GitStorage) WriteDecision(node *decision.DecisionNode) (string, error) {
    // 1. 构建文件路径: data/decisions/{project}/{topic}/{sdr_id}.md
    path := filepath.Join(gs.workDir, "decisions", node.Project, node.Topic, node.SDRID+".md")

    // 2. 确保目录存在
    os.MkdirAll(filepath.Dir(path), 0755)

    // 3. 渲染 YAML frontmatter + Markdown → 写入文件
    content := RenderDecisionFile(node)
    os.WriteFile(path, []byte(content), 0644)

    // 4. git add + commit
    msg := gs.formatCommitMessage("decision", node)
    hash, err := gs.Commit(path, msg)
    if err != nil {
        return "", err
    }

    // 5. auto_push 远程
    if gs.config.AutoPush {
        gs.Push()
    }

    return hash, nil
}

// ReadDecision 读取决策文件
func (gs *GitStorage) ReadDecision(project, topic, sdrID string) (*decision.DecisionNode, error)

// ListDecisions 列出议题下所有决策
func (gs *GitStorage) ListDecisions(project, topic string) ([]*decision.DecisionNode, error)

// BlameDecision 逐行追溯
func (gs *GitStorage) BlameDecision(project, topic, sdrID string) ([]BlameEntry, error)

// SearchContent Git grep 全文搜索
func (gs *GitStorage) SearchContent(project, query string) ([]SearchHit, error)

// Commit 执行 git add + commit
func (gs *GitStorage) Commit(path, message string) (string, error)

// Push 推送到远程
func (gs *GitStorage) Push() error

// Pull 拉取远程最新
func (gs *GitStorage) Pull() error

// formatCommitMessage 格式化提交消息
func (gs *GitStorage) formatCommitMessage(typ string, node *decision.DecisionNode) string {
    return fmt.Sprintf("%s(%s): %s — %s\n\n决策: %s\n依据: %s\n影响: %s\n提出人: %s",
        typ, node.Topic, node.SDRID, node.Title,
        truncate(node.Decision, 100),
        truncate(node.Rationale, 100),
        strings.Join(node.CrossTopicRefs, ", "),
        node.Proposer,
    )
}

// RenderDecisionFile 渲染 YAML frontmatter + Markdown
func RenderDecisionFile(node *decision.DecisionNode) string
```

### 7.2 BitableStore（对齐 openclaw-architecture.md §2.4）

```go
// internal/storage/bitable/bitable_store.go

package bitable

import (
    "feishu-mem/internal/decision"
    "feishu-mem/internal/lark-adapter"
)

// BitableStore 飞书多维表格存储
type BitableStore struct {
    cli     *larkadapter.LarkCLI
    config  BitableConfig
}

type BitableConfig struct {
    BaseToken  string
    Tables     BitableTables
}

type BitableTables struct {
    Decision string // 决策表 ID
    Topic    string // 议题表 ID
    Phase    string // 阶段表 ID
    Relation string // 关系表 ID
}

// === CRUD ===

// UpsertDecision 插入或更新决策记录
func (bs *BitableStore) UpsertDecision(node *decision.DecisionNode) error {
    // lark-cli base +record-upsert \
    //   --base-token <token> \
    //   --table-id <decision_table> \
    //   --field-values '{...}'

    fieldValues := bs.nodeToFieldValues(node)
    _, err := bs.cli.RunCommand(
        "base", "+record-upsert",
        "--base-token", bs.config.BaseToken,
        "--table-id", bs.config.Tables.Decision,
        "--field-values", fieldValues,
    )
    return err
}

// QueryByTopic WHERE topic = "xxx" AND status = "active"
func (bs *BitableStore) QueryByTopic(topic, status string) ([]*decision.DecisionNode, error)

// QueryCrossTopic cross_topic_refs CONTAINS "xxx"
func (bs *BitableStore) QueryCrossTopic(topic string) ([]*decision.DecisionNode, error)

// QueryByPhase 按阶段过滤
func (bs *BitableStore) QueryByPhase(phase string) ([]*decision.DecisionNode, error)

// ListTopics 获取所有议题
func (bs *BitableStore) ListTopics() ([]TopicDef, error)

// VerifyConsistency 一致性校验
func (bs *BitableStore) VerifyConsistency() ([]SyncDrift, error)
```

### 7.3 Git ↔ Bitable 同步（对齐 git-operations-design.md §7）

```go
// internal/storage/bitable/sync.go

package bitable

import "time"

// SyncGitToBitable 正向同步：Git → Bitable
func (bs *BitableStore) SyncGitToBitable(git *GitStorage) error

// SyncBitableToGit 反向同步：Bitable → Git（用户手动修改 Bitable）
func (bs *BitableStore) SyncBitableToGit(git *GitStorage) error

// VerifyConsistency 一致性校验：对比 git_commit_hash
func (bs *BitableStore) VerifyConsistency(git *GitStorage) ([]SyncDrift, error) {
    records, _ := bs.getAllRecords()

    var drifts []SyncDrift
    for _, rec := range records {
        bitableHash := rec["git_commit_hash"].(string)
        gitHash := git.GetFileHash(rec["project"].(string), rec["topic"].(string), rec["sdr_id"].(string))

        if bitableHash != gitHash {
            drifts = append(drifts, SyncDrift{
                SDRID:       rec["sdr_id"].(string),
                BitableHash: bitableHash,
                GitHash:     gitHash,
                DetectedAt:  time.Now(),
            })
        }
    }
    return drifts, nil
}

type SyncDrift struct {
    SDRID       string
    BitableHash string
    GitHash     string
    DetectedAt  time.Time
}
```

---

## 八、MCP Server

### 8.1 Server 结构（对齐 memory-openclaw-integration.md §6.3）

```go
// internal/mcp/server.go

package mcp

import (
    "feishu-mem/internal/core"
    "net/http"
)

// MCPServer MCP 服务器 — 向 OpenClaw 暴露记忆查询工具
type MCPServer struct {
    port        int
    memoryGraph *core.MemoryGraph
    gitStorage  *git.GitStorage
    bitableStore *bitable.BitableStore
    httpServer  *http.Server
}

// NewMCPServer 创建 MCP Server
func NewMCPServer(port int, mg *core.MemoryGraph, gs *git.GitStorage, bs *bitable.BitableStore) *MCPServer {
    return &MCPServer{
        port:         port,
        memoryGraph:  mg,
        gitStorage:   gs,
        bitableStore: bs,
    }
}

// Start 启动 MCP Server
func (s *MCPServer) Start() error {
    mux := http.NewServeMux()

    // 注册 6 个工具端点
    mux.HandleFunc("/tools/search", s.handleSearch)
    mux.HandleFunc("/tools/topic", s.handleTopic)
    mux.HandleFunc("/tools/decision", s.handleDecision)
    mux.HandleFunc("/tools/timeline", s.handleTimeline)
    mux.HandleFunc("/tools/conflict", s.handleConflict)
    mux.HandleFunc("/tools/signal", s.handleSignal)

    // 健康检查
    mux.HandleFunc("/health", s.handleHealth)

    s.httpServer = &http.Server{
        Addr:    fmt.Sprintf(":%d", s.port),
        Handler: mux,
    }

    return s.httpServer.ListenAndServe()
}

// Stop 停止 MCP Server
func (s *MCPServer) Stop() error
```

### 8.2 工具实现

```go
// internal/mcp/search_tool.go

// handleSearch POST /tools/search
// 输入: { "query": "数据库迁移", "topic": "数据库架构", "limit": 10 }
// 输出: { "results": [...], "total": 3, "token_cost": 300 }
func (s *MCPServer) handleSearch(w http.ResponseWriter, r *http.Request) {
    var req SearchRequest
    json.NewDecoder(r.Body).Decode(&req)

    // 1. 先查 MemoryGraph（内存索引）
    candidates := s.memoryGraph.SearchByKeywords(req.Query, req.Topic)

    // 2. 再查 Bitable（跨议题）
    if req.Topic != "" {
        crossCandidates := s.bitableStore.QueryCrossTopic(req.Topic)
        candidates = append(candidates, crossCandidates...)
    }

    // 3. 去重 + 排序
    candidates = deduplicateAndRank(candidates, req.Query)

    // 4. 裁剪到 limit，返回摘要（不含完整正文）
    if len(candidates) > req.Limit {
        candidates = candidates[:req.Limit]
    }

    results := make([]SearchResult, len(candidates))
    for i, d := range candidates {
        results[i] = SearchResult{
            SDRID:         d.SDRID,
            Title:         d.Title,
            Topic:         d.Topic,
            ImpactLevel:   string(d.ImpactLevel),
            Status:        string(d.Status),
            Phase:         d.Phase,
            DecidedAt:     d.DecidedAt,
            RelevanceScore: 0.0, // computed by ranking
        }
    }

    json.NewEncoder(w).Encode(SearchResponse{
        Results:    results,
        Total:      len(results),
        TokenCost:  len(results) * 50, // ~50 tokens per result
    })
}

type SearchRequest struct {
    Query       string `json:"query"`
    Topic       string `json:"topic,omitempty"`
    ImpactLevel string `json:"impact_level,omitempty"`
    Status      string `json:"status,omitempty"`
    Limit       int    `json:"limit"`
}

type SearchResult struct {
    SDRID          string  `json:"sdr_id"`
    Title          string  `json:"title"`
    Topic          string  `json:"topic"`
    ImpactLevel    string  `json:"impact_level"`
    Status         string  `json:"status"`
    Phase          string  `json:"phase"`
    DecidedAt      *time.Time `json:"decided_at"`
    RelevanceScore float64 `json:"relevance_score"`
}

type SearchResponse struct {
    Results    []SearchResult `json:"results"`
    Total      int            `json:"total"`
    TokenCost  int            `json:"token_cost"`
}
```

其他 5 个工具实现同理：`handleTopic`, `handleDecision`, `handleTimeline`, `handleConflict`, `handleSignal`。

---

## 九、渐进式披露

### 9.1 索引层（对齐 memory-openclaw-integration.md §3.2）

```go
// internal/context/index.go

package context

// IndexContent 索引层内容（~800 tokens）
// OpenClaw 每次交互时自动注入
type IndexContent struct {
    ProjectStatus    string          `json:"project_status"`
    Topics           []TopicSummary  `json:"topics"`
    RecentDecisions  []DecisionDigest `json:"recent_decisions"`
    PendingItems     PendingItems    `json:"pending_items"`
    AvailableTools   []string        `json:"available_tools"`
    RefreshedAt      time.Time       `json:"refreshed_at"`
}

type TopicSummary struct {
    Name              string `json:"name"`
    ActiveCount       int    `json:"active_count"`
    LastChangeAt      string `json:"last_change_at"`
    ImpactDistribution string `json:"impact_distribution"` // "major:2, minor:3"
}

type DecisionDigest struct {
    SDRID       string `json:"sdr_id"`
    Title       string `json:"title"`
    ImpactLevel string `json:"impact_level"`
    Status      string `json:"status"`
}

type PendingItems struct {
    Conflicts int `json:"conflicts"`
    Signals   int `json:"signals"`
}

// GenerateIndex 从 MemoryGraph 生成索引
func GenerateIndex(mg *core.MemoryGraph) *IndexContent

// RenderAsMarkdown 渲染为 OpenClaw 使用的 Markdown
func (idx *IndexContent) RenderAsMarkdown() string
```

### 9.2 Token 预算控制

```go
// internal/context/budget.go

package context

// TokenBudget 各层的 Token 预算（对齐 decision-tree.md §3.2）
type TokenBudget struct {
    IndexLayer    int // ~800
    SearchResults int // ~500
    TopicDetail   int // ~1000
    DecisionYAML  int // ~300
    DecisionFull  int // ~800
    Timeline      int // ~600
    Conflict      int // ~400
    Signals       int // ~300
    MaxTotal      int // 8000（硬上限）
}

// DefaultBudget 默认预算（对齐 memory-openclaw-integration.md §10.3）
func DefaultBudget() TokenBudget {
    return TokenBudget{
        IndexLayer:    800,
        SearchResults: 500,
        TopicDetail:   1000,
        DecisionYAML:  300,
        DecisionFull:  800,
        Timeline:      600,
        Conflict:      400,
        Signals:       300,
        MaxTotal:      8000,
    }
}

// BudgetTracker 跟踪当前交互的 Token 消耗
type BudgetTracker struct {
    Budget  TokenBudget
    Used    int
    History []string // 已调用的工具
}

// CanUse 检查是否有足够的预算
func (bt *BudgetTracker) CanUse(tool string) bool {
    cost := bt.toolCost(tool)
    return bt.Used+cost <= bt.Budget.MaxTotal
}

// Consume 消耗预算
func (bt *BudgetTracker) Consume(tool string) {
    bt.Used += bt.toolCost(tool)
    bt.History = append(bt.History, tool)
}

func (bt *BudgetTracker) toolCost(tool string) int {
    switch tool {
    case "memory.search":
        return bt.Budget.SearchResults
    case "memory.topic":
        return bt.Budget.TopicDetail
    case "memory.decision":
        return bt.Budget.DecisionYAML
    case "memory.decision_full":
        return bt.Budget.DecisionFull
    case "memory.timeline":
        return bt.Budget.Timeline
    case "memory.conflict":
        return bt.Budget.Conflict
    case "memory.signal":
        return bt.Budget.Signals
    default:
        return 0
    }
}
```

---

## 十、配置结构

```go
// internal/config/settings.go

package config

// Settings 完整配置结构（对齐 openclaw-architecture.md §5.1）
type Settings struct {
    Project  ProjectConfig  `yaml:"project"`
    LarkCLI  LarkCLIConfig  `yaml:"lark_cli"`
    Git      GitConfig      `yaml:"git"`
    Bitable  BitableConfig  `yaml:"bitable"`
    Events   EventsConfig   `yaml:"events"`
    Polling  PollingConfig  `yaml:"polling"`
    MCP      MCPConfig      `yaml:"mcp"`
    Memory   MemoryConfig   `yaml:"memory"`
    LLM      LLMConfig      `yaml:"llm"`
}

type ProjectConfig struct {
    Name   string   `yaml:"name"`
    Topics []string `yaml:"topics"`
    Phases []string `yaml:"phases"`
}

type LarkCLIConfig struct {
    Bin             string `yaml:"bin"`
    DefaultIdentity string `yaml:"default_identity"` // user | bot
}

type GitConfig struct {
    WorkDir  string `yaml:"work_dir"`  // ./data
    Remote   string `yaml:"remote"`
    AutoPush bool   `yaml:"auto_push"`
    Branch   string `yaml:"branch"`

    ConsistencyCheck ConsistencyCheckConfig `yaml:"consistency_check"`
    Archive          ArchiveConfig          `yaml:"archive"`
    Maintenance      MaintenanceConfig      `yaml:"maintenance"`
}

type BitableConfig struct {
    BaseToken string       `yaml:"base_token"`
    Tables    TablesConfig `yaml:"tables"`
}

type MCPConfig struct {
    Port         int    `yaml:"port"`          // 37777
    RegisterPath string `yaml:"register_path"` // ~/.openclaw/openclaw.json
}

type MemoryConfig struct {
    PreloadOnStart         bool `yaml:"preload_on_start"`
    MaxCacheSize           int  `yaml:"max_cache_size"`
    DirtyFlushIntervalSecs int  `yaml:"dirty_flush_interval_secs"`
}
```

---

## 十一、MCP 配置注册

```json
// config/mcp.json — 部署到 ~/.openclaw/openclaw.json 的 MCP 部分
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

```javascript
// scripts/mcp-proxy.js — MCP 代理脚本
// 将 stdio MCP 协议转发到本地 HTTP API

const { Server } = require('@modelcontextprotocol/sdk/server/index.js');
const { StdioServerTransport } = require('@modelcontextprotocol/sdk/server/stdio.js');

const server = new Server({
  name: 'feishu-memory-mcp',
  version: '1.0.0',
}, {
  capabilities: { tools: {} }
});

// 注册 6 个工具
const TOOLS = [
  {
    name: 'memory.search',
    description: '搜索记忆系统中的决策记录。返回摘要列表（每条~50 tokens），不包含完整正文。',
    inputSchema: {
      type: 'object',
      properties: {
        query: { type: 'string', description: '搜索关键词' },
        topic: { type: 'string', description: '限定议题（可选）' },
        limit: { type: 'number', description: '返回数量，默认 10' }
      },
      required: ['query']
    }
  },
  // ... 其余 5 个工具定义
];

// 工具调用 → 转发到 localhost:37777
server.setRequestHandler('tools/call', async (request) => {
  const { name, arguments: args } = request.params;
  const response = await fetch(`http://localhost:37777/tools/${name.replace('memory.', '')}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(args)
  });
  return { content: [{ type: 'text', text: await response.text() }] };
});

const transport = new StdioServerTransport();
server.connect(transport);
```

---

## 十二、服务入口

```go
// cmd/mem-service/main.go

package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/joho/godotenv"
    "gopkg.in/yaml.v3"

    "feishu-mem/internal/config"
    larkadapter "feishu-mem/internal/lark-adapter"
    "feishu-mem/internal/core"
    "feishu-mem/internal/signal"
    "feishu-mem/internal/storage/git"
    "feishu-mem/internal/storage/bitable"
    "feishu-mem/internal/mcp"
    "feishu-mem/internal/context"
)

func main() {
    _ = godotenv.Load()

    // 1. 加载配置
    cfg := loadConfig("config/openclaw.yaml")

    // 2. 初始化 GitStorage
    gitStorage, err := git.NewGitStorage(cfg.Git)
    if err != nil {
        log.Fatalf("Git 初始化失败: %v", err)
    }

    // 3. 初始化 MemoryGraph（从 Git 加载全量决策）
    memoryGraph := core.NewMemoryGraph()
    if cfg.Memory.PreloadOnStart {
        if err := memoryGraph.LoadFromGit(gitStorage); err != nil {
            log.Printf("MemoryGraph 加载警告: %v", err)
        }
        log.Printf("MemoryGraph: 已加载 %d 条决策", memoryGraph.Count())
    }

    // 4. 初始化 BitableStore
    bitableStore := bitable.NewBitableStore(cfg.Bitable)

    // 5. 初始化 PipelineEngine
    pipeline := &core.PipelineEngine{
        GitStorage:   gitStorage,
        BitableStore: bitableStore,
        MemoryGraph:  memoryGraph,
    }

    // 6. 初始化 SignalActivationEngine
    engine := &signal.SignalActivationEngine{
        Emitters:    signal.NewEmitters(),     // 9 个 emitter
        Router:      signal.NewActivationRouter(),
        Assembler:   &signal.ContextAssembler{},
        StateMachine: &signal.DecisionStateMachine{},
        Patterns:    signal.NewPatternMatcher(),
        Pipeline:    pipeline,
        Memory:      memoryGraph,
    }

    // 7. 创建 9 个检测器
    adapterCfg := larkadapter.LoadConfig()
    detectors := map[signal.AdapterType]larkadapter.Detector{
        signal.AdapterIM:       larkadapter.NewIMExtractor(adapterCfg),
        signal.AdapterCalendar: larkadapter.NewCalendarExtractor(adapterCfg),
        signal.AdapterDocs:     larkadapter.NewDocExtractor(adapterCfg),
        signal.AdapterWiki:     larkadapter.NewWikiExtractor(adapterCfg),
        signal.AdapterVC:       larkadapter.NewVCExtractor(adapterCfg),
        signal.AdapterTask:     larkadapter.NewTaskExtractor(adapterCfg),
        signal.AdapterOKR:      larkadapter.NewOKRExtractor(adapterCfg),
        signal.AdapterContact:  larkadapter.NewContactExtractor(adapterCfg),
    }

    // 8. 启动 MCP Server
    mcpServer := mcp.NewMCPServer(cfg.MCP.Port, memoryGraph, gitStorage, bitableStore)
    go func() {
        log.Printf("MCP Server 启动: :%d", cfg.MCP.Port)
        if err := mcpServer.Start(); err != nil {
            log.Fatalf("MCP Server 错误: %v", err)
        }
    }()

    // 9. 启动检测主循环
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            runDetectionCycle(detectors, engine)
        }
    }()

    // 10. 启动定时维护
    go runMaintenanceTasks(gitStorage, bitableStore, cfg)

    // 11. 生成初始索引
    idx := context.GenerateIndex(memoryGraph)

    log.Println("========== feishu-agent-mem 就绪 ==========")
    log.Printf("  决策数:    %d", memoryGraph.Count())
    log.Printf("  议题数:    %d", memoryGraph.TopicCount())
    log.Printf("  MCP 端口:  :%d", cfg.MCP.Port)
    log.Printf("  索引刷新:  %s", idx.RefreshedAt.Format("15:04:05"))
    log.Println("=============================================")

    // 12. 等待退出信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    ticker.Stop()
    mcpServer.Stop()
    log.Println("feishu-agent-mem 已停止")
}

// runDetectionCycle 执行一轮检测 → 信号 → 处理
func runDetectionCycle(
    detectors map[signal.AdapterType]larkadapter.Detector,
    engine *signal.SignalActivationEngine,
) {
    stateMgr := larkadapter.NewStateManager(filepath.Join(larkadapter.StateDir(), "detect_state.json"))

    for adapter, detector := range detectors {
        lastCheck := stateMgr.GetLastCheck(detector.Name())

        result, err := larkadapter.ExtractDetect(detector)
        if err != nil {
            log.Printf("[%s] 检测失败: %v", detector.Name(), err)
            continue
        }

        if result.HasChanges {
            // 通过 SignalActivationEngine 处理
            report, err := engine.OnDetectResult(adapter, result)
            if err != nil {
                log.Printf("[%s] 信号处理失败: %v", detector.Name(), err)
                continue
            }
            if report != nil {
                log.Printf("[%s] %d 变更 → %d 决策变更, %d 冲突",
                    detector.Name(), len(result.Changes),
                    len(report.Mutations), len(report.Conflicts))
            }
        }
    }
}
```

---

## 十三、Docker 集成

### 13.1 Dockerfile

```dockerfile
# Dockerfile — 基于 openclaw-zh 镜像
FROM openclaw-zh:latest

# Go 运行时依赖（如果基础镜像没有）
RUN apt-get update && apt-get install -y git ca-certificates

# 创建工作目录
WORKDIR /opt/feishu-agent-mem

# 复制二进制和配置
COPY bin/mem-service /opt/feishu-agent-mem/
COPY config/ /opt/feishu-agent-mem/config/
COPY scripts/mcp-proxy.js /opt/feishu-agent-mem/scripts/

# MCP 注册配置
COPY config/mcp.json /root/.openclaw/openclaw.json

# 数据卷（Git 仓库 + 决策文件）
VOLUME ["/opt/feishu-agent-mem/data"]

# 暴露 MCP HTTP 端口
EXPOSE 37777

# 启动记忆系统服务
CMD ["/opt/feishu-agent-mem/mem-service"]
```

### 13.2 Makefile

```makefile
# Makefile

.PHONY: build test run docker-build deploy clean

BINARY := bin/mem-service

build:
	@echo "Building..."
	go build -o $(BINARY) ./cmd/mem-service/

test:
	go test ./internal/... ./test/... -v -count=1

test-detector:
	go test ./test/detector/... -v

run: build
	./$(BINARY)

docker-build:
	docker build -t feishu-agent-mem:latest .

docker-run:
	docker run -d \
		--name feishu-agent-mem \
		-p 37777:37777 \
		-v $(PWD)/data:/opt/feishu-agent-mem/data \
		-v $(PWD)/config:/opt/feishu-agent-mem/config \
		--env-file .env \
		feishu-agent-mem:latest

deploy: build docker-build docker-run
	@echo "Deployed"

clean:
	rm -f $(BINARY)
	rm -rf data/
```

---

## 十四、模块依赖关系图

```
cmd/mem-service/main.go
│
├─ internal/config/         # 配置加载
├─ internal/context/        # 索引生成 + 预算控制
├─ internal/mcp/            # MCP Server
│    ├─ internal/core/      #   MemoryGraph (查询)
│    ├─ internal/storage/   #   GitStorage, BitableStore (查询)
│    └─ internal/context/   #   预算控制
│
├─ internal/core/           # OpenClaw Core
│    ├─ internal/decision/  #   DecisionNode 数据结构
│    └─ internal/storage/   #   GitStorage, BitableStore 接口
│
├─ internal/signal/         # Signal Activation Engine
│    ├─ internal/lark-adapter/ #   Detector 接口
│    ├─ internal/core/         #   PipelineEngine, MemoryGraph
│    └─ internal/decision/     #   DecisionNode
│
├─ internal/storage/git/    # Git Storage
│    └─ internal/decision/  #   DecisionNode
│
├─ internal/storage/bitable/# Bitable Store
│    ├─ internal/lark-adapter/ #   LarkCLI (lark-cli exec)
│    └─ internal/decision/     #   DecisionNode
│
└─ internal/lark-adapter/   # 飞书适配器层 (Detector + Extractor)
```

---

## 十五、实施优先级

| Phase | 模块 | 说明 | 依赖 |
|-------|------|------|------|
| **P0** (已有) | `internal/lark-adapter/` | 9 个检测器、Extractor/Detector 接口 | — |
| **P1** | `internal/decision/` | DecisionNode 数据结构 | — |
| **P1** | `internal/storage/git/` | Git 读写、commit、目录初始化 | P1 |
| **P1** | `internal/core/memory_graph.go` | 内存索引、从 Git 加载 | P1 |
| **P2** | `internal/storage/bitable/` | Bitable CRUD、结构化查询 | P1, linux-adapter |
| **P2** | `internal/signal/` | 信号引擎全部模块 | P1, P2 |
| **P2** | `internal/core/pipeline.go` | 流程编排 | P1, P2 |
| **P3** | `internal/mcp/` | MCP Server 6 个工具 | P1, P2 |
| **P3** | `internal/context/` | 索引层 + 预算控制 | P1 |
| **P3** | `internal/core/conflict_resolver.go` | LLM 冲突仲裁 | P1 |
| **P4** | `cmd/mem-service/` | 服务入口 + 主循环 | P1-P3 |
| **P4** | `Dockerfile`, `Makefile` | 构建部署 | P4 |

---

## 十六、LLM 系统提示词

记忆系统在以下 4 个场景调用 LLM，每次调用必须附带对应的系统提示词（system prompt）。提示词设计原则（对齐 agent 集成设计价值观.md）：

- **静态段 + 动态段**：静态段为固定的角色和规则，动态段注入当前议题列表、候选决策、信号内容
- **指令极其具体**：告诉 LLM 何时该输出、何时该跳过、何时该请求更多信息
- **预算控制**：每次调用限定在 ~2K-6K tokens 内

### 16.1 决策提取提示词（从飞书内容提取决策）

场景：信号引擎检测到 IM 消息/会议纪要/文档评论中含决策信号，需要从内容中提取结构化决策。

```
# 系统提示词：决策提取器

## 角色
你是一个项目决策提取专家。从飞书群聊消息、会议纪要、文档内容中识别和提取决策性信息。

## 任务
给定一段飞书内容，判断是否包含决策，如果是，提取为结构化格式。

## 决策识别信号（对齐 decision-extraction.md §3.1）
- 中文: "决定"、"确认"、"结论"、"通过"、"定下来"、"就这么办"、"不再讨论"、"最终方案"
- 英文: "approve"、"LGTM"、"decided"、"confirmed"、"agreed"
- Pin 消息自动视为高价值锚点
- 会议纪要中的"待办"/"Action Items"自动提取
- 文档评论中的 "/approve" 或"同意"视为审批信号

## 决策状态判定
- 仅有讨论但没有结论 → status: "in_discussion"
- 已有明确结论 → status: "decided"
- 新发现但尚未讨论的决策信号 → status: "pending"

## 输出格式
仅当 confidence >= 0.6 时输出，否则返回 {"has_decision": false}。

{\n  \"has_decision\": true/false,
  \"confidence\": 0.0-1.0,
  \"decision\": {
    \"title\": \"一句话决策标题\",
    \"decision\": \"决策结论（从原文或对话中精确引用）\",
    \"rationale\": \"决策依据（从讨论中提取 1-2 条理由）\",
    \"suggested_topic\": \"建议归属的议题（从候选列表中选择）\",
    \"impact_level\": \"advisory|minor|major|critical\",
    \"phase_scope\": \"Point|Span|Retroactive\",
    \"proposer\": \"提出者姓名\",
    \"executor\": \"执行者姓名（如果提到）\",
    \"related_entities\": {  // 关联的飞书实体
      \"chat_ids\": [], \"doc_tokens\": [], \"meeting_ids\": [],
      \"task_guids\": [], \"event_ids\": []
    }
  },
  \"extracted_from\": \"消息/会议/文档的摘要（< 100 字）\"
}

## 规则
- confidence < 0.6 不输出
- 如果讨论中提到多个方案但未选出一个 → status: "in_discussion"
- 如果是日常闲聊（"今天吃什么"）> confidence: 0.0
- 如果是技术讨论但无决策结论 → has_decision: false
- 不要在决策字段中编造原文没有的内容
```

### 16.2 议题归属提示词（判定决策属于哪个 Topic）

场景：新决策提取后，需要 LLM 判定其归属议题。对齐 decision-tree.md §1.2 — Topic 是决策唯一的树位置锚点。

```
# 系统提示词：议4分类器

## 角色
你是一个项目分类专家。将决策归类到正确的议题（Topic）。

## 任务
给定一个决策内容和候选议题列表，选择最匹配的议题。

## 候选议题列表
{动态注入: 从 Bitable Topic 表获取的 topic_id + topic_name + topic_description}

## 决策内容
{动态注入: 决策标题 + 决策结论 + 决策依据 + 来源摘要}

## 输出格式
{
  "topic": "匹配的议题名称",
  "confidence": 0.0-1.0,
  "reasoning": "一句话解释为什么匹配该议题",
  "alternative_topics": ["备选议题1", "备选议题2"]  // 仅 confidence < 0.7 时输出
}

## 规则
- confidence >= 0.8 直接输出，不输出 alternative_topics
- confidence < 0.7 时必须输出 1-2 个 alternative_topics，供进一步确认
- confidence < 0.5 返回 null，不要猜测
- 优先匹配技术模块名（如 "用户服务" > "通用"）
- 如果决策涉及基础设施/数据库/安全 → 优先匹配对应的技术议题
```

### 16.3 跨议题检测提示词（判定决策是否影响其他议题）

场景：决策归属确定后，LLM 判断是否需要标记 cross_topic_refs。对齐 decision-tree.md §4.5-4.6。

```
# 系统提示词：跨议题影响检测器

## 角色
你是一个技术依赖分析专家。判断一个决策是否会影响其所属议题之外的其他议题。

## 任务
给定决策内容、所属议题、候选议题列表，评估是否跨议题，如果是，列出受影响的其他议题。

## 候选议题列表
{动态注入: 排除自身 Topic 后的所有 Topic 列表}

## 决策信息
- 所属议题: {topic}
- 决策标题: {title}
- 决策结论: {decision}
- 决策依据: {rationale}
- 影响级别: {impact_level}

## 判定维度（对齐 decision-tree.md §4.5）

| 维度 | 信号 | 权重 |
|------|------|------|
| 决策类型 | 基础设施/架构/安全类 → Cross | 高 |
|          | 功能实现/UI/文档类 → Single | |
| 模块名提及 | 明确提到其他议题的模块名 | 中 |
| 技术依赖 | 数据库 Schema 变更 → 所有依赖它的服务 | 高 |
|          | API 协议变更 → 所有调用方 | |
|          | 安全策略变更 → 所有受影响模块 | |

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
```

### 16.4 冲突评估提示词（判定两个决策是否语义矛盾）

场景：新决策插入时，需要与现有决策做语义矛盾评估。对齐 decision-tree.md §5.3。

```
# 系统提示词：决策冲突评估器

## 角色
你是一个技术决策一致性检查专家。评估两个决策是否存在语义矛盾。

## 任务
给定两个决策的内容，判断它们是否矛盾，给出矛盾分数。

## 决策 A（新决策）
{动态注入: 标题 + 决策结论 + 决策依据}

## 决策 B（已有决策）
{动态注入: 标题 + 决策结论 + 决策依据}

## 矛盾类型
1. **直接矛盾**: 两个决策的结论直接矛盾
   例: A 说"用 PostgreSQL"，B 说"用 MySQL" → 分数 0.9-1.0

2. **参数矛盾**: 两个决策对同一参数规定了不同值
   例: A 说"token 长度 256"，B 说"token 长度 512" → 分数 0.7-0.9

3. **时序矛盾**: 两个决策的执行顺序不可调和
   例: A 说"先迁数据库再改 API"，B 说"先改 API 再迁数据库" → 分数 0.5-0.7

4. **范围矛盾**: 一个决策的范围与另一个决策重叠但有冲突
   分数 0.3-0.5

5. **无矛盾**: 两个决策互不影响
   分数 < 0.3

## 输出格式
{
  "contradiction_score": 0.0-1.0,
  "contradiction_type": "direct|param|timing|scope|none",
  "description": "一句话描述矛盾点",
  "suggestion": "如果需要调整某个决策才能共存，给出建议"
}

## 规则（对齐 decision-tree.md §5.3）
- score < 0.3: 无冲突，正常插入
- score 0.3-0.6: 标记 RELATES_TO 关系
- score > 0.6: 需要进一步处理（SUPERSEDES 或 CONFLICTS_WITH）
- 只评估技术事实的矛盾，不评估"谁对谁错"
- 如果两个决策在不同阶段生效（phase 不同），矛盾应降级
```

### 16.5 提示词管理代码

```go
// internal/llm/prompts.go

package llm

// PromptManager 管理所有 LLM 系统提示词
type PromptManager struct {
    prompts map[string]*PromptTemplate
}

// PromptTemplate 提示词模板（静态段 + 动态插入点）
type PromptTemplate struct {
    Name        string
    Static      string            // 静态段（角色 + 规则 + 格式）
    MaxTokens   int               // 最大 token 数
    Temperature float64           // 0.0-1.0
}

// BuildPrompt 构建完整提示词（静态段 + 动态段），控制在预算内
func (pm *PromptManager) BuildPrompt(name string, dynamicData map[string]string, budget int) (string, error)
```

---

## 十七、OpenClaw 24 小时运转机制

### 17.1 双进程模型

系统采用双进程协同运转，各司其职：

```
┌──────────────────────────────────────────────────────────────┐
│                     Docker: openclaw-zh                       │
│                                                               │
│  ┌─────────────────────┐    ┌─────────────────────────────┐  │
│  │   OpenClaw Agent     │    │   feishu-agent-mem (mem-service) │
│  │   (OpenClaw 平台托管) │    │   (自主后台服务)               │  │
│  │                      │    │                               │  │
│  │  • 飞书对话交互       │    │  • 检测循环 (每30s)           │  │
│  │  • LLM 推理           │◄──►│  • 信号引擎                    │  │
│  │  • 用户指令响应       │ MCP│  • Git + Bitable 持久化        │  │
│  │  • lark-cli 写操作    │    │  • 定时轮询 + 一致性校验        │  │
│  │                      │    │  • 索引刷新                    │  │
│  └──────────────────────┘    └─────────────────────────────┘  │
│         │                              │                       │
│         │        lark-cli              │        lark-cli       │
│         └──────────────┬───────────────┘                       │
│                        │                                       │
│                  飞书 OpenAPI                                   │
└──────────────────────────────────────────────────────────────┘
```

### 17.2 运转时间线（24 小时）

```
00:00 ────────────────────────────────────────────────────────── 24:00

持续运行（每 30s）:
  ┌─────────────────────────────────────────────────────────┐
  │  Detection Loop: 9 个检测器 → 信号 → 装配 → 持久化       │
  └─────────────────────────────────────────────────────────┘

持续运行（WebSocket 长连接）:
  ┌─────────────────────────────────────────────────────────┐
  │  Event Stream: lark-cli event +subscribe                 │
  │  IM / VC / Docs / Calendar / Task 事件实时推送           │
  └─────────────────────────────────────────────────────────┘

每日 09:00:
  ├── 日程预识别（calendar +agenda）
  └── 刷新 topic-index.md

每日 18:00:
  ├── 今日消息回顾（im +chat-messages-list）
  └── 今日会议回顾（vc +search）

每周一 09:00:
  ├── 上周文档回顾（docs +search）
  ├── 上周会议回顾（vc +search）
  └── 上周任务回顾（task +get-related-tasks）

每 30 分钟:
  └── Git ↔ Bitable 一致性校验

每月 1 日 02:00:
  ├── git gc --aggressive
  └── 归档检查

OpenClaw Agent 按需:
  ├── 用户在飞书对话中询问 → OpenClaw 通过 MCP 查询记忆系统
  ├── 冲突需人工裁决 → OpenClaw 通过飞书消息通知用户
  └── 用户主动指令 → OpenClaw 调用 lark-cli 执行写操作
```

### 17.3 故障自愈

```go
// internal/core/supervisor.go

package core

// Supervisor 进程守护 — 确保 mem-service 持续运行
type Supervisor struct {
    HealthCheckInterval time.Duration // 健康检查间隔
    MaxRestarts         int           // 最大重启次数
    RestartBackoff      time.Duration // 重启退避
}

// 自愈场景:
//
// 1. 检测器失败 ── 单个检测器失败不影响其他检测器
//    每个检测器有独立的 error handling，失败时记录日志并跳过
//
// 2. Git 操作失败 ── 写入队列缓存，重试 3 次
//    失败超过 3 次 → 降级为仅 Bitable 写入 + 告警
//
// 3. Bitable 同步失败 ── 缓存未同步记录
//    下次一致性校验时自动修复
//
// 4. MCP Server 崩溃 ── goroutine panic recovery
//    自动重启 MCP Server（不影响检测循环）
//
// 5. 进程崩溃 ── Docker restart policy: always
//    docker-compose 或 systemd 自动拉起
//
// 6. LLM 调用超时 ── 30s 超时 + 重试 1 次
//    仍然超时 → 降级为规则判断（不依赖 LLM）
```

### 17.4 OpenClaw Agent 与 mem-service 的交互节奏

```
mem-service（自主运行）              OpenClaw Agent（按需介入）
─────────────────────────           ─────────────────────────

每 30s: 检测循环                      用户消息到达时:
  ├── 9 个检测器执行                      ├── 注入索引层 (~800 tokens)
  ├── 发射信号                            ├── 静默 memory.search
  ├── 低强度信号 → 自动处理（不通知）      ├── LLM 推理
  └── 高强度信号 → 入通知队列              └── 按需调用 memory.topic/decision

每 5min: 通知批处理                    冲突需裁决时:
  ├── 汇总高强度信号                       ├── 通过飞书消息通知用户
  └── 推送精简通知包到 OpenClaw           └── 等待用户指令

每 30min: 一致性校验                   用户主动查询时:
  └── Git ↔ Bitable 对比                 ├── "数据库迁移进展如何？"
                                        ├── OpenClaw 查询记忆系统
                                        └── 基于精简上下文回答
```

---

## 十八、仓库挂载与 OpenClaw 运行机制

### 18.1 目录映射

代码仓库在宿主机 `~/openclaw-workspace/feishu-agent-mem` 通过 Docker volume 挂载到容器内 `/root/openclaw-workspace/feishu-agent-mem`：

```
宿主机                                       Docker 容器 (openclaw-zh)
──────                                       ─────────────────────────

~/openclaw-workspace/                        /root/openclaw-workspace/
├── feishu-agent-mem/          ──volume──►   ├── feishu-agent-mem/
│   ├── cmd/mem-service/                     │   ├── cmd/mem-service/
│   ├── internal/                            │   ├── internal/
│   ├── config/                              │   ├── config/
│   │   ├── openclaw.yaml                    │   │   ├── openclaw.yaml
│   │   └── mcp.json                         │   │   └── mcp.json
│   ├── data/                  ──volume──►   │   ├── data/
│   │   └── decisions/                       │   │   └── decisions/
│   ├── outputs/                             │   ├── outputs/
│   ├── scripts/                             │   │   ├── mcp-proxy.js
│   │   ├── start.sh                         │   │   └── start.sh
│   │   └── mcp-proxy.js                     │   ├── go.mod
│   ├── go.mod                               │   ├── bin/mem-service (编译产物)
│   ├── .env                                 │   └── .env
│   └── Makefile                             │
│                                            ├── .openclaw/
│                                            │   └── openclaw.json ← MCP 注册
│                                            └── .openclaw/plugins/
└── tmp/                                       └── memory-plugin/ (OpenClaw 接口层)
```

### 18.2 OpenClaw 启动该系统的方式

OpenClaw 通过以下机制启动和管理 mem-service：

```
方式一: OpenClaw 启动时通过 Hook 拉起（推荐）
────────────────────────────────────────────

OpenClaw 配置文件 ~/.openclaw/openclaw.json 中注册 SessionStart Hook:

{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "command",
            "command": "/root/openclaw-workspace/feishu-agent-mem/scripts/start.sh"
          }
        ]
      }
    ]
  }
}

start.sh 脚本:
  #!/bin/bash
  # 检查 mem-service 是否已在运行
  if ! pgrep -f "mem-service" > /dev/null; then
    cd /root/openclaw-workspace/feishu-agent-mem
    # 编译（如果需要）
    go build -o bin/mem-service ./cmd/mem-service/
    # 后台启动
    nohup ./bin/mem-service > logs/mem-service.log 2>&1 &
    echo "feishu-agent-mem started (PID: $!)"
  fi

方式二: Docker Compose 双容器（备选）
─────────────────────────────────────

docker-compose.yml:
  services:
    openclaw:
      image: openclaw-zh:latest
      volumes:
        - ~/openclaw-workspace:/root/openclaw-workspace
      ports:
        - "8080:8080"
      depends_on:
        - mem-service

    mem-service:
      build:
        context: ~/openclaw-workspace/feishu-agent-mem
      volumes:
        - ~/openclaw-workspace/feishu-agent-mem/data:/opt/feishu-agent-mem/data
      ports:
        - "37777:37777"

方式三: OpenClaw Skill 触发启动（现有环境）
────────────────────────────────────────

OpenClaw 的 memory-interface skill 在首次调用时触发:
  1. OpenClaw 加载 memory-interface skill
  2. Skill 的 onLoad hook 执行，检查 localhost:37777 是否可达
  3. 不可达 → 执行 start.sh 拉起 mem-service
  4. 可达 → 直接通过 MCP 工具通信

注意: Skill 只是接口层，不包含记忆系统逻辑。
      Skill 的职责 = 触发启动 + MCP 调用封装。
```

### 18.3 OpenClaw 如何"运行"整个系统

关键理解：**OpenClaw 不直接运行记忆系统**。两者的关系是：

```
OpenClaw Agent                    feishu-agent-mem
─────────────────                 ─────────────────

角色: 交互层                       角色: 持久化层
┌──────────────────┐              ┌──────────────────────┐
│ 用户 ←→ 飞书对话  │              │ 飞书 ←→ 事件检测     │
│ LLM 推理          │              │ Git ←→ 决策文件      │
│ 决策执行（写操作） │              │ Bitable ←→ 结构化查询 │
│ 用户通知          │              │ 信号引擎             │
└──────┬───────────┘              └──────────┬───────────┘
       │                                     │
       │  ┌──────────────────────────────┐   │
       └──│  MCP Protocol (localhost)    │───┘
          │  memory.search / .topic /    │
          │  .decision / .timeline /     │
          │  .conflict / .signal         │
          └──────────────────────────────┘
```

OpenClaw 的具体运作方式：

```
1. 启动阶段
   OpenClaw Agent 启动
   → SessionStart Hook 触发
   → start.sh 确保 mem-service 运行
   → MCP Client 连接 localhost:37777
   → 注入记忆系统索引层到 Agent 上下文 (~800 tokens)

2. 用户交互阶段
   用户: "数据库迁移进展如何？"
   → OpenClaw 已有索引层上下文
   → 静默调用 memory.search("数据库迁移")  → mem-service 返回 3 条摘要
   → LLM 推理: 需要知道详情
   → 调用 memory.decision("dec_xxx", include_full=true) → 返回完整内容
   → LLM 组装回答

3. 自主检测阶段（无用户交互）
   mem-service 每 30s 检测循环
   → 检测到 IM 群聊含决策 "用 Redis 做缓存"
   → 发射信号 → 路由 → 装配 → 状态机 → 创建候选决策
   → 决策存入 Git + Bitable
   → 入通知队列

   每 5min 批处理通知队列
   → 发现高优先级信号 → 推送到 OpenClaw
   → OpenClaw 收到通知 → 决定是否需要 LLM 处理
   → 如果需要 → 通过 MCP 获取完整上下文 → LLM 判断

4. 冲突裁决阶段
   mem-service 检测到冲突（contradiction_score > 0.6）
   → 创建 CONFLICTS_WITH 关系
   → 推送通知到 OpenClaw: "决策 dec_A 与 dec_B 冲突，需要裁决"
   → OpenClaw 收到:
     a. 调用 memory.decision("dec_A") + memory.decision("dec_B")
     b. LLM 分析冲突点
     c. 通过飞书消息通知用户:
        "检测到决策冲突:
         - 认证模块: session_token 需要 1024 字符
         - 数据库架构: session_token 分配了 512 字符
         请裁决: 调整 Schema 还是调整认证方案？"
     d. 等待用户回复 → 执行裁决
```

### 18.4 代码仓库的目录结构（容器内视角）

```
/root/openclaw-workspace/feishu-agent-mem/
│
├── cmd/mem-service/main.go        # 服务入口
├── internal/                      # 所有 Go 源码
│   ├── decision/                  # 数据模型
│   ├── lark-adapter/              # 飞书适配器
│   ├── signal/                    # 信号引擎
│   ├── core/                      # OpenClaw Core
│   ├── storage/git/               # Git 存储
│   ├── storage/bitable/           # Bitable 存储
│   ├── mcp/                       # MCP Server
│   ├── llm/                       # LLM 客户端 + 提示词
│   ├── context/                   # 渐进式披露
│   └── config/                    # 配置加载
│
├── config/
│   ├── openclaw.yaml              # 主配置
│   └── mcp.json                   # MCP 注册
│
├── data/                          # Git 工作目录
│   ├── decisions/                 # 决策文件
│   │   └── {project}/{topic}/{sdr_id}.md
│   ├── conflicts/
│   ├── archive/
│   └── L0_RULES.md
│
├── outputs/                       # 提取输出 + 状态文件
│   ├── *.json
│   └── detect_state.json
│
├── scripts/
│   ├── start.sh                   # 启动脚本
│   ├── health_check.sh            # 健康检查
│   └── mcp-proxy.js               # MCP 协议代理
│
├── logs/                          # 日志
│   └── mem-service.log
│
├── go.mod
├── go.sum
├── Makefile
└── .env
```

### 18.5 OpenClaw 的 startup.sh 完整实现

```bash
#!/bin/bash
# /root/openclaw-workspace/feishu-agent-mem/scripts/start.sh
# OpenClaw SessionStart Hook 调用此脚本启动记忆系统

set -e

MEM_DIR="/root/openclaw-workspace/feishu-agent-mem"
LOG_DIR="$MEM_DIR/logs"
PID_FILE="$MEM_DIR/mem-service.pid"

mkdir -p "$LOG_DIR"

# 检查是否已在运行
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        echo "[$(date)] mem-service already running (PID: $OLD_PID)"
        exit 0
    fi
fi

echo "[$(date)] Starting feishu-agent-mem..."

# 加载环境变量
if [ -f "$MEM_DIR/.env" ]; then
    export $(grep -v '^#' "$MEM_DIR/.env" | xargs)
fi

# 检查 lark-cli 可用性
if ! command -v lark-cli &> /dev/null; then
    echo "[$(date)] ERROR: lark-cli not found" >> "$LOG_DIR/mem-service.log"
    exit 1
fi

# 编译（首次或源码有变更时）
cd "$MEM_DIR"
if [ ! -f "bin/mem-service" ] || [ "cmd/mem-service/main.go" -nt "bin/mem-service" ]; then
    echo "[$(date)] Building mem-service..." >> "$LOG_DIR/mem-service.log"
    go build -o bin/mem-service ./cmd/mem-service/ 2>> "$LOG_DIR/mem-service.log"
fi

# 后台启动
nohup ./bin/mem-service >> "$LOG_DIR/mem-service.log" 2>&1 &
PID=$!
echo $PID > "$PID_FILE"

echo "[$(date)] feishu-agent-mem started (PID: $PID)" >> "$LOG_DIR/mem-service.log"

# 注册退出清理
trap "rm -f $PID_FILE; echo '[$(date)] feishu-agent-mem stopped'" EXIT
```

---

## 十九、与设计文档的完整对齐

| 本文档章节 | 对应设计文档 | 对齐的具体内容 |
|-----------|------------|--------------|
| §3.1 DecisionNode | decision-tree.md §2.1 | 所有字段一一对应，impact_level 取代 tree_level |
| §3.3 StateChangeSignal | signal-activation-engine.md §1 | 标准信号协议，信号强度判定规则 |
| §4 适配器层 | decision-extraction.md §1-4 | 8 个适配器保持不变，扩展 Emitter |
| §5 信号引擎 | signal-activation-engine.md §2-6 | 权重矩阵、路由算法、装配器、状态机 |
| §6.1 PipelineEngine | openclaw-architecture.md §2.1.1 | 三阶段流水线 |
| §6.2 ConflictResolver | decision-tree.md §5.3 | 权重比较 + 语义矛盾 → 5 种动作 |
| §7.1 GitStorage | git-operations-design.md §1-15 | 完整 Git 操作接口 |
| §7.2 BitableStore | openclaw-architecture.md §2.4 | CRUD + 结构化查询 |
| §7.3 同步协议 | git-operations-design.md §7 | git_commit_hash 锚点 + 双向同步 |
| §8 MCP Server | memory-openclaw-integration.md §6.3 | 6 个渐进式披露工具 |
| §9 渐进式披露 | memory-openclaw-integration.md §3 | 三层信息架构 |
| §10 配置 | openclaw-architecture.md §5.1 | openclaw.yaml 配置结构 |
| §12 服务入口 | openclaw-architecture.md §5.3 | 启动流程 8 步 |
| §13 Docker 集成 | memory-openclaw-integration.md §11 | 部署架构 |
| §16.1 决策提取提示词 | decision-extraction.md §3.1, §7 | 决策信号识别、提取工作流 |
| §16.2 议题归属提示词 | decision-tree.md §1.2 | Topic 是唯一位置锚点 |
| §16.3 跨议题提示词 | decision-tree.md §4.5-4.6 | 跨议题判定维度 + LLM 推理流程 |
| §16.4 冲突评估提示词 | decision-tree.md §5.3 | 矛盾类型 + 分数阈值 |
| §17 24h运转机制 | openclaw-architecture.md §4, signal-activation-engine.md §6 | 事件驱动+轮询、故障恢复 |
| §18 仓库挂载与运行 | memory-openclaw-integration.md §11 | Docker 部署 + OpenClaw 启动 |
| agent 错误处理 | agent 集成设计价值观.md | 死循环检测、检查点、兜底策略 |
| agent 工具设计 | agent 集成设计价值观.md | 专用工具、预算约束、两级加载 |
