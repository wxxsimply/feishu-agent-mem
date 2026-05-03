# Git 运作设计：OpenClaw 记忆系统中的权威存储

> Git 作为决策记录的唯一真实来源（Single Source of Truth），承载不可变历史、版本控制、全文检索三大核心职责。本文档详细规划 Git 在 OpenClaw 记忆系统中的完整运作方式，与 decision-tree.md、openclaw-architecture.md、signal-activation-engine.md 严格对齐。

---

## 一、Git 在系统中的角色定位

### 1.1 四组件架构中的 Git

```
┌─────────────────────────────────────────────────────────┐
│                    OpenClaw Core                         │
│  PipelineEngine → MemoryGraph → ConflictResolver        │
└───────┬──────────────────────────────────────┬──────────┘
        │                                      │
   ┌────▼────┐                            ┌────▼────┐
   │   Git   │  ←→  双向同步（hash 锚定） ←→ │ Bitable │
   │ 权威存储 │                              │ 查询接口 │
   └────┬────┘                              └─────────┘
        │
        ▼
  decisions/{project}/{topic}/{sdr_id}.md
```

### 1.2 Git vs Bitable 的职责边界

| 维度 | Git（权威存储） | Bitable（查询接口） |
|------|----------------|-------------------|
| **写入时机** | 决策产生/修改时**首先**写入 | Git commit 后**同步**写入 |
| **数据权威性** | 唯一真实来源 | 镜像（可从 Git 重建） |
| **提供的能力** | 不可变历史、版本回溯、blame、全文搜索 | WHERE 过滤、CONTAINS 匹配、分组排序、视图 |
| **删除后果** | 丢失不可恢复的历史 | 丧失结构化查询能力 |
| **一致性锚点** | commit hash 是写入时生成 | `git_commit_hash` 字段记录对应 commit |

### 1.3 Git 不做的事

- **不做实时查询**：`WHERE topic = "认证模块" AND status = "active"` 由 Bitable 执行
- **不做跨议题关联**：`cross_topic_refs CONTAINS "用户服务"` 由 Bitable 执行
- **不做视图分组**：按 Topic/Phase 分组查看由 Bitable 视图负责
- **不做并发冲突仲裁**：语义冲突由 OpenClaw Core 的 ConflictResolver 处理

---

## 二、仓库结构设计

### 2.1 目录即树

Git 目录结构直接映射 decision-tree.md 中的 Topic → Decision 两层树：

```
openclaw-data/                          # Git 仓库根目录
│
├── decisions/                          # 决策文件根目录
│   ├── {project}/                      # 项目维度（如 "Q2-core-platform"）
│   │   ├── {topic}/                    # 议题维度（如 "认证模块"）
│   │   │   ├── {sdr_id}.md            # 单个决策文件
│   │   │   ├── {sdr_id}.md            # ...
│   │   │   └── ...
│   │   ├── {topic}/                    # 另一个议题
│   │   │   └── ...
│   │   └── topic-index.md             # 本项目各议题的决策摘要索引
│   └── {project}/                      # 另一个项目
│       └── ...
│
├── conflicts/                          # 未解决的冲突记录
│   ├── {conflict_id}.md
│   └── ...
│
├── archive/                            # 已归档的历史项目
│   └── {project}/
│       └── ...
│
├── L0_RULES.md                         # 核心规则（不可变，L0 层）
│
└── .git/                               # Git 元数据
    ├── hooks/                          # 自定义 hooks
    │   ├── pre-commit                  # YAML 验证
    │   └── post-commit                 # 触发 Bitable 同步
    └── ...
```

### 2.2 目录与决策树的映射关系

| 决策树概念 | Git 路径 | 说明 |
|-----------|---------|------|
| Project | `decisions/{project}/` | 一级目录 |
| Topic | `decisions/{project}/{topic}/` | 二级目录 |
| Decision | `decisions/{project}/{topic}/{sdr_id}.md` | 文件 |
| L0 核心规则 | `L0_RULES.md` | 仓库根目录，不可变 |
| 冲突记录 | `conflicts/{conflict_id}.md` | 独立目录 |
| 议题摘要 | `decisions/{project}/topic-index.md` | LLM 定期生成 |

### 2.3 目录创建规则

```
规则 1: 首次写入某 project 的决策时，自动创建 decisions/{project}/ 目录
规则 2: 首次写入某 topic 的决策时，自动创建 decisions/{project}/{topic}/ 目录
规则 3: 目录名与 decision-tree.md 中的 topic 字段完全一致
规则 4: 目录名使用 UTF-8 编码，支持中文（如 "认证模块"）
规则 5: archive/ 目录下的项目通过 git mv 从 decisions/ 移入，保留完整历史
```

### 2.4 topic-index.md 格式

每个项目的 topic-index.md 由 LLM 定期生成，汇总该议题下所有 active 决策的摘要：

```markdown
# {project} 决策索引

> 自动生成于 2026-04-29，由 OpenClaw Core 定期刷新

## 认证模块

| SDR ID | 标题 | 影响级别 | 状态 | 阶段 | 最后更新 |
|--------|------|---------|------|------|---------|
| dec_20260428_001 | OAuth2 认证方案 | major | active | dev | 2026-04-28 |
| dec_20260415_002 | Token 格式升级 | minor | active | dev | 2026-04-15 |

## 数据库架构

| SDR ID | 标题 | 影响级别 | 状态 | 阶段 | 最后更新 |
|--------|------|---------|------|------|---------|
| dec_20260420_003 | 主键类型变更 | major | active | dev,test | 2026-04-20 |
```

---

## 三、决策文件格式

### 3.1 文件结构：YAML Frontmatter + Markdown 正文

与 openclaw-architecture.md 2.1.3 节 DecisionNode 结构严格对齐：

