# 多源决策提取设计方案

基于现有 `lark-im` 检测器的实现模式，扩展信号引擎以支持从飞书全功能域（VC、Docs、Calendar、Task、Contact、Wiki）提取决策信息。

---

## 1. 现状分析

### 1.1 已实现

| 组件 | 状态 | 位置 |
|------|------|------|
| IMExtractor (Detector) | ✅ 完整实现 | `internal/lark-adapter/lark_im.go` |
| IMEmitter | ✅ 有关键词匹配逻辑 | `internal/signal/emitter.go:29-98` |
| WorkerPool.processJob() | ✅ 仅处理 `new_text`/`new_post` | `internal/signal/worker.go:96-142` |
| SignalActivationEngine | ✅ LLM 提取 + fallback | `internal/signal/engine.go:51-111` |
| PipelineEngine | ✅ Git + Bitable + MemoryGraph | `internal/core/pipeline.go` |
| StateManager | ✅ 多源时间戳追踪 | `internal/lark-adapter/types.go:72-150` |

### 1.2 待实现（Stub）

| 适配器 | Emitter 状态 | Detector 状态 | 决策价值 |
|--------|-------------|--------------|---------|
| VC (视频会议) | Stub → StrengthMedium | 未实现 | **高** — AI 纪要直接含决策结论和待办 |
| Docs (云文档) | Stub → StrengthMedium | 未实现 | **高** — 正式决策文档、评论中的审批 |
| Calendar (日程) | Stub → StrengthMedium | 未实现 | **中** — 决策评审会议识别 |
| Task (任务) | Stub → StrengthMedium | 未实现 | **中** — 决策执行进度追踪 |
| Contact (通讯录) | Stub → StrengthWeak | 未实现 | **低** — 辅助角色解析 |
| Wiki (知识库) | Stub → StrengthMedium | 未实现 | **中** — 已沉淀决策结构 |
| OKR | Stub → StrengthMedium | 未实现 | **低** — 战略对齐参考 |

---

## 2. 架构总览

```
┌──────────────────────────────────────────────────────────────────┐
│                      Detection Cycle (30s polling)               │
│  main.go: runDetectionCycle()                                    │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐            │
│  │  IM     │  │  VC     │  │  Docs   │  │Calendar │  ...        │
│  │Detector │  │Detector │  │Detector │  │Detector │             │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘            │
│       │            │            │            │                   │
│       ▼            ▼            ▼            ▼                   │
│  ┌─────────────────────────────────────────────────┐            │
│  │           Emitter Layer                          │            │
│  │  IMEmitter    VCEmitter   DocsEmitter  CalEmitter│           │
│  │  (关键词+Pin)  (AI总结)    (评论审批)   (评审会)  │           │
│  └──────────────────────┬──────────────────────────┘            │
│                         │ StateChangeSignal                      │
│                         ▼                                        │
│  ┌─────────────────────────────────────────────────┐            │
│  │        WorkerPool.processJob()                   │            │
│  │  1. 匹配 Change.Type → 提取策略                  │            │
│  │  2. matchDecisionKeywords()                      │            │
│  │  3. ProcessSignalForJob() → LLM / Fallback       │            │
│  └──────────────────────┬──────────────────────────┘            │
│                         │ DecisionMutation                       │
│                         ▼                                        │
│  ┌─────────────────────────────────────────────────┐            │
│  │        PipelineEngine.ApplyMutation()            │            │
│  │  Git + Bitable + MemoryGraph                     │            │
│  └─────────────────────────────────────────────────┘            │
└──────────────────────────────────────────────────────────────────┘
```

---

## 3. 各源检测器详细设计

### 3.1 VC 检测器（视频会议/妙记）

**决策价值**：会议 AI 纪要是最高质量的决策来源，直接包含"决定事项"、"行动项"、"负责人"。

#### Detector 实现

