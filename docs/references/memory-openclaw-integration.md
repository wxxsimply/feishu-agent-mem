# 记忆系统与 OpenClaw 协作方案

> 飞书服务器部署的 OpenClaw 已接入 lark-cli，可通过飞书对话窗口操控。本文档规划 24 小时自动运行的记忆系统如何与 OpenClaw Core 协作，核心约束：避免长上下文导致决策失准，采用渐进式披露模式，不将整个记忆系统设计为 OpenClaw skills。

---

## 一、问题定义

### 1.1 现状

- OpenClaw 已部署在飞书服务器，接入 lark-cli
- 可通过飞书对话窗口直接操控 OpenClaw
- 记忆系统的设计文档已完成（决策树、提取方案、架构、信号引擎、Git 运作）

### 1.2 核心矛盾

```
记忆系统的数据量（Git + Bitable + 飞书 8 个适配器）
    vs
OpenClaw LLM 的有效上下文窗口

矛盾: 记忆越丰富 → 上下文越长 → 决策越失准
```

### 1.3 解决思路

参考 Claude Code 的渐进式披露模式——不把所有信息一次性喂给 LLM，而是：

1. **始终可见**：轻量索引（~800 tokens），让 LLM 知道"有什么"
2. **按需获取**：结构化工具，让 LLM 决定"查什么"
3. **验证后深入**：完整内容，仅对已确认相关的信息展开

### 1.4 不做什么

- **不把记忆系统设计为 OpenClaw 的 skills**：记忆系统是独立的后台服务，不是 OpenClaw 的插件
- **不把 Git 操作暴露给 OpenClaw**：OpenClaw 不直接读写 Git，通过记忆系统的 API 间接操作
- **不把原始飞书事件直接转发给 OpenClaw**：经过记忆系统过滤和装配后的精简信号才进入 OpenClaw

---

## 二、整体架构

### 2.1 组件关系

```
┌─────────────────────────────────────────────────────────────┐
│                        飞书服务                               │
│  IM / VC / Calendar / Docs / Task / OKR / Contact / Wiki    │
└──────────────┬──────────────────────────────┬───────────────┘
               │ 事件 + 数据                    │
               ▼                               ▼
┌──────────────────────────┐    ┌──────────────────────────────┐
│    Memory System         │    │       OpenClaw Core           │
│    (24h 后台服务)         │    │    (飞书上的 Agent)           │
│                          │    │                               │
│  ┌────────────────────┐  │    │  ┌──────────────────────┐    │
│  │ Signal Activation  │  │    │  │  LLM 推理引擎         │    │
│  │ Engine             │  │    │  │  (Claude / GPT)       │    │
│  │                    │  │    │  └──────────┬───────────┘    │
│  │ 8 适配器 → 信号    │  │    │             │                │
│  │ 路由 → 装配       │  │    │  ┌──────────▼───────────┐    │
│  │ 状态机 → 变更      │  │    │  │  Memory MCP Server   │    │
│  └────────┬───────────┘  │◄───│  │  (渐进式披露工具)     │    │
│           │              │    │  └──────────────────────┘    │
│  ┌────────▼───────────┐  │    │                               │
│  │ Git + Bitable      │  │    │  ┌──────────────────────┐    │
│  │ (持久化层)          │  │    │  │  lark-cli 执行器      │    │
│  └────────────────────┘  │    │  │  (飞书写操作)         │    │
│                          │    │  └──────────────────────┘    │
│  ┌────────────────────┐  │    │                               │
│  │ Context Package    │──│────│  信号/上下文包 → OpenClaw     │
│  │ Server (MCP)       │  │    │  OpenClaw 决策 → lark-cli    │
│  └────────────────────┘  │    └──────────────────────────────┘
└──────────────────────────┘
```

### 2.2 职责划分

| 组件 | 职责 | 运行方式 |
|------|------|---------|
| **Memory System** | 信号检测、上下文装配、决策状态管理、Git/Bitable 持久化 | 24h 后台服务（systemd / Docker） |
| **Memory MCP Server** | 向 OpenClaw 暴露渐进式披露工具 | 随 Memory System 启动 |
| **OpenClaw Core** | LLM 推理、用户交互、飞书写操作执行 | OpenClaw 平台托管 |
| **lark-cli** | 飞书 API 的命令行封装 | OpenClaw 已集成 |

### 2.3 数据流方向

```
方向 1: 飞书 → 记忆系统（感知）
  飞书事件 → lark-cli event subscribe → 适配器 → 信号 → 记忆系统

方向 2: 记忆系统 → OpenClaw（通知 + 上下文）
  记忆系统检测到需要 LLM 判断时 → 组装精简上下文包 → 推送到 OpenClaw

方向 3: OpenClaw → 记忆系统（查询）
  OpenClaw 在推理过程中 → 通过 MCP 工具查询记忆系统 → 获取更多上下文

方向 4: OpenClaw → 飞书（执行）
  OpenClaw 做出决策后 → 调用 lark-cli 执行飞书写操作

方向 5: 记忆系统 → Git + Bitable（持久化）
  决策确认后 → 写入 Git → 同步 Bitable
```