```markdown
---
# === 标识 ===
sdr_id: dec_20260428_001
git_commit_hash: ""                    # 首次写入时由 Git 填充

# === 内容 ===
title: "数据库主键从 INT 改为 BIGINT"
decision: |
  将 user_sessions 表的 session_token 字段从 VARCHAR(256) 改为 VARCHAR(512)，
  以支持新的 JWT 格式。
rationale: |
  新的 JWT 格式需要更长的 token 字段。VARCHAR(256) 不足以容纳
  新格式的 session token。

# === 树位置 ===
project: "Q2-core-platform"
topic: "数据库架构"                     # 唯一位置锚点（decision-tree.md §1.2）

# === 时态标签 ===
phase: "dev"                           # 决策产生时的阶段
phase_scope: "Span"                    # Point | Span | Retroactive
version_range:
  from: "v1.0.0"
  to: null                             # null 表示当前生效

# === 影响级别 ===
impact_level: "major"                  # advisory | minor | major | critical
                                       # 取代 tree_level（decision-tree.md §3.1）

# === 跨议题影响 ===
cross_topic_refs:                      # 非空即表示跨议题（decision-tree.md §4.2）
  - "认证模块"
  - "用户服务"

# === 树内父子关系 ===
parent_decision: null                  # 父决策 ID（REFINES 关系）
children_count: 0                      # 直接子决策数

# === 人员 ===
proposer: "张三"
executor: "李四"
stakeholders:
  - "王五"

# === 关系图谱 ===
relations:
  - type: "DEPENDS_ON"
    target_sdr_id: "dec_20260415_002"
  - type: "SUPERSEDES"
    target_sdr_id: "dec_20260401_005"

# === 飞书关联 ===
feishu_links:
  related_chat_ids: ["oc_xxx"]
  related_message_ids: ["om_111", "om_222"]
  related_doc_tokens: ["doxcnxxx"]
  related_event_ids: ["event_xxx"]
  related_meeting_ids: ["meeting_xxx"]
  related_task_guids: ["task_xxx"]
  related_minute_tokens: []

# === 状态 ===
status: "active"                       # active | superseded | deprecated
created_at: "2026-04-28T10:00:00+08:00"
decided_at: "2026-04-28T14:30:00+08:00"
---

## 决策

将 user_sessions 表的 session_token 字段从 VARCHAR(256) 改为 VARCHAR(512)，
以支持新的 JWT 格式。

## 依据

新的 JWT 格式需要更长的 token 字段。当前 VARCHAR(256) 限制了新 JWT 格式的采用。
团队评估后认为 VARCHAR(512) 可覆盖未来 2 年的 token 长度增长。

## 影响范围

- 认证模块：JWT 生成逻辑需适配新字段长度
- 用户服务：登录流程依赖 session_token
- 管理后台：会话信息展示模块需调整

## 备选方案

1. **VARCHAR(1024)**：过度预留，浪费存储
2. **TEXT 类型**：无法建立有效索引
3. **VARCHAR(512)**：平衡存储与扩展性 ← 选定

## 实施计划

- [ ] 修改数据库 Schema（DDL 脚本）
- [ ] 更新 ORM 映射
- [ ] 编写数据迁移脚本
- [ ] 更新 API 文档
```

### 3.2 Frontmatter 字段对照表

与 decision-tree.md §2.1 DecisionNode 一一对应：

| YAML 字段 | 类型 | 必填 | 默认值 | 说明 |
|-----------|------|------|--------|------|
| `sdr_id` | string | 是 | — | 唯一标识，格式 `dec_{date}_{seq}` |
| `git_commit_hash` | string | 否 | `""` | 首次 commit 后由 post-commit hook 回填 |
| `title` | string | 是 | — | 决策标题（一句话） |
| `decision` | string | 是 | — | 决策结论 |
| `rationale` | string | 是 | — | 决策依据 |
| `project` | string | 是 | — | 所属项目 |
| `topic` | string | 是 | — | 所属议题（树中唯一位置） |
| `phase` | string | 是 | — | 决策产生时的阶段 |
| `phase_scope` | enum | 是 | — | `Point` / `Span` / `Retroactive` |
| `version_range.from` | string | 是 | — | 决策生效的起始版本 |
| `version_range.to` | string \| null | 否 | `null` | 决策失效的版本（null = 当前生效） |
| `impact_level` | enum | 是 | — | `advisory` / `minor` / `major` / `critical` |
| `cross_topic_refs` | string[] | 否 | `[]` | 受影响的其他议题 |
| `parent_decision` | string \| null | 否 | `null` | 父决策 ID |
| `children_count` | int | 否 | `0` | 直接子决策数 |
| `proposer` | string | 是 | — | 决策提出人 |
| `executor` | string | 否 | — | 决策执行人 |
| `stakeholders` | string[] | 否 | `[]` | 利益相关方 |
| `relations` | Relation[] | 否 | `[]` | 关系图谱 |
| `feishu_links` | FeishuLinks | 否 | `{}` | 飞书实体关联 |
| `status` | enum | 是 | `active` | `active` / `superseded` / `deprecated` |
| `created_at` | datetime | 是 | — | 决策创建时间 |
| `decided_at` | datetime \| null | 否 | `null` | 决策确认时间 |

### 3.3 L0_RULES.md 格式

核心规则文件，位于仓库根目录，**不可被常规流程修改**：

```markdown
# L0 核心规则

> 本文件中的规则不可被决策覆盖。违反 L0 规则的决策将被阻断。

## 规则列表

### L0-001: 数据安全
- 不得在决策文件中存储密码、密钥、Token 等敏感信息
- 数据库连接字符串使用环境变量引用

### L0-002: 向后兼容
- 涉及 API 变更的决策必须包含迁移方案
- 废弃 API 需保留至少一个版本周期

### L0-003: 审批流程
- impact_level 为 critical 的决策必须经过两人以上审批
- 涉及生产环境的决策必须包含回滚方案
```

---

## 四、Commit 策略

### 4.1 提交消息格式

与 openclaw-architecture.md §2.3 `write_decision` 方法中的格式对齐：

```
<type>(<topic>): <sdr_id> — <title>

<body>
```

| type | 说明 | 示例 |
|------|------|------|
| `decision` | 新增决策 | `decision(认证模块): dec_20260428_001 — OAuth2 认证方案` |
| `update` | 修改决策字段 | `update(数据库架构): dec_20260420_003 — 状态从 active 改为 superseded` |
| `resolve` | 解决冲突 | `resolve(用户服务): dec_20260428_005 冲突已解决 — 采用新方案` |
| `archive` | 归档项目 | `archive(Q1-mvp): 项目 Q1-mvp 已归档，保留 15 条决策` |
| `index` | 刷新索引 | `index(Q2-core-platform): 刷新 topic-index.md` |
| `rule` | 修改 L0 规则 | `rule: 新增 L0-004 审计日志规则（需双人审批）` |

### 4.2 Commit Body 规范

```markdown
decision(数据库架构): dec_20260428_001 — 数据库主键从 INT 改为 BIGINT

决策: 将 user_sessions 表的 session_token 字段从 VARCHAR(256) 改为 VARCHAR(512)
依据: 新 JWT 格式需要更长的 token 字段
影响: 认证模块, 用户服务
提出人: 张三
执行人: 李四
```

Body 结构化地包含决策的核心信息，使得 `git log` 不需要读取文件就能获取决策摘要。

### 4.3 原子提交规则

