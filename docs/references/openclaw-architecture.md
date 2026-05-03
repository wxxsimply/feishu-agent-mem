# OpenClaw: 企业级飞书项目长程协作记忆系统

## 系统定位

OpenClaw 是一个基于飞书生态的企业级项目记忆系统，协调四个核心组件形成完整的数据闭环：

```
lark-cli（传感器 + 执行器）
   ↕ 事件驱动 + 主动轮询
OpenClaw Core（记忆中枢）
   ↕ 双向同步
┌─────────────────────┐
│  Git（权威存储）      │ ←→ Bitable（查询接口）
│  不可变历史 + 版本控制 │    结构化查询 + 实时视图
└─────────────────────┘
```

| 组件 | 角色 | 类比 |
|------|------|------|
| **OpenClaw Core** | 记忆中枢 — 编排提取、推理、同步、冲突解决 | 大脑皮层 |
| **lark-cli** | 传感器 + 执行器 — 从飞书提取信息、执行写操作 | 感官 + 手 |
| **Git** | 权威存储 — 决策文件的不可变记录和版本历史 | 长期记忆（海马体） |
| **Bitable** | 查询接口 — 结构化过滤、跨议题关联、视图分组 | 工作记忆（前额叶） |

---

## 一、系统架构

### 1.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                        OpenClaw System                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │                     OpenClaw Core                          │   │
│  │                                                           │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │   │
│  │  │ Pipeline  │  │  Memory  │  │ Conflict │  │  Config  │  │   │
│  │  │  Engine   │  │   Graph  │  │ Resolver │  │ Manager  │  │   │
│  │  └─────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  │   │
│  │        │              │             │             │         │   │
│  │  ┌─────┴──────────────┴─────────────┴─────────────┴─────┐   │   │
│  │  │                  Bus / Event Channel                   │   │   │
│  │  └──────────────────────────┬───────────────────────────┘   │   │
│  └─────────────────────────────┼─────────────────────────────┘   │
│                                │                                  │
│  ┌─────────────┐  ┌────────────┴────────────┐  ┌─────────────┐  │
│  │ lark-cli    │  │  Git Storage            │  │  Bitable     │  │
│  │ Adapter     │  │  Adapter                │  │  Adapter     │  │
│  │             │  │                         │  │              │  │
│  │ • IM        │  │ • Decision File R/W     │  │ • Record RW  │  │
│  │ • VC        │  │ • Commit Manager        │  │ • Schema     │  │
│  │ • Calendar  │  │ • Diff/Blame            │  │ • Query      │  │
│  │ • Docs      │  │ • Branch Merge          │  │ • View Sync  │  │
│  │ • Task/OKR  │  │                         │  │              │  │
│  │ • Contact   │  │                         │  │              │  │
│  │ • Event Sub │  │                         │  │              │  │
│  └──────┬──────┘  └────────────┬────────────┘  └──────┬───────┘  │
│         │                      │                       │          │
└─────────┼──────────────────────┼───────────────────────┼──────────┘
          │                      │                       │
          ▼                      ▼                       ▼
   ┌──────────┐          ┌──────────────┐        ┌──────────────┐
   │ 飞书服务   │          │  Git 远端     │        │  飞书 Base   │
   │ IM/VC/... │          │  (GitLab)    │        │  (bitable)   │
   └──────────┘          └──────────────┘        └──────────────┘
```

### 1.2 分层职责

```
Layer 0: 飞书基础设施         IM / VC / Calendar / Docs / Task / OKR / Contact
Layer 1: 感知与执行层        lark-cli（标准化命令封装）
Layer 2: 记忆中枢           OpenClaw Core（编排 + 推理 + 状态机）
Layer 3: 持久化层           Git + Bitable（权威存储 + 查询接口）
```

---

## 二、核心模块设计

### 2.1 OpenClaw Core

#### 2.1.1 Pipeline Engine（流程引擎）

```rust
// 定义提取 → 处理 → 同步的三阶段流水线
pub struct PipelineEngine {
    extractors: Vec<Box<dyn Extractor>>,    // lark-cli 提取器
    processors: Vec<Box<dyn Processor>>,    // 决策推理器
    syncers: Vec<Box<dyn Syncer>>,          // Git + Bitable 同步器
}

impl PipelineEngine {
    // 事件驱动入口：收到飞书事件后触发增量流水线
    pub async fn on_event(&self, event: LarkEvent) -> Result<PipelineReport> {
        // Stage 1: 提取原始数据
        let raw_data = self.run_extractors(&event).await?;

        // Stage 2: 处理为决策记录
        let decision = self.run_processors(raw_data).await?;

        // Stage 3: 同步到 Git + Bitable
        self.run_syncers(decision).await?;

        Ok(PipelineReport { /* ... */ })
    }