---

## 三、渐进式披露设计

### 3.1 三层信息架构

参考 Claude-Mem 的渐进式披露模式，将记忆系统的信息分为三层：

```
Layer 1 — 索引层（始终可见，~800 tokens）
  ├── 项目当前阶段
  ├── 各议题的 active 决策数量
  ├── 最近 3 条决策的摘要
  ├── 当前未解决的冲突数
  └── 最近信号的简报

Layer 2 — 上下文层（按需获取，~2K tokens/次）
  ├── 某议题下的所有 active 决策列表
  ├── 某决策的完整 YAML 字段
  ├── 某适配器的最近信号汇总
  ├── 跨议题关联图谱
  └── 某阶段的决策时间线

Layer 3 — 详情层（验证后深入，~500 tokens/项）
  ├── 某决策的完整 Markdown 正文
  ├── 某决策的 Git 历史（blame/log）
  ├── 某信号的原始飞书内容
  ├── 某冲突的完整描述
  └── 某决策的影响链分析
```

### 3.2 索引层内容设计

索引层是 OpenClaw 每次交互时**自动注入**的上下文，类似 Claude Code 的 CLAUDE.md：

```markdown
## 记忆系统索引（自动生成，最后刷新: 2026-04-29 14:30）

### 项目状态
- 项目: Q2-core-platform | 阶段: 开发中 | 活跃决策: 23 条

### 议题概览
| 议题 | Active | 最近变更 | 影响级别分布 |
|------|--------|---------|------------|
| 认证模块 | 5 | 2h 前 | major:2, minor:3 |
| 数据库架构 | 8 | 1d 前 | critical:1, major:3, minor:4 |
| 用户服务 | 4 | 3d 前 | major:1, minor:3 |
| 订单服务 | 6 | 6h 前 | major:2, minor:4 |

### 最近决策（最近 7 天）
- [dec_20260428_001] 数据库主键变更 → major, active
- [dec_20260427_003] 认证流程优化 → minor, active
- [dec_20260425_002] 缓存策略调整 → minor, active

### 待处理
- 未解决冲突: 1 条（dec_20260420_003 vs dec_20260428_001）
- 待确认信号: 2 条（来自 IM 适配器）

### 可用工具
- `memory.search` — 搜索决策（返回摘要列表）
- `memory.topic` — 查看某议题详情
- `memory.decision` — 查看某决策详情
- `memory.timeline` — 查看时间线
- `memory.conflict` — 查看冲突详情
- `memory.signal` — 查看最近信号
```

索引层的成本：~800 tokens。这 800 tokens 让 OpenClaw 知道：
- 系统中有什么
- 哪里有值得深入的信息
- 可以用什么工具获取更多

### 3.3 工具层设计（MCP Server）

Memory MCP Server 暴露 6 个工具给 OpenClaw，遵循"先搜索、再上下文、最后详情"的模式：

```
工具 1: memory.search
  输入: { query: string, topic?: string, impact_level?: string }
  输出: 决策摘要列表（每条 ~50 tokens）
  成本: ~500 tokens（10 条结果）
  用途: OpenClaw 在回答用户前，静默搜索相关决策

工具 2: memory.topic
  输入: { topic_name: string }
  输出: 该议题下所有 active 决策的摘要列表 + 跨议题引用
  成本: ~1K tokens
  用途: 了解某个议题的完整决策图谱

工具 3: memory.decision
  输入: { sdr_id: string, include_full?: boolean }
  输出: 决策的 YAML 字段（include_full=true 时含 Markdown 正文）
  成本: ~300 tokens（字段）/ ~800 tokens（含正文）
  用途: 获取单个决策的完整信息

工具 4: memory.timeline
  输入: { project: string, days?: number }
  输出: 按时间排序的决策变更列表
  成本: ~600 tokens
  用途: 了解项目的决策演变

工具 5: memory.conflict
  输入: { conflict_id?: string }
  输出: 冲突详情（冲突双方、矛盾点、当前状态）
  成本: ~400 tokens
  用途: 查看未解决的决策冲突

工具 6: memory.signal
  输入: { adapter?: string, strength?: string, limit?: number }
  输出: 最近的信号列表（来源、强度、摘要）
  成本: ~300 tokens
  用途: 查看最近的飞书状态变化信号
```

### 3.4 渐进式披露的交互模式