```
规则 1: 每次决策变更产生一个原子 commit
        └── 一次 commit 只修改一个 {sdr_id}.md 文件
        └── 例外: topic-index.md 可与决策文件同 commit（如 index type）

规则 2: 写入顺序 = Git 先，Bitable 后
        └── Git commit 成功 → 拿到 commit hash → 写入 Bitable 的 git_commit_hash 字段
        └── Git commit 失败 → 不写 Bitable，回滚 MemoryGraph

规则 3: 不使用 --amend
        └── 每次变更创建新 commit，保留完整历史
        └── 纠错通过新 commit（如 update type）而非修改历史

规则 4: 冲突文件独立提交
        └── conflicts/{conflict_id}.md 单独 commit
        └── 不与决策文件混在同一 commit
```

### 4.4 并发写入处理

OpenClaw Core 可能同时处理多个飞书事件，产生并发写入：

```
并发场景:
  Event A (IM 消息) → Pipeline → 写入 dec_20260428_001.md
  Event B (Task 完成) → Pipeline → 更新 dec_20260420_003.md

处理策略:
  ┌─────────────────────────────────────────────────┐
  │ 1. Git 仓库级别: 使用文件锁 (flock)              │
  │    ├── 写入前获取排他锁                           │
  │    ├── git add + commit 在锁内完成                │
  │    └── 锁释放后下一个写入者进入                    │
  │                                                   │
  │ 2. 应用级别: 写入队列                             │
  │    ├── PipelineEngine 产出的变更进入写入队列        │
  │    ├── 队列消费器串行执行 git add + commit         │
  │    └── 不同文件的变更可并行（不同 Topic 目录）      │
  │                                                   │
  │ 3. 冲突处理: git merge --no-commit               │
  │    ├── 若两个事件修改同一文件（极端情况）           │
  │    ├── 先到者正常 commit                          │
  │    └── 后到者 rebase onto 后的 commit              │
  └─────────────────────────────────────────────────┘
```

```go
// 写入队列实现
type CommitQueue struct {
    queue    chan CommitRequest
    lock     sync.Mutex
    gitLock  *flock.Flock  // 文件锁
}

type CommitRequest struct {
    FilePath string
    Content  []byte
    Message  string
    ResultCh chan CommitResult
}

func (cq *CommitQueue) Submit(req CommitRequest) CommitResult {
    cq.queue <- req
    result := <-req.ResultCh
    return result
}

func (cq *CommitQueue) Process() {
    for req := range cq.queue {
        cq.lock.Lock()
        // 获取 Git 仓库文件锁
        cq.gitLock.Lock()
        defer cq.gitLock.Unlock()

        // 写入文件
        os.WriteFile(req.FilePath, req.Content, 0644)
        // git add + commit
        hash := cq.gitCommit(req.FilePath, req.Message)
        req.ResultCh <- CommitResult{Hash: hash, Err: nil}

        cq.lock.Unlock()
    }
}
```

---

## 五、分支模型

### 5.1 分支结构

```
main                                    ← 永久分支，决策的权威记录
  │
  ├── discussion/{sdr_id}/{timestamp}   ← 讨论分支（重大决策的审议）
  │     └── 讨论结束后合并回 main
  │
  └── hotfix/{sdr_id}                   ← 紧急修复分支（错误决策的快速纠正）
        └── 修复后合并回 main
```

### 5.2 main 分支

- **唯一权威分支**：所有已确认的决策都在 main 上
- **不允许 force push**：历史不可变
- **自动推送**：commit 后自动 push 到远程（`auto_push: true`）
- **保护规则**：通过 GitLab/GitHub branch protection 实现

### 5.3 discussion 分支

用于重大决策（`impact_level: major` 或 `critical`）的审议流程：

```
使用场景:
  - 新决策 D_new 与现有决策 D_old 语义矛盾 > 0.6 且权重相当
  - impact_level 为 critical 的决策需要双人审批
  - 用户主动要求创建讨论分支

流程:
  1. OpenClaw Core 创建分支: discussion/dec_20260428_005/20260429
  2. 在分支上写入 D_new 文件（status: pending_review）
  3. 在飞书群聊中通知相关人员
  4. 讨论期间的修改在分支上进行
  5. 审批通过 → merge into main，status 改为 active
  6. 审批不通过 → 删除分支，D_new 标记为 rejected
```

### 5.4 hotfix 分支

用于紧急纠正错误决策：

```
使用场景:
  - 已确认的决策包含事实错误
  - 决策的 impact_level 需要紧急提升
  - 决策被误标为 active 但实际应为 deprecated

流程:
  1. 从 main 创建 hotfix 分支
  2. 修改决策文件（status 或内容修正）
  3. 直接 merge 回 main（跳过 discussion 流程）
  4. Bitable 同步更新
```

---

## 六、合并策略

### 6.1 合并场景

| 场景 | 触发条件 | 合并方式 | 冲突处理 |
|------|---------|---------|---------|
| discussion 分支合并 | 审批通过 | `git merge --no-ff` | 手动解决 |
| hotfix 分支合并 | 紧急修复 | `git merge --no-ff` | 手动解决 |
| 远程 pull | 定时同步 | `git pull --rebase` | LLM 辅助 |
| Bitable 反向同步 | 用户在 Bitable 修改 | 生成新 commit | 字段级合并 |

### 6.2 LLM 辅助合并

当 Git 合并产生冲突时，使用 LLM 进行语义级合并：

```go
// LLM 合并器
type LLMMerger struct {
    llmClient LLMClient
}

func (m *LLMMerger) MergeConflict(conflict *GitConflict) (*MergedFile, error) {
    // 1. 解析冲突区域
    ours := conflict.OursSections()
    theirs := conflict.TheirsSections()

    // 2. 构建 LLM prompt
    prompt := fmt.Sprintf(`
以下决策文件存在合并冲突。请根据语义选择正确的版本或合并两者。

文件路径: %s

我们的版本（main 分支）:
%s

他们的版本（讨论分支）:
%s

请输出合并后的完整 YAML frontmatter 和 Markdown 正文。
保留双方的有效信息，以 main 分支为权威基准。
`, conflict.FilePath, ours, theirs)

    // 3. 调用 LLM
    merged, err := m.llmClient.Complete(prompt)
    if err != nil {
        return nil, err
    }

    // 4. 验证合并结果（YAML 合法性 + 必填字段）
    if err := validateYAML(merged); err != nil {
        return nil, fmt.Errorf("LLM 合并结果 YAML 无效: %w", err)
    }

    return &MergedFile{Content: merged}, nil
}
```

### 6.3 字段级合并规则