```go
// internal/lark-adapter/lark_vc.go
type VCDetector struct {
    config *Config
    cli    *LarkCLI
}

func (d *VCDetector) Name() string { return "lark_vc" }

func (d *VCDetector) Detect(lastCheck time.Time) (*DetectResult, error) {
    var changes []Change

    // 1. 搜索已结束会议
    //    lark-cli vc +search --start-time <ts> --end-time <ts> --page-all
    meetings := d.searchMeetings(lastCheck)

    for _, meeting := range meetings {
        meetingID := meeting["meeting_id"].(string)

        // 2. 获取会议纪要产物
        //    lark-cli vc +notes --meeting-ids <meeting_id>
        notes := d.getMeetingNotes(meetingID)

        if notes != nil {
            // AI 总结 → 强信号
            if noteDocToken, ok := notes["note_doc_token"].(string); ok && noteDocToken != "" {
                changes = append(changes, Change{
                    Type:       "meeting_note",
                    EntityType: "meeting",
                    EntityID:   meetingID,
                    Summary:    fmt.Sprintf("[会议纪要] %s: AI总结已生成", meeting["topic"]),
                    Timestamp:  extractMeetingEndTime(meeting),
                })
            }

            // 待办事项 → 强信号（直接含行动项）
            if todos, ok := notes["todos"].([]any); ok && len(todos) > 0 {
                changes = append(changes, Change{
                    Type:       "meeting_todos",
                    EntityType: "meeting",
                    EntityID:   meetingID,
                    Summary:    fmt.Sprintf("[会议待办] %s: %d个行动项", meeting["topic"], len(todos)),
                    Timestamp:  extractMeetingEndTime(meeting),
                })
            }
        }
    }

    // 3. 搜索妙记
    //    lark-cli minutes +search --query "" --start-time <ts> --end-time <ts>
    minutes := d.searchMinutes(lastCheck)
    for _, minute := range minutes {
        changes = append(changes, Change{
            Type:       "minute_created",
            EntityType: "minute",
            EntityID:   minute["minute_token"].(string),
            Summary:    fmt.Sprintf("[妙记] %s", minute["title"]),
            Timestamp:  extractMinuteTime(minute),
        })
    }

    return &DetectResult{
        Source:     d.Name(),
        HasChanges: len(changes) > 0,
        DetectedAt: time.Now(),
        LastCheck:  lastCheck,
        Changes:    changes,
    }, nil
}
```

#### Emitter 信号分类

```go
func (e *VCEmitter) EmitSignal(result *DetectResult) (*StateChangeSignal, error) {
    signal := NewSignal(AdapterVC, "VC changes detected")
    strength := StrengthWeak

    for _, ch := range result.Changes {
        switch ch.Type {
        case "meeting_note":
            // AI 纪要 = 强信号：直接包含决策结论
            strength = maxStrength(strength, StrengthStrong)
            signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "ai_summary")
        case "meeting_todos":
            // 待办 = 强信号：行动项即决策执行
            strength = maxStrength(strength, StrengthStrong)
            signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "action_items")
        case "minute_created":
            // 新妙记 = 中信号：可能含决策
            strength = maxStrength(strength, StrengthMedium)
        }
    }

    signal.Strength = strength
    return signal, nil
}
```

#### Worker 处理扩展

VC 检测器产生的 Change 类型（`meeting_note`、`meeting_todos`、`minute_created`）需要在 `processJob()` 中特殊处理：

```go
// worker.go processJob() 扩展
func (wp *WorkerPool) processJob(job *DetectionJob) *DecisionResult {
    // ... 现有 IM 逻辑 ...

    // VC 会议纪要：直接读取 AI 总结内容进行提取
    if job.Change.Type == "meeting_note" || job.Change.Type == "meeting_todos" {
        content := wp.fetchMeetingNoteContent(job.Change.EntityID)
        if content != "" {
            sig := NewSignal(job.AdapterType, job.Change.Summary)
            sig.Strength = StrengthStrong
            proposer := "会议系统"
            mut, err := wp.engine.ProcessSignalForJob(sig, proposer, content)
            // ...
        }
    }

    // 妙记：读取纪要内容
    if job.Change.Type == "minute_created" {
        content := wp.fetchMinuteContent(job.Change.EntityID)
        // ...
    }
}
```