```
场景: 用户在飞书对话中问 "数据库迁移进展如何？"

Step 1: OpenClaw 收到用户消息
  ├── 自动注入: 索引层 (~800 tokens)
  └── 用户消息: "数据库迁移进展如何？"

Step 2: OpenClaw 静默调用 memory.search
  ├── query: "数据库迁移"
  ├── 结果: 3 条相关决策的摘要 (~300 tokens)
  └── 此时上下文: 800 + 300 = ~1,100 tokens

Step 3: OpenClaw 判断需要更多上下文
  ├── 调用 memory.topic("数据库架构")
  ├── 结果: 8 条 active 决策列表 (~1K tokens)
  └── 此时上下文: 1,100 + 1,000 = ~2,100 tokens

Step 4: OpenClaw 需要查看关键决策的详情
  ├── 调用 memory.decision("dec_20260420_003", include_full=true)
  ├── 结果: 完整决策内容 (~800 tokens)
  └── 此时上下文: 2,100 + 800 = ~2,900 tokens

Step 5: OpenClaw 组装回答
  ├── 基于 ~2,900 tokens 的精简上下文
  ├── 不需要加载整个记忆系统的数据
  └── 回答: "数据库迁移已完成 DDL 脚本编写，迁移脚本正在测试中..."

总上下文: ~2,900 tokens（vs 全量加载可能 > 30K tokens）
```

---

## 四、记忆系统作为独立后台服务

### 4.1 服务架构

```
memory-system (Docker / systemd)
│
├── 主进程
│   ├── EventManager          # 飞书事件订阅管理
│   ├── SignalActivationEngine # 信号 → 装配 → 状态机
│   ├── PipelineEngine        # 决策变更执行
│   └── Scheduler             # 定时任务（轮询、一致性校验）
│
├── 存储层
│   ├── GitStorage            # Git 读写
│   ├── BitableStore          # Bitable 读写
│   └── MemoryGraph           # 内存索引
│
├── MCP Server (端口 37777)
│   ├── memory.search
│   ├── memory.topic
│   ├── memory.decision
│   ├── memory.timeline
│   ├── memory.conflict
│   └── memory.signal
│
└── 通知器
    ├── FeishuNotifier        # 向飞书群/用户推送信号摘要
    └── OpenClawBridge        # 向 OpenClaw 推送上下文包
```

### 4.2 服务启动流程

```
1. 加载配置 (memory-system.yaml)
2. 初始化 GitStorage
   ├── 打开/初始化 Git 仓库
   ├── 拉取远程最新
   └── 安装 hooks
3. 初始化 MemoryGraph
   └── 从 Git 加载全量决策到内存
4. 初始化 BitableStore
   └── 验证 base_token 和表结构
5. 初始化 EventManager
   ├── 建立飞书 WebSocket 订阅
   └── 注册事件处理器
6. 初始化 SignalActivationEngine
   ├── 加载权重矩阵
   └── 注册适配器发射器
7. 启动 MCP Server
   ├── 监听 localhost:37777
   └── 注册 6 个工具
8. 启动 Scheduler
   ├── 每日轮询任务
   ├── 一致性校验
   └── 索引刷新
9. 服务就绪 → 日志输出
```

### 4.3 服务配置

```yaml
# memory-system.yaml
service:
  name: "openclaw-memory"
  port: 37777                  # MCP Server 端口
  log_level: "info"

git:
  work_dir: "./openclaw-data"
  remote: "git@gitlab.com:team/openclaw-decisions.git"
  auto_push: true

bitable:
  base_token: "base_xxxxx"
  tables:
    decision: "tbl_decision"
    topic: "tbl_topic"

openclaw:
  # 如何通知 OpenClaw
  notify_method: "mcp"         # mcp | webhook | feishu_message
  # MCP 注册路径
  mcp_config_path: "~/.openclaw/openclaw.json"
  # 飞书通知（备选）
  feishu_webhook: ""

events:
  subscriptions:
    - events: ["im.message.receive_v1", "im.message.pin"]
      filter: "^oc_project_"
      handler: "direct_pipeline"
    - events: ["vc.meeting.meeting_ended"]
      handler: "direct_pipeline"
    - events: ["task.task.updated"]
      handler: "buffered"
      interval_secs: 120

polling:
  daily:
    - time: "09:00"
      tasks: ["calendar:agenda"]
    - time: "18:00"
      tasks: ["im:chat-messages", "vc:search"]
  consistency_check:
    interval_minutes: 30

context:
  index_refresh_interval_minutes: 15
  max_index_tokens: 800
  max_context_tokens: 2000
  max_detail_tokens: 500
```

---

## 五、记忆系统如何通知 OpenClaw

### 5.1 通知时机

不是所有信号都需要通知 OpenClaw。只有当信号需要 **LLM 做出判断** 时才通知：

```
需要通知 OpenClaw 的场景:
  ├── 新信号到达，记忆系统无法自动判定决策归属（需要 LLM 判断 Topic）
  ├── 检测到语义冲突（需要 LLM 判断是否真正矛盾）
  ├── 状态机触发关键转换（如 执行中 → 已完成，需要 LLM 确认）
  ├── 用户在飞书中主动询问决策相关问题
  └── 冲突需要人工裁决（通知用户 + OpenClaw）

不需要通知的场景:
  ├── 信号强度为 Weak（直接记录，不触发 LLM）
  ├── 决策归属明确（Topic 已知，无跨议题影响）
  ├── 状态转换是机械的（如 Point 决策的 phase 自动切换）
  └── Bitable 同步操作（纯技术操作）
```