当同一决策文件的不同字段被修改时，按字段粒度合并：

```
合并优先级（高→低）:
  1. sdr_id, project, topic          ← 永远以 main 为准
  2. status                          ← 取最新状态（时间戳更大者）
  3. decision, rationale             ← LLM 语义合并
  4. impact_level                    ← 取更高者（安全优先）
  5. cross_topic_refs                ← 取并集
  6. relations                       ← 取并集
  7. feishu_links                    ← 取并集
  8. proposer, executor, stakeholders ← 取最新
  9. phase, phase_scope              ← 取最新
  10. version_range                  ← 取最新
```

---

## 七、Git ↔ Bitable 双向同步协议

### 7.1 同步锚点：git_commit_hash

`git_commit_hash` 是 Git 与 Bitable 之间**唯一的同步锚点**（openclaw-architecture.md §3.3）。

```
Git commit hash 的生命周期:

  1. 决策文件首次写入 → Git commit → 获得 commit_hash_1
  2. Bitable 写入记录 → 同时写入 git_commit_hash = commit_hash_1
  3. 决策文件修改     → Git commit → 获得 commit_hash_2
  4. Bitable 更新记录 → 更新 git_commit_hash = commit_hash_2

一致性判断:
  Bitable.git_commit_hash == Git.HEAD_hash_of_file
  → 一致
  Bitable.git_commit_hash ≠ Git.HEAD_hash_of_file
  → 存在未同步变更，触发修复流程
```

### 7.2 正向同步：Git → Bitable

```
触发时机:
  - Git commit 完成后（post-commit hook）
  - 定时轮询（每 30 分钟）

流程:
  ┌─────────────────────────────────────────────┐
  │ 1. Git post-commit hook 触发                │
  │    └── 传递: commit_hash, file_path         │
  │                                              │
  │ 2. 解析决策文件的 YAML frontmatter           │
  │    └── 得到完整的 DecisionNode               │
  │                                              │
  │ 3. Bitable.upsert_decision(node)            │
  │    ├── sdr_id → 作为记录主键                 │
  │    ├── 所有 YAML 字段 → 映射为 Bitable 字段  │
  │    ├── git_commit_hash → 写入当前 commit     │
  │    └── cross_topic_refs → 逗号分隔字符串     │
  │                                              │
  │ 4. 更新 MemoryGraph                          │
  │    └── 加载新决策到内存索引                   │
  └─────────────────────────────────────────────┘
```

### 7.3 反向同步：Bitable → Git

当用户在 Bitable Web UI 手动修改决策记录时：

```
触发时机:
  - Bitable Webhook（record.updated 事件）
  - 定时一致性校验发现 hash 不匹配

流程:
  ┌──────────────────────────────────────────────────┐
  │ 1. 检测到 Bitable 记录变更                        │
  │    └── record_id + changed_fields                │
  │                                                    │
  │ 2. 读取 Git 中对应决策文件的当前内容               │
  │    └── git show HEAD:{path}/{sdr_id}.md          │
  │                                                    │
  │ 3. 读取 Bitable 当前记录值                         │
  │    └── base +record-get --record-id {id}         │
  │                                                    │
  │ 4. 字段级对比                                     │
  │    ├── Bitable 修改的字段 → 以 Bitable 值为准     │
  │    ├── Bitable 未修改的字段 → 以 Git 值为准       │
  │    └── 冲突字段 → LLM 语义判断                    │
  │                                                    │
  │ 5. 生成合并后的 YAML frontmatter                   │
  │    └── 写入 Git 文件                               │
  │                                                    │
  │ 6. Git commit                                     │
  │    ├── message: "update({topic}): {sdr_id} —      │
  │    │            Bitable 手动修改: {changed_fields}"│
  │    └── 获得 new_commit_hash                       │
  │                                                    │
  │ 7. 回写 Bitable 的 git_commit_hash                │
  │    └── 更新为 new_commit_hash                     │
  └──────────────────────────────────────────────────┘
```

### 7.4 一致性校验

```
触发: 每 30 分钟（consistency_check.interval_minutes: 30）

流程:
  1. 遍历 Bitable 中所有决策记录
  2. 对每条记录:
     a. 读取 Bitable.git_commit_hash
     b. 计算 Git 中对应文件的 HEAD commit hash
        cmd: git log -1 --format=%H -- {path}/{sdr_id}.md
     c. 比较两者
  3. 不一致的记录 → 分类:
     ├── Git 有新 commit, Bitable 旧 → 正向同步
     ├── Bitable 有修改, Git 旧 → 反向同步
     └── 两者都有修改 → 冲突，通知用户
  4. 批量修复（auto_fix: false 时仅报告，不自动修复）
```

### 7.5 字段映射表

YAML frontmatter 字段与 Bitable 字段的对应关系：

| YAML 字段 | Bitable 字段 | 类型 | 备注 |
|-----------|-------------|------|------|
| `sdr_id` | `sdr_id` | 文本 | 记录主键 |
| `title` | `title` | 文本 | |
| `decision` | `decision` | 多行文本 | |
| `rationale` | `rationale` | 多行文本 | |
| `project` | `project` | 文本 | |
| `topic` | `topic` | 单选 | 用于 WHERE 过滤 |
| `phase` | `phase` | 多选 | 用于分组视图 |
| `phase_scope` | `phase_scope` | 单选 | |
| `impact_level` | `impact_level` | 单选 | |
| `cross_topic_refs` | `cross_topic_refs` | 文本 | 逗号分隔，用于 CONTAINS |
| `parent_decision` | `parent_decision` | 文本（关联） | 关联到决策表自身 |
| `children_count` | `children_count` | 数字 | 预计算字段 |
| `proposer` | `proposer` | 文本（人员） | |
| `executor` | `executor` | 文本（人员） | |
| `stakeholders` | `stakeholders` | 文本 | 逗号分隔 |
| `relations` | `relations` | 文本 | JSON 字符串 |
| `status` | `status` | 单选 | |
| `created_at` | `created_at` | 日期 | |
| `decided_at` | `decided_at` | 日期 | |
| `git_commit_hash` | `git_commit_hash` | 文本 | 同步锚点 |

---

## 八、Git Hooks

### 8.1 Pre-commit Hook

在 commit 前验证决策文件的合法性：