---

### 3.2 Docs 检测器（云文档）

**决策价值**：正式决策文档（PRD、技术方案）和文档评论中的审批/反对意见。

#### Detector 实现

```go
// internal/lark-adapter/lark_docs.go
type DocsDetector struct {
    config *Config
    cli    *LarkCLI
}

func (d *DocsDetector) Name() string { return "lark_docs" }

func (d *DocsDetector) Detect(lastCheck time.Time) (*DetectResult, error) {
    var changes []Change

    // 1. 搜索新创建/更新的文档
    //    lark-cli docs +search --query "<项目关键词>" --page-all
    //    注意：+search 不支持时间过滤，需对比 lastCheck
    docs := d.searchDocs(lastCheck)

    for _, doc := range docs {
        docToken := doc["doc_token"].(string)
        title := doc["title"].(string)

        // 检测标题是否含决策关键词
        if containsDecisionKeyword(title) {
            changes = append(changes, Change{
                Type:       "doc_decision",
                EntityType: "doc",
                EntityID:   docToken,
                Summary:    fmt.Sprintf("[文档] 决策相关文档: %s", title),
                Timestamp:  extractDocTime(doc),
            })
        } else {
            changes = append(changes, Change{
                Type:       "doc_updated",
                EntityType: "doc",
                EntityID:   docToken,
                Summary:    fmt.Sprintf("[文档] 文档更新: %s", title),
                Timestamp:  extractDocTime(doc),
            })
        }

        // 2. 获取未解决评论
        //    lark-cli drive file.comments list --params '{"file_token":"<token>","file_type":"docx","is_solved":false}'
        comments := d.getDocComments(docToken)
        for _, comment := range comments {
            commentText := comment["reply_list"].(map[string]any)["replies"].([]any)[0].(map[string]any)["content"].(map[string]any)["elements"].([]any)[0].(map[string]any)["text_run"].(map[string]any)["text"].(string)

            if containsApprovalKeyword(commentText) {
                changes = append(changes, Change{
                    Type:       "doc_comment_approval",
                    EntityType: "doc_comment",
                    EntityID:   comment["comment_id"].(string),
                    Summary:    fmt.Sprintf("[文档评论] %s: %s", title, truncate(commentText, 60)),
                    Timestamp:  extractCommentTime(comment),
                })
            }
        }
    }

    return &DetectResult{...}, nil
}
```

#### Emitter 信号分类

```go
func (e *DocsEmitter) EmitSignal(result *DetectResult) (*StateChangeSignal, error) {
    signal := NewSignal(AdapterDocs, "Doc changes detected")
    strength := StrengthWeak

    for _, ch := range result.Changes {
        switch ch.Type {
        case "doc_decision":
            // 标题含决策关键词 = 强信号
            strength = maxStrength(strength, StrengthStrong)
            signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "decision_doc")
        case "doc_comment_approval":
            // 评论含审批关键词 = 强信号
            strength = maxStrength(strength, StrengthStrong)
            signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "approval_comment")
        case "doc_updated":
            // 普通文档更新 = 弱信号
            strength = maxStrength(strength, StrengthWeak)
        }
    }

    signal.Strength = strength
    return signal, nil
}
```

---

### 3.3 Calendar 检测器（日程）

**决策价值**：识别决策评审会议、里程碑事件，追踪决策时间线。

#### Detector 实现