### 5.2 通知方式

采用**精简信号包**而非完整上下文：

```
通知包结构 (~300 tokens):
  {
    "signal_type": "new_decision_candidate",
    "summary": "IM 群聊中讨论了数据库字段变更",
    "adapter": "IM",
    "strength": "medium",
    "suggested_topic": "数据库架构",
    "confidence": 0.75,
    "related_decisions": ["dec_20260420_003"],  // 仅 ID
    "action_needed": "confirm_topic_and_check_conflicts",
    "context_available_via": "memory.topic('数据库架构')"
  }
```

OpenClaw 收到通知后，**自行决定**是否需要通过 MCP 工具获取更多上下文。

### 5.3 通知通道

```
方案 A: MCP Server 推送（推荐）
  ├── 记忆系统的 MCP Server 维护一个通知队列
  ├── OpenClaw 启动时查询待处理通知
  ├── 通过 memory.signal 工具拉取最新信号
  └── 优点: OpenClaw 主动拉取，不被打断

方案 B: 飞书消息推送
  ├── 记忆系统通过 lark-cli 向指定群/用户发送消息
  ├── 消息格式: 结构化的信号摘要卡片
  ├── OpenClaw 在飞书中看到消息后触发处理
  └── 优点: 可见性高，用户也能看到

方案 C: Webhook 回调
  ├── 记忆系统通过 HTTP 向 OpenClaw 的 webhook 端点推送
  ├── OpenClaw 被动接收
  └── 优点: 实时性最高
```

**推荐方案 A + B 组合**：MCP 供 OpenClaw 主动查询，飞书消息用于需要用户参与的场景。

---

## 六、OpenClaw 如何调用记忆系统

### 6.1 MCP 注册

记忆系统启动时，自动将 MCP Server 注册到 OpenClaw 的配置中：

```json
// ~/.openclaw/openclaw.json 中追加
{
  "mcpServers": {
    "memory": {
      "command": "node",
      "args": ["./memory-system/mcp-server.js"],
      "env": {
        "MEMORY_SERVICE_URL": "http://localhost:37777"
      }
    }
  }
}
```

OpenClaw 启动后自动连接 MCP Server，获得 6 个 memory.* 工具。

### 6.2 OpenClaw 的决策流程

```
用户在飞书中发送消息 / 飞书事件触发
    │
    ▼
OpenClaw 收到输入
    │
    ├── Step 1: 注入索引层（~800 tokens，自动）
    │   └── 来自 MCP Server 的 memory.index() 或本地缓存
    │
    ├── Step 2: 静默搜索（~500 tokens，自动）
    │   └── 调用 memory.search(query=用户消息关键词)
    │
    ├── Step 3: LLM 推理
    │   ├── 基于索引 + 搜索结果
    │   ├── 判断是否需要更多上下文
    │   └── 如果需要 → 调用 memory.topic / memory.decision / memory.timeline
    │
    ├── Step 4: 做出决策
    │   ├── 决策内容
    │   ├── 影响级别判断
    │   ├── 跨议题标记
    │   └── 状态转换
    │
    └── Step 5: 执行动作
        ├── 调用 lark-cli 执行飞书写操作
        └── 通知记忆系统持久化决策（通过 MCP 工具 memory.save_decision）
```

### 6.3 记忆系统的 MCP 工具详细设计

#### 6.3.1 memory.search — 搜索决策

```
用途: OpenClaw 在处理用户请求前，静默搜索相关决策
触发: 每次用户消息到达时自动调用，或 OpenClaw 主动调用

输入:
  {
    "query": "数据库迁移",           // 搜索关键词
    "topic": "数据库架构",           // 可选：限定议题
    "impact_level": "major",         // 可选：限定影响级别
    "status": "active",              // 可选：限定状态，默认 active
    "limit": 10                      // 可选：返回数量，默认 10
  }

输出:
  {
    "results": [
      {
        "sdr_id": "dec_20260420_003",
        "title": "数据库主键从 INT 改为 BIGINT",
        "topic": "数据库架构",
        "impact_level": "major",
        "status": "active",
        "phase": "dev",
        "decided_at": "2026-04-20",
        "relevance_score": 0.92       // 语义相关度
      }
      // ... 更多结果
    ],
    "total": 3,
    "token_cost": 300                 // 本次查询消耗的 token 数
  }

内部实现:
  1. 先查 MemoryGraph（内存索引，O(1)）
  2. 再查 Bitable（跨议题 cross_topic_refs CONTAINS）
  3. 按语义相关度排序
  4. 返回摘要（不含完整正文）
```

#### 6.3.2 memory.topic — 查看议题