```bash
#!/bin/bash
# .git/hooks/pre-commit

# 获取本次 commit 涉及的 .md 文件
CHANGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.md$')

for file in $CHANGED_FILES; do
    # 跳过非决策文件
    if [[ ! "$file" =~ ^decisions/ ]]; then
        continue
    fi

    # 1. YAML frontmatter 存在性检查
    if ! head -1 "$file" | grep -q '^---$'; then
        echo "❌ $file: 缺少 YAML frontmatter"
        exit 1
    fi

    # 2. 必填字段检查
    for field in sdr_id title decision rationale project topic phase phase_scope impact_level status created_at; do
        if ! grep -q "^${field}:" "$file"; then
            echo "❌ $file: 缺少必填字段 $field"
            exit 1
        fi
    done

    # 3. YAML 语法验证
    if command -v yq &> /dev/null; then
        # 提取 frontmatter（两个 --- 之间的内容）
        sed -n '/^---$/,/^---$/p' "$file" | sed '1d;$d' | yq eval '.' - > /dev/null 2>&1
        if [ $? -ne 0 ]; then
            echo "❌ $file: YAML frontmatter 语法错误"
            exit 1
        fi
    fi

    # 4. impact_level 枚举值检查
    LEVEL=$(grep '^impact_level:' "$file" | awk '{print $2}' | tr -d '"')
    if [[ ! "$LEVEL" =~ ^(advisory|minor|major|critical)$ ]]; then
        echo "❌ $file: impact_level 值无效: $LEVEL"
        exit 1
    fi

    # 5. status 枚举值检查
    STATUS=$(grep '^status:' "$file" | awk '{print $2}' | tr -d '"')
    if [[ ! "$STATUS" =~ ^(active|superseded|deprecated|pending_review|rejected)$ ]]; then
        echo "❌ $file: status 值无效: $STATUS"
        exit 1
    fi

    # 6. L0 规则检查（critical 决策必须有审批信息）
    if [ "$LEVEL" == "critical" ]; then
        if ! grep -q 'approver:' "$file"; then
            echo "⚠️  $file: critical 级别决策建议包含 approver 字段"
            # 不阻断，仅警告
        fi
    fi

    # 7. sdr_id 格式检查
    SDR_ID=$(grep '^sdr_id:' "$file" | awk '{print $2}' | tr -d '"')
    if [[ ! "$SDR_ID" =~ ^dec_[0-9]{8}_[0-9]{3}$ ]]; then
        echo "❌ $file: sdr_id 格式应为 dec_YYYYMMDD_NNN，实际: $SDR_ID"
        exit 1
    fi

    echo "✅ $file: 验证通过"
done

exit 0
```

### 8.2 Post-commit Hook

commit 成功后触发 Bitable 同步：

```bash
#!/bin/bash
# .git/hooks/post-commit

# 获取最新 commit 的信息
COMMIT_HASH=$(git log -1 --format=%H)
CHANGED_FILES=$(git diff-tree --no-commit-id --name-only -r HEAD | grep '\.md$')

for file in $CHANGED_FILES; do
    if [[ "$file" =~ ^decisions/.*\.md$ ]]; then
        # 通知 OpenClaw Core 进行 Bitable 同步
        # 通过 Unix socket 或 HTTP 回调
        curl -s -X POST http://localhost:8421/hooks/post-commit \
            -H "Content-Type: application/json" \
            -d "{\"commit_hash\": \"$COMMIT_HASH\", \"file_path\": \"$file\"}" \
            > /dev/null 2>&1 &
    fi
done

# 更新 topic-index.md（如果决策文件有变更）
if echo "$CHANGED_FILES" | grep -q '^decisions/'; then
    # 异步触发索引刷新（不阻塞 commit）
    curl -s -X POST http://localhost:8421/hooks/refresh-index \
        > /dev/null 2>&1 &
fi

exit 0
```

### 8.3 Commit-msg Hook

验证 commit message 格式：

```bash
#!/bin/bash
# .git/hooks/commit-msg

MSG=$(cat "$1")

# 格式: <type>(<topic>): <sdr_id> — <title>
if ! echo "$MSG" | head -1 | grep -qE '^(decision|update|resolve|archive|index|rule)\(.+\): .+ — .+'; then
    echo "❌ Commit message 格式错误"
    echo "   期望: <type>(<topic>): <sdr_id> — <title>"
    echo "   示例: decision(认证模块): dec_20260428_001 — OAuth2 认证方案"
    exit 1
fi

exit 0
```

---

## 九、历史操作

### 9.1 版本追溯

```bash
# 查看某决策的完整修改历史
git log --follow --format="%H %ai %s" -- decisions/{project}/{topic}/{sdr_id}.md

# 示例输出:
# a1b2c3d 2026-04-28 14:30:00 +0800 decision(数据库架构): dec_20260428_001 — 主键变更
# e4f5g6h 2026-04-29 10:15:00 +0800 update(数据库架构): dec_20260428_001 — 状态改为 superseded
```

### 9.2 Blame（逐行追溯）

```bash
# 查看某决策文件每一行的最后修改者
git blame decisions/{project}/{topic}/{sdr_id}.md

# 在 OpenClaw Core 中的封装
func (gs *GitStorage) BlameDecision(project, topic, sdr_id string) []BlameEntry {
    path := fmt.Sprintf("decisions/%s/%s/%s.md", project, topic, sdr_id)
    blame, _ := gs.repo.BlameFile(path, git2.NewBlameOptions())
    entries := []BlameEntry{}
    for i := 0; i < blame.HunksCount(); i++ {
        hunk := blame.HunkByIndex(i)
        entries = append(entries, BlameEntry{
            Line:      hunk.FinalStartLine(),
            Commit:    hunk.FinalCommitId().String(),
            Author:    hunk.FinalSignature().Name(),
            Date:      hunk.FinalSignature().When(),
        })
    }
    return entries
}
```

### 9.3 Diff（版本对比）

```bash
# 对比某决策的两个版本
git diff {commit_1}..{commit_2} -- decisions/{project}/{topic}/{sdr_id}.md

# 对比某决策在两个时间点的状态
git diff @{2026-04-28}..@{2026-04-29} -- decisions/{project}/{topic}/{sdr_id}.md
```

### 9.4 决策回滚

```bash
# 场景：决策 dec_20260428_001 的最新修改有误，回滚到上一个版本
git log --oneline -- decisions/Q2-core-platform/数据库架构/dec_20260428_001.md
# 输出:
# e4f5g6h update(数据库架构): dec_20260428_001 — 错误的修改
# a1b2c3d decision(数据库架构): dec_20260428_001 — 原始正确的决策

# 方式 1: revert（推荐，保留历史）
git revert e4f5g6h

# 方式 2: checkout 恢复文件 + 新 commit
git checkout a1b2c3d -- decisions/Q2-core-platform/数据库架构/dec_20260428_001.md
git commit -m "update(数据库架构): dec_20260428_001 — 回滚到 a1b2c3d 版本"
```