    // 主动轮询入口：定时任务批量提取
    pub async fn poll(&self, scope: PollScope) -> Result<Vec<PipelineReport>> {
        // 按时间范围/来源批量提取 → 批量处理 → 批量同步
    }
}
```

#### 2.1.2 Memory Graph（记忆图谱）

```rust
/// 内存中的决策图 — 运行时加速，启动时从 Git 重建
pub struct MemoryGraph {
    // 决策索引: sdr_id → DecisionNode
    decisions: HashMap<String, Arc<DecisionNode>>,

    // 议题索引: topic_name → Vec<sdr_id>
    topics: HashMap<String, Vec<String>>,

    // 关系索引: sdr_id → Vec<(RelationType, target_sdr_id)>
    relations: HashMap<String, Vec<(RelationType, String)>>,

    // 议题间的跨引用索引
    cross_topic_refs: HashMap<String, Vec<String>>, // topic → [related_topic]

    // 脏标记
    dirty_decisions: HashSet<String>,
}

impl MemoryGraph {
    /// 启动时从 Git 加载全量决策
    pub fn load_from_git(repo: &GitRepo) -> Result<Self>;

    /// 按议题检索 active 决策
    pub fn query_by_topic(&self, topic: &str) -> Vec<Arc<DecisionNode>>;

    /// 跨议题检索（含 cross_topic_refs）
    pub fn query_cross_topic(&self, topic: &str) -> Vec<Arc<DecisionNode>>;

    /// 检测新决策与现有决策的冲突
    pub fn detect_conflicts(&self, new: &DecisionNode) -> Vec<Conflict>;