```go
// internal/lark-adapter/lark_calendar.go
type CalendarDetector struct {
    config *Config
    cli    *LarkCLI
}

func (d *CalendarDetector) Name() string { return "lark_calendar" }

func (d *CalendarDetector) Detect(lastCheck time.Time) (*DetectResult, error) {
    var changes []Change

    // 1. 搜索日程（含决策关键词）
    //    lark-cli calendar events search --params '{"query":"评审|决策|确认|review","start_time":"<ts>","end_time":"<ts>"}'
    events := d.searchEvents(lastCheck, []string{"评审", "决策", "确认", "review", "milestone", "里程碑"})

    for _, event := range events {
        eventID := event["event_id"].(string)
        title := event["summary"].(string)

        changes = append(changes, Change{
            Type:       "decision_meeting",
            EntityType: "calendar_event",
            EntityID:   eventID,
            Summary:    fmt.Sprintf("[日程] 决策相关日程: %s", title),
            Timestamp:  extractEventTime(event),
        })
    }

    // 2. 获取今日日程概览（用于上下文）
    //    lark-cli calendar +agenda --date <today>
    //    不产生 Change，但可作为上下文补充

    return &DetectResult{...}, nil
}
```

#### Emitter 信号分类

```go
func (e *CalendarEmitter) EmitSignal(result *DetectResult) (*StateChangeSignal, error) {
    signal := NewSignal(AdapterCalendar, "Calendar changes detected")
    strength := StrengthWeak

    for _, ch := range result.Changes {
        if ch.Type == "decision_meeting" {
            // 决策评审日程 = 中信号（日程本身不是决策，但指向决策场景）
            strength = maxStrength(strength, StrengthMedium)
            signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "review_meeting")
        }
    }

    signal.Strength = strength
    return signal, nil
}
```

---

### 3.4 Task 检测器（任务）

**决策价值**：追踪决策执行进度，任务状态变更反映决策推进。

#### Detector 实现

```go
// internal/lark-adapter/lark_task.go
type TaskDetector struct {
    config *Config
    cli    *LarkCLI
}

func (d *TaskDetector) Name() string { return "lark_task" }

func (d *TaskDetector) Detect(lastCheck time.Time) (*DetectResult, error) {
    var changes []Change

    // 1. 获取与我相关的任务
    //    lark-cli task +get-related-tasks --page-size 50
    tasks := d.getRelatedTasks()

    for _, task := range tasks {
        taskGUID := task["guid"].(string)
        title := task["summary"].(string)
        status := task["status"].(string)
        updatedAt := parseTaskTime(task["updated_at"].(string))

        if updatedAt.After(lastCheck) {
            changeType := "task_updated"
            if status == "done" {
                changeType = "task_completed"
            }

            changes = append(changes, Change{
                Type:       changeType,
                EntityType: "task",
                EntityID:   taskGUID,
                Summary:    fmt.Sprintf("[任务] %s (状态: %s)", title, status),
                Timestamp:  updatedAt.Unix(),
            })
        }
    }

    // 2. 搜索决策相关任务
    //    lark-cli task +search --query "<决策关键词>" --page-size 50
    decisionTasks := d.searchDecisionTasks()
    // ... 类似处理 ...

    return &DetectResult{...}, nil
}
```

#### Emitter 信号分类

```go
func (e *TaskEmitter) EmitSignal(result *DetectResult) (*StateChangeSignal, error) {
    signal := NewSignal(AdapterTask, "Task changes detected")
    strength := StrengthWeak

    for _, ch := range result.Changes {
        switch ch.Type {
        case "task_completed":
            // 任务完成 = 中信号（可能推进决策阶段）
            strength = maxStrength(strength, StrengthMedium)
            signal.Context.DecisionSignals = append(signal.Context.DecisionSignals, "task_done")
        case "task_updated":
            // 任务更新 = 弱信号
            strength = maxStrength(strength, StrengthWeak)
        }
    }

    signal.Strength = strength
    return signal, nil
}
```

---

### 3.5 Wiki 检测器（知识库）

**决策价值**：已沉淀的决策文档结构，知识节点变更反映决策归档。

#### Detector 实现