---

## 十、全文搜索

### 10.1 Git Grep

```bash
# 搜索决策内容中的关键词
git grep -l "PostgreSQL" -- decisions/

# 搜索特定议题下的关键词
git grep -l "缓存" -- decisions/Q2-core-platform/认证模块/

# 搜索所有 active 状态的决策中提到某个人
git grep "executor: 张三" -- decisions/

# 搜索 YAML frontmatter 中特定字段值
git grep "impact_level:.*critical" -- decisions/
```

### 10.2 OpenClaw Core 中的搜索封装

```go
// 全文搜索
func (gs *GitStorage) SearchContent(project, query string) []SearchHit {
    args := []string{"grep", "-l", "--regexp", query}
    if project != "" {
        args = append(args, "--", fmt.Sprintf("decisions/%s/", project))
    } else {
        args = append(args, "--", "decisions/")
    }

    output, _ := gs.runGit(args...)
    hits := []SearchHit{}
    for _, line := range strings.Split(output, "\n") {
        if line == "" {
            continue
        }
        hits = append(hits, SearchHit{
            FilePath: line,
            // 进一步读取文件，定位匹配行
        })
    }
    return hits
}

// 结构化搜索（组合 git grep + YAML 解析）
func (gs *GitStorage) SearchDecisions(opts SearchOpts) []DecisionNode {
    // 1. 先用 git grep 缩小范围
    hits := gs.SearchContent(opts.Project, opts.Keywords)

    // 2. 解析每个命中的文件
    results := []DecisionNode{}
    for _, hit := range hits {
        node, err := gs.ReadDecision(hit.FilePath)
        if err != nil {
            continue
        }
        // 3. 应用过滤条件
        if opts.Topic != "" && node.Topic != opts.Topic {
            continue
        }
        if opts.Status != "" && node.Status != opts.Status {
            continue
        }
        if opts.ImpactLevel != "" && node.ImpactLevel != opts.ImpactLevel {
            continue
        }
        results = append(results, node)
    }
    return results
}
```

---

## 十一、归档策略

### 11.1 归档触发条件

```
项目满足以下所有条件时触发归档:
  1. 项目中所有决策的 status 均为 superseded 或 deprecated
  2. 项目最后一条决策的 decided_at 距今超过 90 天
  3. 用户确认（或 cron 自动执行时，需 auto_archive: true）
```

### 11.2 归档流程

```
Step 1: 移动目录
  git mv decisions/{project} archive/{project}
  ├── 保留完整 Git 历史（git mv 不破坏 blame/log）
  └── archive/ 目录下的决策不再参与日常检索

Step 2: 更新 topic-index.md
  在 decisions/{project}/topic-index.md 中标注:
  > ⚠️ 本项目已于 2026-08-01 归档，请查阅 archive/{project}/

Step 3: Git commit
  commit message: archive({project}): 项目 {project} 已归档，保留 {N} 条决策

Step 4: Bitable 同步
  将 Bitable 中该项目的所有决策记录的 status 更新为 archived
  创建一个 "已归档" 视图，移出默认视图

Step 5: MemoryGraph 清理
  从内存索引中移除归档项目的决策
  保留归档路径映射: project → archive/{project}
```

### 11.3 归档后检索

```
日常检索: 只搜索 decisions/ 目录
归档检索: 显式指定搜索 archive/ 目录
  git grep "PostgreSQL" -- archive/
  或在 Bitable 中切换到 "已归档" 视图
```

---

## 十二、仓库初始化与维护

### 12.1 初始化流程

```go
// 与 openclaw-architecture.md §5.3 启动流程对齐
func (gs *GitStorage) Initialize(config Config) error {
    // 1. 检查仓库是否存在
    if _, err := git2.OpenRepository(config.WorkDir); err != nil {
        // 2a. 仓库不存在 → 初始化
        repo, err := git2.InitRepository(config.WorkDir, false) // false = 非 bare
        if err != nil {
            return err
        }

        // 2b. 创建基础目录结构
        os.MkdirAll(filepath.Join(config.WorkDir, "decisions"), 0755)
        os.MkdirAll(filepath.Join(config.WorkDir, "conflicts"), 0755)
        os.MkdirAll(filepath.Join(config.WorkDir, "archive"), 0755)

        // 2c. 写入 L0_RULES.md
        os.WriteFile(filepath.Join(config.WorkDir, "L0_RULES.md"),
            []byte(DEFAULT_L0_RULES), 0644)

        // 2d. 写入 .gitignore
        os.WriteFile(filepath.Join(config.WorkDir, ".gitignore"),
            []byte(".DS_Store\n__pycache__/\n*.pyc\n"), 0644)

        // 2e. 初始 commit
        gs.commitAll("init: 初始化 OpenClaw 决策仓库")

        // 2f. 设置远程仓库
        if config.Remote != "" {
            repo.Remotes.Create("origin", config.Remote)
        }

        // 2g. 安装 Git hooks
        gs.installHooks()

    } else {
        // 3. 仓库已存在 → 打开并拉取最新
        repo, _ := git2.OpenRepository(config.WorkDir)
        gs.repo = repo

        // 拉取远程最新
        gs.pull()
    }

    // 4. 从 Git 加载全量决策到 MemoryGraph
    gs.MemoryGraph.LoadFromGit(gs)

    return nil
}
```

### 12.2 Hook 安装

```go
func (gs *GitStorage) InstallHooks() {
    hooksDir := filepath.Join(gs.workDir, ".git", "hooks")

    // pre-commit: YAML 验证
    os.WriteFile(filepath.Join(hooksDir, "pre-commit"),
        []byte(PRE_COMMIT_HOOK_SCRIPT), 0755)

    // post-commit: Bitable 同步触发
    os.WriteFile(filepath.Join(hooksDir, "post-commit"),
        []byte(POST_COMMIT_HOOK_SCRIPT), 0755)

    // commit-msg: 格式验证
    os.WriteFile(filepath.Join(hooksDir, "commit-msg"),
        []byte(COMMIT_MSG_HOOK_SCRIPT), 0755)
}
```

### 12.3 定期维护任务

| 任务 | 频率 | 命令 | 说明 |
|------|------|------|------|
| 远程同步 | 每次 commit 后 | `git push origin main` | auto_push 配置 |
| 远程拉取 | 每 5 分钟 | `git pull --rebase origin main` | 获取其他节点的变更 |
| 索引刷新 | 每日 09:00 | 重新生成 topic-index.md | 保持索引最新 |
| 一致性校验 | 每 30 分钟 | 对比 git_commit_hash | 检测 Git ↔ Bitable 漂移 |
| 仓库 GC | 每月 1 日 | `git gc --aggressive` | 压缩历史，优化存储 |
| 归档检查 | 每周一 10:00 | 检查归档条件 | 自动或提示归档 |
| 备份 | 每日 02:00 | `git bundle create backup.bundle --all` | 完整备份 |