```
用途: 了解某个议题的完整决策图谱
触发: OpenClaw 判断需要了解某议题全貌时

输入:
  {
    "topic_name": "数据库架构",
    "include_cross_refs": true        // 是否包含跨议题引用
  }

输出:
  {
    "topic": "数据库架构",
    "active_decisions": [
      {
        "sdr_id": "dec_20260420_003",
        "title": "主键类型变更",
        "impact_level": "major",
        "parent": null,
        "children": ["dec_20260422_001"]
      }
      // ...
    ],
    "cross_topic_refs": [
      {
        "sdr_id": "dec_20260415_002",
        "from_topic": "认证模块",
        "relation": "影响数据库 Schema"
      }
    ],
    "total_active": 8,
    "token_cost": 1000
  }
```

#### 6.3.3 memory.decision — 查看决策详情

```
用途: 获取单个决策的完整信息
触发: OpenClaw 需要深入查看某个决策时

输入:
  {
    "sdr_id": "dec_20260420_003",
    "include_full": false,            // true = 含 Markdown 正文
    "include_history": false,         // true = 含 Git 历史
    "include_relations": true         // true = 含关联决策
  }

输出:
  {
    "sdr_id": "dec_20260420_003",
    "title": "数据库主键从 INT 改为 BIGINT",
    "decision": "将 user_sessions 表的 session_token 字段...",
    "rationale": "新的 JWT 格式需要更长的 token 字段...",
    "topic": "数据库架构",
    "phase": "dev",
    "impact_level": "major",
    "cross_topic_refs": ["认证模块", "用户服务"],
    "status": "active",
    "proposer": "张三",
    "executor": "李四",
    "relations": [
      {"type": "DEPENDS_ON", "target": "dec_20260415_002"}
    ],
    "full_content": "...",            // 仅 include_full=true 时
    "git_history": [...],             // 仅 include_history=true 时
    "token_cost": 300 / 800           // 取决于 include_full
  }
```

#### 6.3.4 memory.timeline — 时间线

```
用途: 了解项目的决策演变
触发: OpenClaw 需要了解决策的时间顺序时

输入:
  {
    "project": "Q2-core-platform",
    "topic": "数据库架构",           // 可选
    "days": 30                       // 可选，默认 30
  }

输出:
  {
    "timeline": [
      {
        "date": "2026-04-28",
        "events": [
          {
            "sdr_id": "dec_20260428_001",
            "action": "created",
            "title": "主键类型变更",
            "actor": "张三"
          }
        ]
      }
      // ...
    ],
    "token_cost": 600
  }
```

#### 6.3.5 memory.conflict — 冲突详情

```
用途: 查看未解决的决策冲突
触发: 索引层提示有未解决冲突时

输入:
  {
    "conflict_id": "conf_20260429_001"  // 可选，不传则返回所有
  }

输出:
  {
    "conflicts": [
      {
        "conflict_id": "conf_20260429_001",
        "decision_a": {
          "sdr_id": "dec_20260420_003",
          "title": "主键类型变更",
          "impact_level": "major"
        },
        "decision_b": {
          "sdr_id": "dec_20260428_001",
          "title": "新认证方案",
          "impact_level": "major"
        },
        "contradiction_score": 0.72,
        "description": "两决策对 session_token 字段长度要求不一致",
        "status": "pending_resolution"
      }
    ],
    "token_cost": 400
  }
```

#### 6.3.6 memory.signal — 最近信号

```
用途: 查看最近的飞书状态变化信号
触发: 索引层提示有待处理信号时

输入:
  {
    "adapter": "IM",                  // 可选
    "strength": "medium",             // 可选
    "limit": 5                        // 可选，默认 5
  }

输出:
  {
    "signals": [
      {
        "signal_id": "sig_20260429_001",
        "adapter": "IM",
        "strength": "medium",
        "summary": "群聊讨论了数据库字段变更",
        "timestamp": "2026-04-29T14:30:00+08:00",
        "action_taken": "candidate_decision_created",
        "decision_id": "dec_20260429_001"
      }
    ],
    "token_cost": 300
  }
```

---

## 七、记忆系统如何与 Bitable 协作

### 7.1 协作时机

```
写入时机（Git → Bitable）:
  ├── 决策新增/修改后 → Git commit → Bitable upsert
  ├── 状态变更后 → Git commit → Bitable update
  └── 冲突解决后 → Git commit → Bitable update

读取时机（Bitable → 记忆系统）:
  ├── 跨议题查询 → Bitable CONTAINS → 补充结果
  ├── 用户在 Bitable 手动修改 → Webhook → 反向同步到 Git
  └── 一致性校验 → 对比 git_commit_hash
```

### 7.2 上下文装配中的 Bitable 角色

当 Signal Activation Engine 装配上下文时，Bitable 提供 Git 做不到的查询能力：