```go
// internal/lark-adapter/lark_wiki.go
type WikiDetector struct {
    config *Config
    cli    *LarkCLI
}

func (d *WikiDetector) Name() string { return "lark_wiki" }

func (d *WikiDetector) Detect(lastCheck time.Time) (*DetectResult, error) {
    var changes []Change

    // 1. 获取知识空间列表
    //    lark-cli wiki spaces list
    spaces := d.listSpaces()

    for _, space := range spaces {
        spaceID := space["space_id"].(string)

        // 2. 获取子节点列表
        //    lark-cli wiki nodes list --params '{"space_id":"<id>","page_size":50}'
        nodes := d.listNodes(spaceID)

        for _, node := range nodes {
            nodeToken := space["node_token"].(string)
            title := node["title"].(string)

            // 检测新增/更新的决策相关节点
            if containsDecisionKeyword(title) {
                changes = append(changes, Change{
                    Type:       "wiki_decision_node",
                    EntityType: "wiki_node",
                    EntityID:   nodeToken,
                    Summary:    fmt.Sprintf("[知识库] 决策节点: %s", title),
                    Timestamp:  time.Now().Unix(),
                })
            }
        }
    }

    return &DetectResult{...}, nil
}
```

---

### 3.6 Contact 检测器（通讯录）

**决策价值**：辅助角色解析，将 open_id 映射为人名/部门。不直接产生决策信号。

#### 设计说明

Contact 不作为独立检测器运行，而是作为**工具函数**被其他检测器调用：

```go
// internal/lark-adapter/lark_contact.go
type ContactResolver struct {
    cli *LarkCLI
}

// ResolveUser 将 open_id 解析为用户信息
func (r *ContactResolver) ResolveUser(openID string) (name, department string, err error) {
    // lark-cli contact +get-user --user-id <open_id>
    output, err := r.cli.RunCommand("contact", "+get-user", "--user-id", openID)
    // ...
}

// SearchUser 按姓名搜索用户
func (r *ContactResolver) SearchUser(query string) ([]UserInfo, error) {
    // lark-cli contact +search-user --query <姓名>
    // ...
}
```

Contact 在信号引擎中的角色：
- `ContactEmitter` 保持 Weak 强度，不主动触发决策提取
- 在 `ProcessSignalForJob()` 中，当 proposer/executor 为 open_id 时，调用 `ContactResolver` 解析为人名

---

### 3.7 OKR 检测器

**决策价值**：战略对齐参考，辅助判断决策影响级别。

#### 设计说明

OKR 作为**上下文补充**而非独立决策源：

```go
// internal/lark-adapter/lark_okr.go
type OKRDetector struct {
    config *Config
    cli    *LarkCLI
}

func (d *OKRDetector) Name() string { return "lark_okr" }

func (d *OKRDetector) Detect(lastCheck time.Time) (*DetectResult, error) {
    // OKR 变更频率低，仅作上下文
    // lark-cli okr +cycle-list --user-id <open_id>
    // lark-cli okr +cycle-detail --cycle-id <id>
    // 不产生强信号，仅在 ContextAssembler 中使用
    return &DetectResult{HasChanges: false}, nil
}
```

---

## 4. Worker Pool 扩展

### 4.1 当前限制

`processJob()` 仅处理 `new_text` 和 `new_post`，其他类型直接跳过：

```go
// worker.go:107-111
if job.Change.Type != "new_text" && job.Change.Type != "new_post" {
    log.Println("[Worker] Not a text message, skipping")
    return result
}
```

### 4.2 扩展方案

按适配器类型分发处理策略：

```go
func (wp *WorkerPool) processJob(job *DetectionJob) *DecisionResult {
    result := &DecisionResult{Job: job, Processed: time.Now()}

    // 按适配器类型分发
    switch job.AdapterType {
    case signal.AdapterIM:
        return wp.processIMJob(job, result)
    case signal.AdapterVC:
        return wp.processVCJob(job, result)
    case signal.AdapterDocs:
        return wp.processDocsJob(job, result)
    case signal.AdapterCalendar:
        return wp.processCalendarJob(job, result)
    case signal.AdapterTask:
        return wp.processTaskJob(job, result)
    case signal.AdapterWiki:
        return wp.processWikiJob(job, result)
    default:
        return result
    }
}
```

### 4.3 各源处理策略