    /// 标记脏记录，触发后续同步
    pub fn mark_dirty(&mut self, sdr_id: &str);
}
```

#### 2.1.3 DecisionNode（决策节点 — 与 decision-tree.md 一致）

```rust
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DecisionNode {
    // 标识
    pub sdr_id: String,              // e.g. "dec_20260428_001"
    pub git_commit_hash: String,
    pub title: String,
    pub decision: String,            // 决策结论
    pub rationale: String,           // 决策依据

    // 树位置
    pub project: String,
    pub topic: String,               // 所属议题（树中唯一位置锚点）

    // 时态标签
    pub phase: String,               // 决策产生时的阶段
    pub phase_scope: PhaseScope,     // Point | Span | Retroactive
    pub version_range: VersionRange,

    // 影响级别
    pub impact_level: ImpactLevel,   // advisory | minor | major | critical

    // 跨议题
    pub cross_topic_refs: Vec<String>,

    // 树内父子
    pub parent_decision: Option<String>,
    pub children_count: u32,

    // 人员
    pub proposer: String,
    pub executor: String,
    pub stakeholders: Vec<String>,

    // 关系图谱
    pub relations: Vec<Relation>,

    // 飞书关联（extraction.md 中的关联模型）
    pub feishu_links: FeishuLinks,

    // 状态
    pub status: DecisionStatus,
    pub created_at: DateTime<Utc>,
    pub decided_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeishuLinks {
    pub related_chat_ids: Vec<String>,
    pub related_message_ids: Vec<String>,
    pub related_doc_tokens: Vec<String>,
    pub related_event_ids: Vec<String>,
    pub related_meeting_ids: Vec<String>,
    pub related_task_guids: Vec<String>,
    pub related_minute_tokens: Vec<String>,
}

pub enum RelationType { DependsOn, Supersedes, Refines, ConflictsWith }
pub enum PhaseScope { Point, Span, Retroactive }
pub enum ImpactLevel { Advisory, Minor, Major, Critical }
pub enum DecisionStatus { Pending, InDiscussion, Decided, Executing, Completed, Shelved, Rejected }
```

### 2.2 lark-cli Adapter（感知与执行层）

```rust
/// lark-cli 命令封装层 — 所有飞书操作通过此层转发
pub struct LarkCli {
    bin_path: PathBuf,              // lark-cli 二进制路径
    identity: Identity,             // --as user | --as bot
    event_subscribers: Vec<EventSubscription>,
}

pub enum Identity { User, Bot }

impl LarkCli {
    // === 工厂方法 ===
    pub fn new(bin_path: &str) -> Self;

    /// 设置身份
    pub fn as(mut self, identity: Identity) -> Self;

    // === 群聊提取 ===
    pub async fn search_chat(&self, query: &str) -> Result<Vec<Chat>>;
    pub async fn list_messages(&self, chat_id: &str, start: i64, end: i64) -> Result<Vec<Message>>;
    pub async fn search_messages(&self, query: &str, opts: SearchOpts) -> Result<Vec<Message>>;
    pub async fn get_pin_messages(&self, chat_id: &str) -> Result<Vec<Message>>;

    // === 会议提取 ===
    pub async fn search_meetings(&self, opts: MeetingSearchOpts) -> Result<Vec<Meeting>>;
    pub async fn get_meeting_notes(&self, meeting_id: &str) -> Result<MeetingNotes>;

    // === 文档提取 ===
    pub async fn search_docs(&self, query: &str) -> Result<Vec<DocRef>>;
    pub async fn fetch_doc(&self, doc_token: &str) -> Result<String>;
    pub async fn get_doc_comments(&self, doc_token: &str) -> Result<Vec<Comment>>;

    // === 日程提取 ===
    pub async fn get_agenda(&self, date: NaiveDate) -> Result<Vec<Event>>;
    pub async fn search_events(&self, query: &str, start: i64, end: i64) -> Result<Vec<Event>>;

    // === 任务提取 ===
    pub async fn get_my_tasks(&self) -> Result<Vec<Task>>;
    pub async fn search_tasks(&self, query: &str) -> Result<Vec<Task>>;

    // === OKR 提取 ===
    pub async fn get_okr_cycle(&self, user_id: &str) -> Result<OkrCycle>;
    pub async fn get_okr_detail(&self, cycle_id: &str) -> Result<OkrDetail>;

    // === 通讯录 ===
    pub async fn search_user(&self, query: &str) -> Result<User>;
    pub async fn get_user(&self, user_id: &str) -> Result<User>;

    // === 事件监听 ===
    pub async fn subscribe_events(&self, config: EventConfig) -> Result<EventStream>;
}
```

#### 2.2.1 命令执行器（实际调用 lark-cli）

```rust
/// 实际的 CLI 命令执行
struct CliExecutor {
    bin: PathBuf,
    identity: Identity,
}

impl CliExecutor {
    /// 通用命令执行
    async fn run(&self, args: &[&str]) -> Result<CommandOutput> {
        let mut cmd = tokio::process::Command::new(&self.bin);
        cmd.args(args);
        if matches!(self.identity, Identity::Bot) {
            cmd.arg("--as").arg("bot");
        } else {
            cmd.arg("--as").arg("user");
        }
        let output = cmd.output().await?;
        // 解析 stdout 为 JSON 或 text
        Ok(CommandOutput::from(output))
    }

    /// 带 schema 检查的调用
    async fn call_with_schema(&self, resource: &str, method: &str) -> Result<serde_json::Value> {
        // 1. 先 schema 查看参数结构
        self.run(&["schema", &format!("{}.{}", resource, method)]).await?;
        // 2. 执行命令
        let out = self.run(&[resource, method, "--format", "json"]).await?;
        Ok(serde_json::from_str(&out.text)?)
    }

    /// 解析返回 JSON 中的 open_id → 联系人姓名
    async fn resolve_user(&self, open_id: &str) -> Result<ContactInfo> {
        let raw = self.run(&["contact", "+get-user", "--user-id", open_id]).await?;
        Ok(serde_json::from_str(&raw.text)?)
    }
}
```

### 2.3 Git Storage Adapter（权威存储层）

```rust
/// Git 存储封装 — 决策文件的读写、版本控制
pub struct GitStorage {
    repo: git2::Repository,
    work_dir: PathBuf,
}

impl GitStorage {
    // === 初始化 ===
    pub fn open(path: &Path) -> Result<Self>;
    pub fn init(path: &Path, remote: &str) -> Result<Self>;

    // === 目录结构 ===
    // decisions/{project}/{topic}/{sdr_id}.md

    /// 写决策文件（原子写入 + git commit）
    pub async fn write_decision(&self, node: &DecisionNode) -> Result<String> {
        let path = self.decision_path(&node.project, &node.topic, &node.sdr_id);
        let content = self.render_decision_file(node);
        fs::write(&path, content).await?;

        let commit_hash = self.git_commit(
            &format!("decision({}): {} — {}\n\n{}",
                node.topic, node.sdr_id, node.title, node.decision),
            &[path.as_path()]
        ).await?;

        Ok(commit_hash)
    }

    /// 读决策文件
    pub async fn read_decision(&self, project: &str, topic: &str, sdr_id: &str)
        -> Result<DecisionNode>
    {
        let path = self.decision_path(project, topic, sdr_id);
        let content = fs::read_to_string(&path).await?;
        DecisionNode::from_markdown(&content)
    }

    /// 按议题列出所有 active 决策
    pub fn list_decisions(&self, project: &str, topic: &str) -> Result<Vec<DecisionNode>> {
        let dir = self.work_dir.join("decisions").join(project).join(topic);
        let mut decisions = vec![];
        for entry in fs::read_dir(dir)? {
            let entry = entry?;
            if entry.path().extension().map_or(false, |e| e == "md") {
                let content = fs::read_to_string(entry.path())?;
                decisions.push(DecisionNode::from_markdown(&content)?);
            }
        }
        Ok(decisions)
    }

    /// 生成冲突文件
    pub fn write_conflict(&self, conflict: &Conflict) -> Result<String> {
        // decisions/conflicts/{conflict_id}.md
    }

    // === Git 操作 ===
    async fn git_commit(&self, message: &str, paths: &[&Path]) -> Result<String> {
        let mut index = self.repo.index()?;
        for p in paths {
            index.add_path(p.strip_prefix(&self.work_dir).unwrap_or(p))?;
        }
        index.write()?;
        let tree_id = index.write_tree()?;
        let tree = self.repo.find_tree(tree_id)?;
        let author = self.repo.signature()?;
        let parent = self.repo.head()?.peel_to_commit()?;
        let commit = self.repo.commit(
            Some("HEAD"), &author, &author,
            message, &tree, &[&parent]
        )?;
        Ok(commit.to_string())
    }

    /// 获取决策文件的版本历史
    pub fn blame_decision(&self, project: &str, topic: &str, sdr_id: &str)
        -> Result<Vec<BlameEntry>>
    {
        let path = self.decision_path(project, topic, sdr_id);
        let blame = self.repo.blame_file(path, git2::BlameOptions::new())?;
        // 解析 blame 为 { line, commit, author, date }
    }

    /// Git grep 全文搜索
    pub fn search_content(&self, project: &str, query: &str) -> Result<Vec<SearchHit>>;
}

// 决策文件格式（YAML frontmatter + Markdown 正文）
// ---
// sdr_id: dec_20260428_001
// topic: 数据库架构
// impact_level: major
// ...
// ---
// ## 决策
// 将 user_sessions 表的 session_token 从 VARCHAR(256) 改为 VARCHAR(512)
//
// ## 依据
// 新的 JWT 格式需要更长的 token 字段
```

### 2.4 Bitable Adapter（查询接口层）

```rust
/// Bitable 封装 — 结构化查询接口
pub struct BitableStore {
    cli: LarkCli,
    config: BitableConfig,
}

pub struct BitableConfig {
    pub base_token: String,         // 多维表格 token
    pub project: String,
    // 表名约定:
    //   "决策表" — DecisionNode 记录
    //   "议题表" — Topic 定义
    //   "阶段表" — Phase 定义
    //   "关系表" — Relation 边
}

impl BitableStore {
    // === 决策表 CRUD ===
    pub async fn upsert_decision(&self, node: &DecisionNode) -> Result<()> {
        // lark-cli base +record-upsert \
        //   --base-token <token> \
        //   --table-id <decision_table> \
        //   --field-values 'json...'
        let field_values = serde_json::json!({
            "sdr_id": node.sdr_id,
            "title": node.title,
            "topic": node.topic,
            "status": node.status.as_str(),
            "impact_level": node.impact_level.as_str(),
            "phase": node.phase,
            "proposer": node.proposer,
            "executor": node.executor,
            "cross_topic_refs": node.cross_topic_refs.join(", "),
            "git_commit_hash": node.git_commit_hash,
            "created_at": node.created_at.to_rfc3339(),
            "related_chats": node.feishu_links.related_chat_ids.join(", "),
            "related_docs": node.feishu_links.related_doc_tokens.join(", "),
            // ...
        });
        self.cli.run_base_cmd(&[
            "base", "+record-upsert",
            "--base-token", &self.config.base_token,
            "--table-id", DECISION_TABLE_ID,
            "--field-values", &field_values.to_string(),
        ]).await
    }

    // === 查询接口 ===
    pub async fn query_by_topic(&self, topic: &str, status: &str)
        -> Result<Vec<BitableRecord>>
    {
        // 使用视图筛选 + record-list 的组合
        // 1. +view-set-filter 设置筛选条件
        // 2. +record-list 读取
        self.cli.run_base_cmd(&[
            "base", "+record-list",
            "--base-token", &self.config.base_token,
            "--table-id", DECISION_TABLE_ID,
            "--filter", &format!(r#"{{"conjunction":"and","conditions":[{{"field_name":"topic","operator":"is","value":"{}"}},{{"field_name":"status","operator":"is","value":"{}"}}]}}"#, topic, status)
        ]).await
    }

    /// 跨议题查询（含 cross_topic_refs）
    pub async fn query_cross_topic(&self, topic: &str)
        -> Result<Vec<BitableRecord>>
    {
        // Step 1: 同议题查询
        let same_topic = self.query_by_topic(topic, "active").await?;

        // Step 2: cross_topic_refs CONTAINS
        let cross = self.cli.run_base_cmd(&[
            "base", "+record-search",
            "--base-token", &self.config.base_token,
            "--table-id", DECISION_TABLE_ID,
            "--field-name", "cross_topic_refs",
            "--contains", topic,
        ]).await?;

        Ok(same_topic.into_iter().chain(cross).collect())
    }

    /// 按阶段过滤
    pub async fn query_by_phase(&self, phase: &str) -> Result<Vec<BitableRecord>>;

    /// 获取所有议题列表
    pub async fn list_topics(&self) -> Result<Vec<TopicDef>> {
        // +record-list --table-id TOPIC_TABLE_ID
    }

    /// 同步 git_commit_hash 校验
    pub async fn verify_consistency(&self) -> Result<Vec<SyncDrift>>;
}

// Bitable 表结构定义
pub const DECISION_TABLE_ID: &str = "tbl_decision";    // 决策表
pub const TOPIC_TABLE_ID: &str = "tbl_topic";          // 议题定义表
pub const PHASE_TABLE_ID: &str = "tbl_phase";          // 阶段定义表
pub const RELATION_TABLE_ID: &str = "tbl_relation";    // 关系边表
```

---

## 三、数据流与协作协议

### 3.1 完整数据闭环

```
事件/定时触发
    │
    ▼
┌──────────────────────────────────────────────────────────┐
│                 1. 感知（lark-cli）                        │
│                                                          │
│  IM +messages-search    群聊消息中的决策关键词             │
│  VC +search             已结束会议 + AI 纪要              │
│  Minutes +search        妙记内容                          │
│  Docs +fetch            文档内容                          │
│  Calendar +agenda       日程中的里程碑                     │
│  Task +get-my-tasks     任务状态变更                      │
│  OKR +cycle-detail      OKR 进度                         │
└──────────────────────┬───────────────────────────────────┘
                       │ 原始数据 RawEvent
                       ▼
┌──────────────────────────────────────────────────────────┐
│                 2. 推理（OpenClaw Core）                   │
│                                                          │
│  2a. 决策识别 → 是否包含决策信号？                         │
│      - 关键词匹配（"决定了"/"确认"/"approve"）              │
│      - Pin 消息自动标记                                    │
│      - 会议待办自动提取                                    │
│                                                          │
│  2b. 议题归属 → 属于哪个 Topic？                           │
│      - LLM 从内容判定 + Bitable Topic 列表                 │
│      - 映射到 Topic → Decision 树                         │
│                                                          │
│  2c. 关联推理 → 关联哪些实体？                             │
│      - 消息中的文档链接 → docs +fetch                      │
│      - @提及的人员 → contact +get-user                    │
│      - 关联日程 → calendar events.search                   │
│                                                          │
│  2d. 冲突检测 → 与现有决策是否矛盾？                        │
│      - MemoryGraph.detect_conflicts()                     │
│      - impact_level 权重比较                               │
│      - LLM 语义矛盾评估                                    │
└──────────────────────┬───────────────────────────────────┘
                       │ 结构化决策 DecisionNode
                       ▼
┌──────────────────────────────────────────────────────────┐
│                 3. 持久化（Git + Bitable）                  │
│                                                          │
│  3a. Git:                                                 │
│      decisions/{project}/{topic}/{sdr_id}.md             │
│      git add + git commit (原子提交)                       │
│      → git_commit_hash 回传                               │
│                                                          │
│  3b. Bitable:                                             │
│      +record-upsert (含 git_commit_hash)                  │
│      → 建立结构化查询索引                                  │
│                                                          │
│  3c. MemoryGraph 更新                                     │
│      → 加载新决策到内存                                    │
└──────────────────────┬───────────────────────────────────┘
                       │ 完成信号
                       ▼
┌──────────────────────────────────────────────────────────┐
│                 4. 反馈（lark-cli 写操作）                   │
│                                                          │
│  可选: 根据决策内容在飞书中创建/更新实体                    │
│  - task +create → 从会议待办创建飞书任务                   │
│  - docs +update → 在决策文档追加决策记录                    │
│  - im +messages-send → 在群聊中广播决策摘要                 │
└──────────────────────────────────────────────────────────┘
```

### 3.2 协作协议矩阵

| 操作 | OpenClaw Core | lark-cli | Git | Bitable |
|------|--------------|----------|-----|---------|
| **提取决策** | 编排 Pipeline | 执行命令获取数据 | — | 提供 Topic 候选清单 |
| **识别议题** | LLM 推理 + 冲突检测 | 解析联系人 open_id | — | 返回 Topic 列表供 LLM 选择 |
| **写入决策** | 构建 DecisionNode | — | 写 .md + git commit | upsert 记录（含 commit hash） |
| **查询决策** | MemoryGraph 加速 | — | 回退兜底 + git grep | 首选查询接口（WHERE/CONTAINS） |
| **同步校验** | 触发一致性检查 | — | 提供 HEAD commit hash | 对比 git_commit_hash 字段 |
| **事件响应** | 订阅管理 + 调度 | 维护 WebSocket 连接 | — | — |
| **定时轮询** | Cron 调度 | 批量执行命令 | — | — |

### 3.3 同步一致性协议

```
Git → Bitable（正向同步）
  1. git log --after={last_sync_time} 获取新提交
  2. 解析每个提交变更的决策文件
  3. Bitable.upsert_decision(node) 同步到 bitable
  4. 更新 last_sync_time

Bitable → Git（反向同步 — 用户在 Bitable 手动修改）
  1. Bitable.verify_consistency() → 发现 git_commit_hash ≠ HEAD
  2. 读取 Bitable 当前值 vs Git 对应文件
  3. LLM 生成 merge commit / 覆盖写入 / 冲突标记
  4. 更新 git_commit_hash

一致性校验（定时）
  每 N 分钟:
    1. 遍历 Bitable 所有决策记录
    2. 对每条记录: bitable.hash == git.HEAD?hash of 对应文件
    3. 不一致 → 触发修复流程
```

---

## 四、事件驱动 — 完整拓扑

### 4.1 事件类型与处理路由

```
飞书事件                            OpenClaw 处理路由
─────────                          ─────────────────

im.message.receive_v1
  └─ 关键词匹配? ──yes──→ 决策提取 Pipeline
  └─ 包含文档链接? ──yes──→ 关联文档提取

im.message.pin
  └─ 强信号 → 直接进入决策提取 Pipeline

vc.meeting.meeting_ended
  └─ 强信号 → 会议纪要提取 → 决策识别 → 待办任务创建

drive.file.updated_v1
  └─ 属于项目文档? ──yes──→ 决策内容重新提取

drive.file.comment_add_v1
  └─ 包含 /approve? ──yes──→ 决策状态更新

calendar.event.created
  └─ 含"评审"/"决策"关键词? ──yes──→ 关联到对应议题

task.task.updated
  └─ 关联决策的任务完成? ──yes──→ 决策阶段推进

定时轮询（Cron）
  ├─ 每日 09:00 → agenda + 今日决策会议预识别
  ├─ 每日 18:00 → 今日消息回顾 + 决策提取
  ├─ 每周一 09:00 → 上周会议 + 文档 + 任务综合回顾
  └─ 每 N 分钟 → Git ↔ Bitable 一致性校验
```

### 4.2 事件订阅管理

```rust
pub struct EventManager {
    cli: LarkCli,
    subscriptions: Vec<EventSubscription>,
    pipeline_engine: Arc<PipelineEngine>,
}

pub struct EventSubscription {
    pub events: Vec<String>,          // 订阅的事件类型
    pub filter: Option<String>,       // regex filter
    pub output_path: PathBuf,         // 事件日志路径
    pub handler: EventHandler,        // 处理函数
}

pub enum EventHandler {
    /// 直接触发 Pipeline
    DirectPipeline { topic_hint: Option<String> },
    /// 先缓存批量处理
    Buffered { max_batch: usize, interval: Duration },
    /// 仅记录日志，不做实时处理
    LogOnly,
}

impl EventManager {
    /// 启动所有订阅（后台 WebSocket 连接）
    pub async fn start(&self) -> Result<()> {
        for sub in &self.subscriptions {
            let events = sub.events.join(",");
            let mut cmd = vec![
                "event", "+subscribe",
                "--events", &events,
                "--compact",
                "--output", &sub.output_path.to_string_lossy(),
            ];
            if let Some(filter) = &sub.filter {
                cmd.extend(["--filter", filter]);
            }
            // 后台启动 lark-cli event +subscribe
            // 解析 stdout NDJSON 流 → 调用 handler
            self.spawn_subscriber(sub).await?;
        }
        Ok(())
    }

    /// 处理 NDJSON 事件流
    async fn handle_event(&self, raw: &str) -> Result<()> {
        let event: LarkEvent = serde_json::from_str(raw)?;
        match event.r#type.as_str() {
            "im.message.receive_v1" => {
                if self.contains_decision_keywords(&event) {
                    self.pipeline_engine.on_event(event).await?;
                }
            }
            "im.message.pin" | "vc.meeting.meeting_ended" => {
                // 强信号，直接触发
                self.pipeline_engine.on_event(event).await?;
            }
            "drive.file.comment_add_v1" => {
                if self.is_approval_comment(&event) {
                    self.pipeline_engine.on_event(event).await?;
                }
            }
            // ... others
            _ => {}
        }
        Ok(())
    }

    /// 决策关键词检测
    fn contains_decision_keywords(&self, event: &LarkEvent) -> bool {
        let text = event.message_text();
        KEYWORDS.iter().any(|kw| text.contains(kw))
    }
}

const KEYWORDS: &[&str] = &[
    "决定", "确认", "结论", "通过", "定下来", "就这么办",
    "approve", "LGTM", "decided", "confirmed", "agreed",
    "就这么定了", "不再讨论", "最终方案",
];
```

---

## 五、配置与部署

### 5.1 系统配置

```yaml
# openclaw.yaml
project:
  name: "Q2-core-platform"
  topics:
    - "认证模块"
    - "数据库架构"
    - "用户服务"
    - "订单服务"
  phases:
    - "需求评审"
    - "技术选型"
    - "开发"
    - "测试"
    - "上线"

lark_cli:
  bin: "/usr/local/bin/lark-cli"
  default_identity: "user"     # user | bot

git:
  work_dir: "./openclaw-data"
  remote: "git@gitlab.com:team/openclaw-decisions.git"
  auto_push: true
  branch: "main"

bitable:
  base_token: "base_xxxxx"
  tables:
    decision: "tbl_decision"
    topic: "tbl_topic"
    phase: "tbl_phase"
    relation: "tbl_relation"

events:
  subscriptions:
    - events:
        - "im.message.receive_v1"
        - "im.message.pin"
        - "im.message.unpin"
      filter: "^oc_project_"
      handler: "direct_pipeline"
    - events:
        - "vc.meeting.meeting_ended"
      handler: "direct_pipeline"
    - events:
        - "drive.file.created_v1"
        - "drive.file.updated_v1"
        - "drive.file.comment_add_v1"
      handler: "buffered"
      max_batch: 10
      interval_secs: 60
    - events:
        - "calendar.event.created"
        - "calendar.event.updated"
        - "calendar.event.deleted"
      handler: "log_only"
    - events:
        - "task.task.updated"
      handler: "buffered"
      max_batch: 20
      interval_secs: 120

polling:
  daily:
    - time: "09:00"
      tasks:
        - "calendar:agenda"                              # 今日日程
    - time: "18:00"
      tasks:
        - "im:chat-messages --chat-id oc_xxx"            # 今日消息
        - "vc:search --range today"                       # 今日会议
  weekly:
    - day: "monday"
      time: "09:00"
      tasks:
        - "docs:search --query <project>"                 # 本周新增文档
        - "vc:search --range last-week"                   # 上周会议
        - "task:related --page-size 100"                  # 本周任务
  consistency_check:
    interval_minutes: 30
    auto_fix: false

memory:
  preload_on_start: true          # 启动时从 Git 加载全量决策到内存
  max_cache_size: 10000           # 内存中最多缓存的决策数
  dirty_flush_interval_secs: 10   # 脏记录批量刷入 Git 的间隔
```

### 5.2 目录结构

```
openclaw/
├── Cargo.toml / package.json       # 项目依赖
├── openclaw.yaml                   # 系统配置
│
├── src/
│   ├── main.rs                     # 入口 + 初始化
│   │
│   ├── core/
│   │   ├── mod.rs
│   │   ├── pipeline.rs             # PipelineEngine
│   │   ├── memory_graph.rs         # MemoryGraph
│   │   ├── decision.rs             # DecisionNode
│   │   ├── topic.rs                # Topic 管理
│   │   ├── phase.rs                # Phase 管理
│   │   ├── relation.rs             # Relation 类型
│   │   ├── conflict.rs             # ConflictDetector
│   │   └── types.rs                # 公共类型
│   │
│   ├── adapters/
│   │   ├── mod.rs
│   │   ├── lark_cli.rs             # LarkCli Adapter
│   │   ├── cli_executor.rs         # 命令执行器
│   │   ├── git_storage.rs          # GitStorage Adapter
│   │   └── bitable_store.rs        # BitableStore Adapter
│   │
│   ├── extractors/
│   │   ├── mod.rs
│   │   ├── im_extractor.rs         # IM 消息提取
│   │   ├── meeting_extractor.rs     # 会议纪要提取
│   │   ├── doc_extractor.rs        # 文档提取
│   │   ├── calendar_extractor.rs   # 日程提取
│   │   ├── task_extractor.rs       # 任务提取
│   │   └── okr_extractor.rs        # OKR 提取
│   │
│   ├── processors/
│   │   ├── mod.rs
│   │   ├── decision_identifier.rs  # 决策识别器
│   │   ├── topic_classifier.rs     # 议题归属分类器
│   │   └── relation_linker.rs      # 关联推理器
│   │
│   ├── sync/
│   │   ├── mod.rs
│   │   ├── git_bitable_sync.rs     # Git ↔ Bitable 同步
│   │   └── consistency.rs          # 一致性校验
│   │
│   └── config/
│       ├── mod.rs
│       └── settings.rs             # 配置加载
│
├── openclaw-data/                  # Git 工作目录（自动管理）
│   ├── decisions/
│   │   ├── {project}/
│   │   │   ├── {topic}/
│   │   │   │   ├── {sdr_id}.md
│   │   │   │   └── ...
│   │   │   └── topic-index.md
│   │   ├── conflicts/
│   │   │   └── {conflict_id}.md
│   │   └── L0_RULES.md
│   └── .git/
│
└── events/                         # 事件订阅日志
    ├── im.log
    ├── docs.log
    ├── calendar.log
    ├── task.log
    └── vc.log
```

### 5.3 启动流程

```
1. 加载配置 openclaw.yaml
2. 初始化 GitStorage
   ├── open/init Git 仓库
   ├── 拉取远程最新提交
   └── 确保目录结构存在
3. 初始化 MemoryGraph
   └── 从 Git 加载全量决策（preload_on_start=true）
4. 初始化 BitableStore
   └── 验证 base_token 和表结构
5. 初始化 LarkCli Adapter
   └── 验证 lark-cli 可用性
6. 启动 EventManager
   ├── 建立所有 WebSocket 订阅
   ├── 注册事件 Handler → PipelineEngine
   └── 启动 NDJSON 流解析
7. 启动定时轮询
   ├── 注册每日任务（Cron）
   ├── 注册每周任务
   └── 注册一致性校验
8. 系统就绪 → 进入事件循环
```

---

## 六、四者协作速查

### 6.1 各司其职

| 你想做什么 | 用哪个组件 | 方式 |
|-----------|-----------|------|
| **提取群聊决策** | lark-cli → OpenClaw | `im +messages-search` → Pipeline |
| **提取会议结论** | lark-cli → OpenClaw | `vc +search` + `+notes` → Pipeline |
| **查看所有 active 决策** | Bitable | 按 Topic 分组视图 |
| **跨议题搜索关联决策** | Bitable | `cross_topic_refs CONTAINS` |
| **查看决策修改历史** | Git | `git log -- {sdr_id}.md` |
| **查看谁在某天改了什么** | Git | `git blame {file}` |
| **按阶段筛选生效决策** | Bitable | 按 phase 字段过滤 |
| **回滚错误决策** | Git | `git revert {commit}` |
| **全文搜索决策内容** | Git | `git grep` |
| **一致性校验** | OpenClaw | `verify_consistency()` 对比 hash |
| **实时感知变化** | lark-cli → OpenClaw | `event +subscribe` WebSocket |
| **创建待办任务** | lark-cli | `task +create`（OpenClaw 编排） |
| **启动系统** | OpenClaw | `cargo run / npm start` |

### 6.2 数据状态 & 责任人

```
                    ┌──────────┐
                    │  飞书服务   │  ← 飞书官方维护
                    └─────┬─────┘
                          │  JSON/HTTP
                    ┌─────▼─────┐
                    │  lark-cli  │  ← Feishu OpenAPI SDK
                    │  (命令)    │     开发者维护 cli
                    └─────┬─────┘
                          │  stdout NDJSON
                    ┌─────▼─────┐
                    │ OpenClaw  │  ← 本项目核心
                    │  (进程)    │     开发者维护
                    └──┬───┬───┘
                       │   │
               ┌───────┘   └───────┐
               ▼                   ▼
        ┌──────────┐        ┌──────────┐
        │   Git    │        │ Bitable  │  ← 开发者维护
        │ (远端)   │        │ (飞书)   │     两者双向同步
        └──────────┘        └──────────┘
```

### 6.3 关键设计原则

1. **Git 是权威，Bitable 是接口**：所有决策的最终真实来源是 Git 中的 `.md` 文件。Bitable 提供 Git 做不到的结构化查询能力（WHERE、CONTAINS、分组、排序），删掉 Bitable 系统退化为纯 Git + 本地检索。

2. **lark-cli 只负责 I/O，不负责决策**：lark-cli 是 OpenClaw 的"感官和手"——感知飞书世界的变化、执行飞书世界的写操作。所有推理、关联、冲突检测都在 OpenClaw Core 内完成。

3. **事件驱动 + 定时轮询双通道**：实时事件（WebSocket）负责即时触发，定时轮询（Cron）负责兜底补全。事件缺失不影响系统完整性，只影响响应时效。

4. **增量处理**：每次只处理新数据（按时间戳/偏移量/commit hash），不重放历史。MemoryGraph 启动时从 Git 全量加载，之后只增量更新。

5. **冲突不自动解决**：当 `impact_level` 权重相当且语义矛盾 > 0.6 时，创建 `CONFLICTS_WITH` 关系并等待人工裁决——不自动覆盖。

6. **Hash 锚定一致性**：`git_commit_hash` 是 Git ↔ Bitable 间唯一的同步锚点。任何不一致都可以通过对比 hash 发现并修复。