```
信号到达 → 激活路由器确定需要查询的目标
    │
    ├── 同议题查询 → 直接查 MemoryGraph（内存，最快）
    │   └── WHERE topic = "数据库架构" AND status = "active"
    │
    ├── 跨议题查询 → 查 Bitable
    │   └── WHERE cross_topic_refs CONTAINS "用户服务"
    │   └── Git 做不到这个查询
    │
    ├── 按阶段过滤 → 查 Bitable
    │   └── WHERE phase CONTAINS "dev"
    │   └── Git 需要遍历所有文件
    │
    └── 按人员查询 → 查 Bitable
        └── WHERE proposer = "张三" OR executor = "张三"
        └── Git 需要 grep 所有文件
```

### 7.3 避免冗长上下文的策略

```
策略 1: 只传 ID，不传内容
  记忆系统 → OpenClaw: "相关决策: dec_20260420_003, dec_20260428_001"
  OpenClaw 需要时: 调用 memory.decision() 获取详情

策略 2: 摘要先行，详情按需
  记忆系统 → OpenClaw: "数据库架构议题有 8 条 active 决策"
  OpenClaw 需要时: 调用 memory.topic() 获取列表

策略 3: 评分过滤，只推高相关度
  记忆系统内部: 对候选决策评分，只推送 score > 0.7 的
  OpenClaw 收到: 精简的高相关度决策列表

策略 4: 时间窗口限制
  记忆系统: 只推送最近 N 天的决策变更
  OpenClaw 需要历史: 调用 memory.timeline() 回溯
```

---

## 八、OpenClaw Skill 设计（仅暴露接口层）

记忆系统**不是** OpenClaw 的 skill，但需要一个轻量的 skill 作为**接口层**，让 OpenClaw 知道如何使用记忆系统：

### 8.1 Skill 职责

```
这个 skill 做的事:
  ├── 告诉 OpenClaw 记忆系统的存在和能力
  ├── 提供 MCP 工具的使用说明
  ├── 在用户消息到达时，自动触发 memory.search
  └── 在决策产生时，调用 memory.save_decision

这个 skill 不做的事:
  ├── 不包含记忆系统的核心逻辑
  ├── 不直接读写 Git
  ├── 不直接操作 Bitable
  └── 不处理飞书事件
```

### 8.2 Skill 配置

```json
// openclaw-memory-skill/skill.json
{
  "name": "memory-interface",
  "version": "1.0.0",
  "description": "OpenClaw 记忆系统接口层",
  "capabilityTags": ["memory", "decision-tracking"],
  "mcpServers": {
    "memory": {
      "command": "node",
      "args": ["./mcp-proxy.js"],
      "env": {
        "MEMORY_SERVICE_URL": "http://localhost:37777"
      }
    }
  },
  "hooks": {
    "onMessage": {
      "action": "memory.search",
      "params": {
        "query": "${user_message}",
        "limit": 5
      }
    },
    "onDecisionMade": {
      "action": "memory.save_decision",
      "params": {
        "decision": "${decision_content}",
        "topic": "${detected_topic}"
      }
    }
  }
}
```

### 8.3 Skill 的系统提示词

```markdown
## 记忆系统

你有持久化的项目记忆能力。在回答用户问题前：

1. **先搜索**: 静默调用 memory.search 搜索相关决策
2. **再判断**: 基于搜索结果判断是否需要更多上下文
3. **按需获取**: 调用 memory.topic / memory.decision / memory.timeline 获取详情
4. **回答**: 基于精简但充分的上下文回答

### 可用工具
- `memory.search(query, topic?, impact_level?)` — 搜索决策摘要
- `memory.topic(topic_name)` — 查看某议题的决策图谱
- `memory.decision(sdr_id, include_full?)` — 查看决策详情
- `memory.timeline(project, days?)` — 查看决策时间线
- `memory.conflict(conflict_id?)` — 查看冲突详情
- `memory.signal(adapter?, strength?)` — 查看最近信号

### 规则
- 不要在回答中暴露工具调用细节
- 如果搜索无结果，正常回答（不要提及"记忆系统中没有找到"）
- 冲突需要用户裁决时，清晰呈现双方决策的矛盾点
```

---

## 九、完整协作场景走读

### 场景 A：用户在飞书中询问

```
用户: "数据库迁移进展如何？"

OpenClaw 内部流程:
  1. 注入索引层 (~800 tokens)
     └── 知道有"数据库架构"议题，8 条 active 决策

  2. 静默调用 memory.search(query="数据库迁移")
     └── 返回 3 条相关决策摘要 (~300 tokens)
     └── 总上下文: ~1,100 tokens

  3. LLM 推理: 搜索结果中有 dec_20260420_003 "主键类型变更"
     └── 调用 memory.decision("dec_20260420_003", include_full=true)
     └── 获取完整决策 (~800 tokens)
     └── 总上下文: ~1,900 tokens

  4. LLM 推理: 决策状态为 "executing"
     └── 调用 memory.timeline("Q2-core-platform", topic="数据库架构", days=7)
     └── 获取最近变更 (~600 tokens)
     └── 总上下文: ~2,500 tokens

  5. 组装回答: "数据库主键变更决策已于 4/20 确认，当前处于执行阶段。
     DDL 脚本已完成，迁移脚本正在测试中。预计本周内完成。"

总上下文: ~2,500 tokens（vs 全量 > 30K tokens）
```