---

## 十三、性能考量

### 13.1 操作成本估算

| 操作 | Git 命令 | 预估耗时 | Token 预算影响 |
|------|---------|---------|---------------|
| 读取单个决策 | `cat file` | < 5ms | ~500 tokens |
| 按议题列出决策 | `ls + cat` | < 50ms | ~2K tokens |
| 全文搜索 | `git grep` | < 100ms | 按结果计 |
| Blame | `git blame` | < 200ms | ~1K tokens |
| Diff 两个版本 | `git diff` | < 100ms | ~1K tokens |
| Commit | `git add + commit` | < 100ms | 0（不消耗上下文） |
| Pull/Push | `git pull/push` | 100ms-5s | 0（后台操作） |

### 13.2 大规模仓库优化

```
当决策文件数量超过 10,000 时:

1. 浅克隆（Shallow Clone）
   git clone --depth 1 <remote>
   └── 只拉最新版本，不拉完整历史
   └── 适用场景: 新节点初始化
   └── 代价: 无法执行完整 blame/log

2. 稀疏检出（Sparse Checkout）
   git sparse-checkout init --cone
   git sparse-checkout set decisions/Q2-core-platform
   └── 只检出当前项目的决策
   └── 适用场景: 多项目仓库中只关注一个项目

3. Git LFS（大文件存储）
   └── 不适用于 .md 文件（通常 < 10KB）
   └── 如有附件（截图、PDF），使用 LFS 存储

4. 分库策略（极端情况）
   └── 按 project 拆分为独立 Git 仓库
   └── 每个仓库 < 5,000 文件，保证性能
```

### 13.3 MemoryGraph 预加载优化

```go
// 启动时从 Git 全量加载到内存（openclaw-architecture.md §2.1.2）
func (mg *MemoryGraph) LoadFromGit(gs *GitStorage) error {
    start := time.Now()

    // 并行加载各 project 的决策
    projects := gs.ListProjects()
    var wg sync.WaitGroup
    for _, project := range projects {
        wg.Add(1)
        go func(p string) {
            defer wg.Done()
            topics := gs.ListTopics(p)
            for _, topic := range topics {
                decisions := gs.ListDecisions(p, topic)
                for _, d := range decisions {
                    mg.AddDecision(d)
                }
            }
        }(project)
    }
    wg.Wait()

    log.Printf("MemoryGraph 加载完成: %d 条决策, 耗时 %v",
        mg.Count(), time.Since(start))
    return nil
}

// 增量更新: 只加载新 commit 后的变更
func (mg *MemoryGraph) IncrementalUpdate(gs *GitStorage, since string) error {
    changes, _ := gs.ChangesSince(since)
    for _, change := range changes {
        switch change.Type {
        case "added", "modified":
            node, _ := gs.ReadDecision(change.FilePath)
            mg.UpsertDecision(node)
        case "deleted":
            mg.RemoveDecision(change.SDRID)
        }
    }
    return nil
}
```

---

## 十四、与 SignalActivationEngine 的集成

### 14.1 信号 → 决策变更 → Git Commit 的完整链路

与 signal-activation-engine.md §6 的编排器对齐：

```
飞书事件
    │
    ▼
SignalActivationEngine.on_event(event)
    │
    ├── Step 1: 适配器识别 → 发射信号
    │   └── StateChangeSignal { adapter, strength, context }
    │
    ├── Step 2: 激活路由 → 上下文装配
    │   └── 查权重矩阵 → 从 MemoryGraph + 各适配器拉取上下文
    │
    ├── Step 3: 状态机评估 → 决策变更操作
    │   └── DecisionMutation {
    │         type: "create" | "update" | "status_change" | "conflict",
    │         sdr_id: string,
    │         fields: map[string]interface{},
    │         commit_message: string
    │       }
    │
    └── Step 4: 通过 GitStorage 执行变更
        │
        ▼
GitStorage.ApplyMutation(mutation)
    │
    ├── type == "create":
    │   ├── 生成新 sdr_id (dec_{date}_{seq})
    │   ├── 渲染 YAML frontmatter + Markdown
    │   ├── 写入 decisions/{project}/{topic}/{sdr_id}.md
    │   └── git commit: "decision({topic}): {sdr_id} — {title}"
    │
    ├── type == "update":
    │   ├── 读取现有文件
    │   ├── 合并变更字段
    │   ├── 写回文件
    │   └── git commit: "update({topic}): {sdr_id} — {变更摘要}"
    │
    ├── type == "status_change":
    │   ├── 修改 status 字段
    │   ├── 如有 decided_at 变更，同步更新
    │   └── git commit: "update({topic}): {sdr_id} — 状态从 {old} 改为 {new}"
    │
    └── type == "conflict":
        ├── 写入 conflicts/{conflict_id}.md
        ├── 在两个冲突决策间建立 CONFLICTS_WITH 关系
        └── git commit: "resolve({topic}): 发现冲突 {conflict_id}"
```

### 14.2 状态机转换的 Git 操作映射

与 signal-activation-engine.md §4.2 转换规则对齐：

| 状态转换 | 触发信号 | Git Commit Type | Commit Message 示例 |
|---------|---------|----------------|-------------------|
| 待决策 → 决策中 | IM 讨论/Calendar 评审会 | `update` | `update(认证模块): dec_xxx — 进入决策讨论阶段` |
| 决策中 → 已决策 | Docs 审批/VC 确认/IM LGTM | `update` | `update(认证模块): dec_xxx — 决策已确认` |
| 已决策 → 执行中 | Task 创建关联任务 | `update` | `update(认证模块): dec_xxx — 进入执行阶段` |
| 执行中 → 已完成 | Task 全部完成/里程碑到达 | `update` | `update(认证模块): dec_xxx — 执行完成` |
| 任意 → 搁置 | 用户/LLM 判定 | `update` | `update(认证模块): dec_xxx — 决策搁置` |
| 任意 → 否决 | 用户/LLM 判定 | `update` | `update(认证模块): dec_xxx — 决策否决` |
| 任意 → superseded | 新决策 SUPERSEDES | `update` | `update(认证模块): dec_xxx — 被 dec_yyy 替代` |

### 14.3 决策生命周期的完整 Git 历史示例