| 适配器 | Change.Type | 处理方式 | 内容来源 |
|--------|------------|---------|---------|
| IM | `new_text`, `new_post` | 关键词匹配 → LLM 提取 | 消息 body |
| VC | `meeting_note` | 直接 LLM 提取（高置信度） | `lark-cli docs +fetch --doc <note_doc_token>` |
| VC | `meeting_todos` | 直接 LLM 提取 | 待办列表 JSON |
| Docs | `doc_decision` | 关键词匹配 → LLM 提取 | `lark-cli docs +fetch --doc <token>` |
| Docs | `doc_comment_approval` | 直接 LLM 提取（审批信号） | 评论内容 |
| Calendar | `decision_meeting` | 标记为决策上下文，不直接提取 | 日程标题+描述 |
| Task | `task_completed` | 更新关联决策状态 | 任务详情 |
| Wiki | `wiki_decision_node` | LLM 提取 | `lark-cli docs +fetch --doc <node_token>` |

---

## 5. 跨源关联设计

### 5.1 ActivationRouter 权重调整

当前 `router.go` 中的 `relatedPairs` 需要扩展：

```go
relatedPairs := map[[2]AdapterType]float64{
    {AdapterIM, AdapterVC}:       0.3,  // 群聊讨论 ↔ 会议决策
    {AdapterIM, AdapterDocs}:     0.2,  // 群聊讨论 ↔ 文档审批
    {AdapterIM, AdapterTask}:     0.2,  // 群聊讨论 ↔ 任务分配
    {AdapterVC, AdapterDocs}:     0.2,  // 会议纪要 ↔ 决策文档
    {AdapterVC, AdapterTask}:     0.3,  // 会议待办 ↔ 任务创建
    {AdapterDocs, AdapterWiki}:   0.2,  // 文档 ↔ 知识库归档
    {AdapterTask, AdapterOKR}:    0.2,  // 任务进度 ↔ OKR 对齐
    {AdapterCalendar, AdapterVC}: 0.3,  // 日程 ↔ 会议纪要
    {AdapterCalendar, AdapterIM}: 0.1,  // 日程 ↔ 群聊通知
}
```

### 5.2 ContextAssembler 关联提示

当多个源同时产生信号时，Assembler 生成关联提示：

```go
// 场景：IM 消息"决定用方案B" + Docs 文档"方案B技术方案" + Calendar "方案B评审会"
// → Assembler 生成 CorrelationHint:
{
    Type:        "DocDecisionLink",
    Confidence:  0.85,
    TargetSDR:   "sdr_xxx",
    Description: "群聊决策 '方案B' 与文档 '方案B技术方案' 和日程 '方案B评审会' 关联",
}
```

### 5.3 DecisionNode.FeishuLinks 填充

多源信号合并时，填充 `FeishuLinks` 的对应字段：

```go
// IM 信号 → 填充 RelatedChatIDs, RelatedMessageIDs
node.FeishuLinks.RelatedChatIDs = []string{"oc_xxx"}
node.FeishuLinks.RelatedMessageIDs = []string{"om_xxx"}

// VC 信号 → 填充 RelatedMeetingIDs, RelatedMinuteTokens
node.FeishuLinks.RelatedMeetingIDs = []string{"meeting_xxx"}
node.FeishuLinks.RelatedMinuteTokens = []string{"minute_xxx"}

// Docs 信号 → 填充 RelatedDocTokens
node.FeishuLinks.RelatedDocTokens = []string{"doxcn_xxx"}

// Calendar 信号 → 填充 RelatedEventIDs
node.FeishuLinks.RelatedEventIDs = []string{"event_xxx"}

// Task 信号 → 填充 RelatedTaskGUIDs
node.FeishuLinks.RelatedTaskGUIDs = []string{"task_xxx"}
```

---

## 6. 实现优先级

### Phase 1: 高价值源（决策直接来源）

| 优先级 | 检测器 | 理由 | 工作量 |
|--------|--------|------|--------|
| P0 | VC Detector | AI 纪要直接含决策结论，信号质量最高 | 中 |
| P0 | Docs Detector | 正式决策文档 + 评论审批 | 中 |

### Phase 2: 中价值源（决策辅助）