### 场景 B：记忆系统检测到新信号

```
飞书事件: 群聊消息 "决定了，用 Redis 做缓存"

记忆系统内部:
  1. IM 适配器发射信号 (strength=Medium, keywords=["Redis","缓存"])
  2. 激活路由: IM/Medium → {Task:0.6, Docs:0.7, VC:0.5}
  3. 上下文装配: MemoryGraph 查到"缓存方案"议题有 2 条 active 决策
  4. 状态机: 无匹配的已有决策 → 创建候选决策 (status: pending)

记忆系统 → OpenClaw:
  通知包 (~300 tokens):
  {
    "signal_type": "new_decision_candidate",
    "summary": "IM 群聊确认使用 Redis 做缓存",
    "suggested_topic": "缓存方案",
    "confidence": 0.80,
    "related_decisions": ["dec_20260410_001"],
    "action_needed": "confirm_and_create_decision"
  }

OpenClaw 内部:
  1. 收到通知包
  2. 调用 memory.topic("缓存方案") 了解现有决策
  3. LLM 判断: 确认 Topic，检查是否与现有决策冲突
  4. 创建新决策 → 调用 memory.save_decision(...)
  5. 在飞书群聊中确认: "已记录决策: 使用 Redis 做缓存"
```

### 场景 C：任务完成推进决策状态

```
飞书事件: 任务 "数据库迁移脚本" 标记完成

记忆系统内部:
  1. Task 适配器发射信号 (strength=Strong, primary_id=task_xxx)
  2. 激活路由: 查 MemoryGraph → 找到关联决策 dec_20260420_003
  3. 检查所有关联任务 → 全部完成
  4. 状态机: executing + all_tasks_completed → completed

记忆系统 → OpenClaw:
  通知包 (~200 tokens):
  {
    "signal_type": "decision_status_change",
    "summary": "数据库主键变更决策的所有关联任务已完成",
    "decision_id": "dec_20260420_003",
    "old_status": "executing",
    "new_status": "completed",
    "action_needed": "confirm_completion"
  }

OpenClaw 内部:
  1. 收到通知包
  2. LLM 确认: 状态变更合理
  3. 调用 memory.update_decision("dec_20260420_003", status="completed")
  4. 在飞书群聊中通知: "数据库主键变更决策已执行完成"
```

### 场景 D：跨议题冲突检测

```
记忆系统内部:
  1. 新决策 D_new 插入"认证模块"议题
  2. D_new.cross_topic_refs = ["数据库架构"]
  3. 冲突检测: D_new 与"数据库架构"下的 dec_20260420_003 矛盾
  4. contradiction_score = 0.72 (> 0.6)
  5. 创建冲突记录

记忆系统 → OpenClaw:
  通知包 (~400 tokens):
  {
    "signal_type": "conflict_detected",
    "summary": "新认证方案与数据库主键变更存在冲突",
    "decision_a": {"sdr_id": "dec_20260429_001", "title": "新认证方案"},
    "decision_b": {"sdr_id": "dec_20260420_003", "title": "主键类型变更"},
    "contradiction_score": 0.72,
    "description": "新认证方案要求 session_token 长度为 1024，但主键变更决策只分配了 512",
    "action_needed": "user_resolution"
  }

OpenClaw 内部:
  1. 收到冲突通知
  2. 调用 memory.decision("dec_20260429_001") 和 memory.decision("dec_20260420_003")
  3. LLM 分析冲突点
  4. 在飞书中通知用户: "检测到决策冲突:
     - 认证模块: session_token 需要 1024 字符
     - 数据库架构: session_token 分配了 512 字符
     请裁决: 调整数据库 Schema 还是调整认证方案？"
```

---

## 十、上下文预算控制

### 10.1 各层 Token 预算

| 层级 | 内容 | Token 预算 | 何时加载 |
|------|------|-----------|---------|
| 索引层 | 项目状态 + 议题概览 + 最近决策 + 可用工具 | ~800 | 每次交互自动 |
| 工具定义 | 6 个 MCP 工具的 schema | ~600 | OpenClaw 启动时 |
| 搜索结果 | 10 条决策摘要 | ~500 | 每次交互自动 |
| 议题详情 | 某议题的全部 active 决策 | ~1,000 | 按需 |
| 决策详情 | 单个决策的 YAML 字段 | ~300 | 按需 |
| 决策全文 | 单个决策含 Markdown 正文 | ~800 | 按需 |
| 时间线 | 30 天的决策变更 | ~600 | 按需 |
| 冲突详情 | 单个冲突的完整描述 | ~400 | 按需 |
| 信号列表 | 5 条最近信号 | ~300 | 按需 |

### 10.2 典型交互的总预算