```
commit 1: decision(数据库架构): dec_20260428_001 — 主键类型变更
  └── status: active, phase: dev, impact_level: major

commit 2: update(数据库架构): dec_20260428_001 — 进入执行阶段
  └── status: executing（触发: Task 创建了迁移脚本任务）

commit 3: update(数据库架构): dec_20260428_001 — 状态从 executing 改为 completed
  └── status: completed（触发: 所有关联任务完成）

commit 4: update(数据库架构): dec_20260428_001 — 被 dec_20260515_002 替代
  └── status: superseded（触发: 新决策 SUPERSEDES 关系）
```

---

## 十五、故障恢复

### 15.1 场景分类

| 故障场景 | 影响 | 恢复方式 |
|---------|------|---------|
| Git commit 失败 | 决策未持久化 | 重试 + 回滚 MemoryGraph |
| Bitable 同步失败 | 查询接口数据过期 | 定时一致性校验修复 |
| Git push 失败 | 远程仓库未更新 | 重试 + 手动 push |
| 文件损坏 | 决策文件不可读 | Git reflog 恢复 |
| 仓库损坏 | Git 元数据失效 | 从远程重新 clone |
| MemoryGraph 不一致 | 内存数据过期 | 从 Git 重新加载 |

### 15.2 恢复流程

```go
// Git commit 失败恢复
func (gs *GitStorage) SafeWrite(node *DecisionNode) error {
    // 1. 写入临时文件
    tmpPath := gs.TempPath(node)
    os.WriteFile(tmpPath, gs.RenderFile(node), 0644)

    // 2. Git commit
    hash, err := gs.Commit(tmpPath, gs.CommitMessage(node))
    if err != nil {
        // 3. 失败 → 清理临时文件，回滚 MemoryGraph
        os.Remove(tmpPath)
        gs.MemoryGraph.Rollback(node.SDRID)
        return fmt.Errorf("commit 失败: %w", err)
    }

    // 4. 成功 → 移动到正式路径（如果需要）+ 更新 hash
    node.GitCommitHash = hash
    gs.MemoryGraph.Confirm(node)
    return nil
}

// 仓库损坏恢复
func (gs *GitStorage) Recover() error {
    // 1. 备份当前仓库
    backupPath := gs.workDir + ".backup." + time.Now().Format("20060102")
    os.Rename(gs.workDir, backupPath)

    // 2. 从远程重新 clone
    _, err := git2.Clone(gs.Remote, gs.workDir, &git2.CloneOptions{})
    if err != nil {
        // 3. clone 失败 → 从备份恢复
        os.Rename(backupPath, gs.workDir)
        return fmt.Errorf("恢复失败: %w", err)
    }

    // 4. 重新加载 MemoryGraph
    gs.MemoryGraph.LoadFromGit(gs)
    return nil
}

// reflog 恢复（找回丢失的 commit）
func (gs *GitStorage) RecoverFromReflog() error {
    reflog, _ := gs.repo.Reflog("HEAD")
    for i := 0; i < reflog.EntryCount(); i++ {
        entry := reflog.EntryByIndex(i)
        log.Printf("reflog[%d]: %s — %s", i,
            entry.IdOld(), entry.Message())
    }
    // 用户选择要恢复的 commit → cherry-pick 或 reset
    return nil
}
```

---

## 十六、配置项速查

与 openclaw-architecture.md §5.1 配置对齐：

```yaml
# openclaw.yaml 中 Git 相关配置
git:
  work_dir: "./openclaw-data"           # Git 仓库工作目录
  remote: "git@gitlab.com:team/openclaw-decisions.git"  # 远程仓库
  auto_push: true                       # commit 后自动 push
  branch: "main"                        # 默认分支

  # 一致性校验
  consistency_check:
    interval_minutes: 30                # 校验频率
    auto_fix: false                     # 是否自动修复漂移

  # 归档策略
  archive:
    auto_archive: false                 # 是否自动归档
    inactive_days: 90                   # 项目不活跃天数阈值

  # 写入队列
  commit_queue:
    buffer_size: 100                    # 队列容量
    flush_interval_ms: 1000             # 刷入间隔

  # 维护任务
  maintenance:
    gc_schedule: "0 2 1 * *"            # 每月 1 日 02:00 执行 git gc
    index_refresh_schedule: "0 9 * * *" # 每日 09:00 刷新索引
    backup_schedule: "0 2 * * *"        # 每日 02:00 备份
```

---

## 十七、与其他设计文档的对齐索引

| 本文档章节 | 对齐的文档和章节 | 对齐点 |
|-----------|----------------|-------|
| §2 仓库结构 | decision-tree.md §6.3 | `decisions/{project}/{topic}/{sdr_id}.md` |
| §3 文件格式 | openclaw-architecture.md §2.1.3 | DecisionNode 字段一一对应 |
| §3.3 L0 规则 | decision-tree.md §3.1 | L0 核心规则不可被决策覆盖 |
| §4 Commit 策略 | openclaw-architecture.md §2.3 | commit message 格式 |
| §7 同步协议 | openclaw-architecture.md §3.3 | git_commit_hash 锚点 + 一致性校验 |
| §8 Hooks | openclaw-architecture.md §5.3 | 启动流程中的 hook 安装 |
| §13 性能 | decision-tree.md §3.2 | Token 预算与 impact_level 的关系 |
| §14 集成 | signal-activation-engine.md §6 | 信号 → 变更 → Git 的完整链路 |
| §14.2 状态机 | signal-activation-engine.md §4.2 | 状态转换规则的 Git 操作映射 |
| §15 故障恢复 | openclaw-architecture.md §3.3 | 一致性校验 + 修复流程 |

---

## 十八、总结

Git 在 OpenClaw 记忆系统中的核心价值：

1. **不可变历史**：每个决策的每一次变更都有完整的 commit 历史，支持 blame、diff、回滚
2. **原子性保证**：每个 commit 是原子操作，不产生中间状态
3. **同步锚点**：`git_commit_hash` 是 Git ↔ Bitable 唯一的一致性判据
4. **全文检索**：`git grep` 提供决策内容的全文搜索能力（Bitable 做不到）
5. **版本控制**：分支模型支持决策的讨论、审议、紧急修复
6. **可审计性**：谁在什么时候改了什么——`git blame` + `git log` 完整回答

Git 不做的事：

- 不做实时查询（Bitable 负责）
- 不做跨议题关联（Bitable 的 CONTAINS 负责）
- 不做并发冲突仲裁（OpenClaw Core 的 ConflictResolver 负责）
- 不做语义级合并（LLM 负责）

**一句话：Git 记录历史，Bitable 回答问题，OpenClaw Core 做判断。**