| 优先级 | 检测器 | 理由 | 工作量 |
|--------|--------|------|--------|
| P1 | Calendar Detector | 识别决策评审场景 | 小 |
| P1 | Task Detector | 追踪决策执行进度 | 小 |

### Phase 3: 低价值源（上下文补充）

| 优先级 | 检测器 | 理由 | 工作量 |
|--------|--------|------|--------|
| P2 | Wiki Detector | 已沉淀决策结构 | 小 |
| P2 | Contact Resolver | 角色解析工具 | 小 |
| P3 | OKR Detector | 战略对齐参考 | 极小 |

---

## 7. 实现步骤

### 7.1 新增 Detector 文件

每个检测器一个文件，遵循 `lark_im.go` 的模式：

```
internal/lark-adapter/
├── lark_im.go        ✅ 已有
├── lark_vc.go        ← 新增
├── lark_docs.go      ← 新增
├── lark_calendar.go  ← 新增
├── lark_task.go      ← 新增
├── lark_wiki.go      ← 新增
├── lark_contact.go   ← 新增（工具类）
├── lark_cli.go       ✅ 已有
├── config.go         ✅ 已有
└── types.go          ✅ 已有
```

### 7.2 注册到 main.go

```go
// cmd/mem-service/main.go
detectors := map[signal.AdapterType]larkadapter.Detector{
    signal.AdapterIM:       larkadapter.NewIMExtractor(larkCfg),
    signal.AdapterVC:       larkadapter.NewVCDetector(larkCfg),
    signal.AdapterDocs:     larkadapter.NewDocsDetector(larkCfg),
    signal.AdapterCalendar: larkadapter.NewCalendarDetector(larkCfg),
    signal.AdapterTask:     larkadapter.NewTaskDetector(larkCfg),
    signal.AdapterWiki:     larkadapter.NewWikiDetector(larkCfg),
}
```

### 7.3 扩展 WorkerPool

修改 `processJob()` 支持多适配器类型的 Change 处理。

### 7.4 扩展 Emitter

将 Stub Emitter 替换为有实际逻辑的实现。

---

## 8. 检测器轮询间隔策略

不同源的变更频率不同，应使用不同的轮询间隔：

| 源 | 建议间隔 | 理由 |
|----|---------|------|
| IM | 30s | 消息高频，需要实时性 |
| VC | 5min | 会议结束后才产生纪要，低频 |
| Docs | 2min | 文档更新频率中等 |
| Calendar | 10min | 日程变更低频 |
| Task | 2min | 任务状态变更中频 |
| Wiki | 10min | 知识库变更低频 |

实现方式：为每个 Detector 增加 `PollInterval()` 方法，`runDetectionCycle()` 根据间隔决定是否跳过。

---

## 9. 测试验证方案

### 9.1 单元测试

每个 Detector 的 `Detect()` 方法需要 mock `LarkCLI` 输出进行测试。

### 9.2 集成测试

1. 启动 mem-service，注册所有检测器
2. 在飞书中制造决策场景：
   - 群聊发送含决策关键词的消息
   - 创建会议并生成 AI 纪要
   - 创建决策文档并添加评论
   - 创建任务并更新状态
3. 检查 `outputs/detect_state.json` 确认各源都被检测
4. 检查 `data/decisions/` 确认决策被正确提取和存储

### 9.3 端到端验证

```bash
# 1. 重启 mem-service
docker exec openclaw-zh bash -c 'cd /root/openclaw-workspace/feishu-agent-mem && \
  pkill -f mem-service 2>/dev/null; sleep 1; \
  go build -o bin/mem-service ./cmd/mem-service/main.go && \
  ./bin/mem-service &'

# 2. 查看日志确认所有检测器启动
tail -f logs/app.log | grep -E "\[Detector\]|\[Worker\]|\[ResultProcessor\]"

# 3. 检查各源检测结果
cat outputs/detect_state.json | jq 'keys'
# 期望输出: ["lark_calendar", "lark_docs", "lark_im", "lark_task", "lark_vc", "lark_wiki"]
```