```
简单问答（用户问一个事实）:
  索引层 (800) + 搜索 (500) + 1 个决策详情 (300) = ~1,600 tokens

中等复杂（需要了解议题全貌）:
  索引层 (800) + 搜索 (500) + 议题详情 (1000) + 2 个决策详情 (600) = ~2,900 tokens

高复杂（需要处理冲突）:
  索引层 (800) + 搜索 (500) + 冲突详情 (400) + 2 个决策全文 (1600) = ~3,300 tokens

最坏情况（所有工具都调用）:
  索引层 (800) + 搜索 (500) + 议题 (1000) + 3 决策全文 (2400)
  + 时间线 (600) + 冲突 (400) + 信号 (300) = ~6,000 tokens
```

### 10.3 预算超限保护

```
规则:
  1. 单次交互的总上下文预算上限: 8,000 tokens
  2. 当累计接近上限时，记忆系统的 MCP 工具自动截断返回结果
  3. OpenClaw 应优先使用摘要（Layer 2），避免不必要的全文获取（Layer 3）
  4. 如果 OpenClaw 连续调用超过 5 次工具，记忆系统返回提示:
     "建议基于当前上下文直接回答，避免过度查询"
```

---

## 十一、部署与运维

### 11.1 部署架构

```
飞书服务器
├── OpenClaw（已有部署）
│   ├── Agent 进程
│   └── lark-cli 集成
│
└── Memory System（新增部署）
    ├── Docker 容器
    │   ├── 主进程 (SignalActivationEngine)
    │   ├── MCP Server (端口 37777)
    │   └── Git 工作目录 (volume mount)
    │
    └── 共享资源
        ├── Git 远程仓库 (GitLab)
        └── Bitable (飞书 Base)
```

### 11.2 启动顺序

```
1. 启动 Memory System
   ├── 从 Git 加载全量决策到内存
   ├── 建立飞书 WebSocket 订阅
   └── 启动 MCP Server (localhost:37777)

2. 注册 MCP Server 到 OpenClaw
   └── 更新 ~/.openclaw/openclaw.json

3. 重启 OpenClaw（或热加载 MCP 配置）
   └── OpenClaw 连接 Memory MCP Server

4. 验证
   ├── 在飞书中发送 "查看记忆系统状态"
   └── OpenClaw 应能调用 memory.search 并返回结果
```

### 11.3 健康检查

```
Memory System 健康检查:
  ├── HTTP GET http://localhost:37777/health
  ├── 返回: { status: "ok", decisions_loaded: 23, last_sync: "..." }
  └── 异常: status: "degraded" 或 "error"

OpenClaw 侧验证:
  ├── 调用 memory.search(query="test")
  ├── 返回非空结果 → MCP 连接正常
  └── 返回错误 → 检查 Memory System 状态
```

---

## 十二、与现有设计文档的对齐

| 本文档章节 | 对齐的文档 | 对齐点 |
|-----------|-----------|-------|
| §3 渐进式披露 | claude-mem.ai/progressive-disclosure | 三层信息架构：索引→上下文→详情 |
| §3.3 工具层 | claude-mem.ai/search-architecture | MCP 工具设计、3 层工作流 |
| §5.2 通知方式 | claude-mem.ai/hooks-architecture | 非阻塞、fire-and-forget |
| §7 Bitable 协作 | openclaw-architecture.md §6 | Git → Bitable 同步、跨议题查询 |
| §8 Skill 设计 | openclaw-architecture.md §2.2 | lark-cli Adapter 角色 |
| §9 场景 A | signal-activation-engine.md §7 | 信号 → 装配 → 状态机的完整链路 |
| §9 场景 B | decision-extraction.md §7 | 群聊决策提取工作流 |
| §9 场景 C | signal-activation-engine.md §4.2 | 任务完成 → 决策状态推进 |
| §9 场景 D | decision-tree.md §5.3 | 冲突检测与解决 |
| §10 预算控制 | decision-tree.md §3.2 | impact_level 与 token 预算的关系 |

---

## 十三、总结

### 核心设计原则

1. **记忆系统是独立服务，不是 OpenClaw 的 skill**
   - 记忆系统 24h 运行，负责信号检测、上下文装配、持久化
   - 通过 MCP Server 向 OpenClaw 暴露查询接口

2. **渐进式披露，避免上下文膨胀**
   - 索引层（~800 tokens）始终可见
   - 工具层按需获取
   - 详情层验证后深入

3. **OpenClaw 只做判断，不做存储**
   - OpenClaw 的 LLM 负责推理和决策
   - 记忆系统负责数据管理和持久化
   - lark-cli 负责飞书执行

4. **信号驱动，按需通知**
   - 记忆系统自主处理大部分信号
   - 只有需要 LLM 判断的信号才通知 OpenClaw
   - 通知包精简（~300 tokens），不含完整上下文

### 一句话总结

**记忆系统是 OpenClaw 的"长期记忆"——它 24 小时默默运行，只在需要时以最精简的方式提供上下文，让 OpenClaw 的 LLM 专注于推理而非数据管理。**
