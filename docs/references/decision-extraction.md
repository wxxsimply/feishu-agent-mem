# 飞书决策信息提取方案

基于 lark-cli 从飞书群聊、日程、云文档、知识库、妙记/会议中自动识别和提取决策性信息，建立决策议题-负责人-关联文档-关联群聊-关联日程的完整关联图谱。

---

## 1. 决策性信息来源

### 1.1 群聊沟通（lark-im）

| 信息源    | lark-cli 能力                                            | 决策价值                          |
| ------ | ------------------------------------------------------ | ----------------------------- |
| 群聊消息   | `+chat-messages-list` / `+messages-search` 按时间范围和关键词检索 | 决策讨论过程、结论、关键表态                |
| Pin 消息 | `pins.list` 获取群内所有 Pin 消息                              | 高价值决策锚点，被 Pin 的消息几乎必定是重要公告/决策 |
| 消息回复线程 | `+threads-messages-list` 展开整条线程                        | 决策完整上下文、反对意见、修改过程             |
| 群成员    | `chat.members.get` 获取群成员列表                             | 决策参与者识别                       |

### 1.2 会议与妙记（lark-vc + lark-minutes）

| 信息源 | lark-cli 能力 | 决策价值 |
|--------|--------------|---------|
| 会议记录 | `vc +search` 按时间/关键词/参会人搜索已结束会议 | 会议中的议定事项 |
| AI 智能纪要 | `vc +notes` 获取 `note_doc_token` 对应的 AI 总结 + 待办 + 章节 | **决策结论、行动项、负责人** — 核心决策提取来源 |
| 逐字稿 | `vc +notes` 获取 `verbatim_doc_token` | 决策依据、争议点、逐字表态 |
| 妙记 | `minutes +search` / `minutes minutes.get` | 同上，含未关联会议的录制 |

### 1.3 云文档（lark-doc + lark-drive）

| 信息源 | lark-cli 能力 | 决策价值 |
|--------|--------------|---------|
| 决策文档 | `docs +search` 搜索 + `docs +fetch` 读取内容 | 正式决策记录（PRD、技术方案、评审结论） |
| 文档评论 | `drive file.comments list` 获取评论 | 决策讨论、approval 表达、修改意见 |
| 文档元数据 | `drive metas batch_query` 批量获取 | 文档活跃度、最后更新时间 |
| 访问者记录 | `drive file.view_records list` | 谁在跟踪这份文档 |

### 1.4 知识库（lark-wiki）

| 信息源 | lark-cli 能力 | 决策价值 |
|--------|--------------|---------|
| 知识空间 | `wiki spaces list` / `spaces.get` | 项目知识域划分 |
| 知识节点 | `wiki nodes list` 获取子节点层级 | 已沉淀的决策记录结构化组织 |
| 节点解析 | `wiki spaces get_node` 解析 wiki 链接 | 识别背后文档的真实类型（docx/sheet/bitable） |

### 1.5 日程（lark-calendar）

| 信息源 | lark-cli 能力 | 决策价值 |
|--------|--------------|---------|
| 日程视图 | `calendar +agenda` 查看日程安排 | 决策时间线、决策会议 |
| 日程搜索 | `calendar events search` 按关键词/时间 | 识别决策评审会 |
| 参会人 | `calendar event.attendees list` | 决策参与者 |
| 忙闲状态 | `calendar +freebusy` | 决策时机和可用性 |

### 1.6 任务（lark-task）

| 信息源 | lark-cli 能力 | 决策价值 |
|--------|--------------|---------|
| 我的任务 | `task +get-my-tasks` 分配给我的 | 决策后的行动项、任务状态反映决策执行进度 |
| 相关任务 | `task +get-related-tasks` 我创建/关注的 | 覆盖更广的任务视图 |
| 任务搜索 | `task +search` 按关键词 | 关联特定决策议题的任务 |
| 任务清单 | `task tasklists.list` / `tasklists.tasks` | 项目阶段的行动划分 |

### 1.7 OKR（lark-okr）

| 信息源 | lark-cli 能力 | 决策价值 |
|--------|--------------|---------|
| OKR 周期 | `okr +cycle-list` 获取用户周期 | 决策的战略时序背景 |
| 目标与关键结果 | `okr +cycle-detail` 获取周期内 OKR 详情 | 决策是否对齐上级目标、关键结果的进度反映决策效果 |

### 1.8 通讯录（lark-contact）

| 信息源 | lark-cli 能力 | 决策价值 |
|--------|--------------|---------|
| 用户搜索 | `contact +search-user` 按姓名/邮箱/手机号 | 将 open_id 解析为人名，建立负责人信息 |
| 用户详情 | `contact +get-user` 获取完整信息 | 部门、职位等组织信息 |

---

## 2. 决策关联关系模型

每个决策议题关联以下实体：

```
Decision（决策议题）
├── Responsible（负责人）
│   ├── 决策提出人（消息 sender）
│   ├── 决策审批人（文档评论 /approve 者）
│   ├── 决策执行人（任务 assignee）
│   └── 参会人（日程 attendee / 会议 participant）
├── RelatedDocs（关联文档）
│   ├── PRD / 技术方案（docs +search / +fetch）
│   ├── 会议纪要（vc +notes → note_doc_token）
│   └── 评论互动（drive file.comments list）
├── RelatedChats（关联群聊）
│   ├── 讨论群（im +chat-search / chat.members.get）
│   ├── 关键消息（im +messages-mget / +messages-search）
│   └── Pin 消息锚点（im pins.list）
├── RelatedCalendar（关联日程）
│   ├── 决策会议（calendar +agenda / events.search）
│   └── 截止日期里程碑
├── RelatedTasks（关联任务）
│   ├── 决策待办（task +get-related-tasks）
│   └── 行动项进度（task tasks.get）
├── RelatedMeeting（关联会议）
│   ├── 会议纪要 AI 总结（vc +notes）
│   ├── 逐字稿（verbatim_doc_token）
│   └── 妙记（minutes minutes.get）
├── RelatedOKR（关联 OKR）
│   ├── 对齐的 Objective（okr +cycle-detail）
│   └── 对齐的 Key Result 进度
├── Progress（项目进度）— 综合分析
│   ├── 当前阶段判断
│   ├── 任务完成率
│   └── 阻塞项/风险
└── Stage（项目阶段）— 状态机
    ├── 待决策 / 决策中 / 已决策
    ├── 执行中 / 已完成 / 已关闭
    └── 否决 / 搁置
```

### 2.1 决策状态流转

```
待决策 ──→ 决策中（讨论/评审中）──→ 已决策
                                    ├──→ 执行中 ──→ 已完成
                                    ├──→ 搁置
                                    └──→ 否决
```

---

## 3. 提取信息类型

### 3.1 决策议题识别信号

| 信号类型 | 检测方式 | 示例关键词/模式 |
|---------|---------|----------------|
| 群聊消息关键词 | `im +messages-search --query` | `决定`、`确认`、`结论`、`通过`、`定下来`、`就这么办` |
| 英文决策信号 | `im +messages-search --query` | `approve`、`LGTM`、`decided`、`confirmed`、`agreed` |
| Pin 消息 | `im pins.list` | 被 Pin 的消息自动标记为高价值信息 |
| 文档评论 | `drive file.comments list` 配合 `replys.list` | `/approve`、`同意`、`不通过`、`需要修改` |
| 会议纪要待办 | `vc +notes` 读取 AI 待办章节 | 所有出现在 "待办" / "Action Items" 中的内容 |
| AI 总结 | `vc +notes` 读取 AI 总结章节 | 包含"决定"、"确认"、"结论"等关键词的段落 |

### 3.2 项目进度提取

| 维度 | 提取方式 | 命令 |
|------|---------|------|
| 任务完成率 | 统计 `+get-my-tasks` 中的 `status=completed` 占比 | `task +get-my-tasks --page-all` |
| OKR 进度 | 读取 Key Result 的进度指标 | `okr +cycle-detail --cycle-id <id>` |
| 文档活跃度 | 查看最近更新/访问时间 | `drive file.statistics.get` |
| 决策里程碑 | 查看日程中的里程碑事件 | `calendar events.search --params '{"query": "里程碑"}'` |

### 3.3 项目阶段识别

| 维度 | 提取方式 | 命令 |
|------|---------|------|
| OKR 周期 | 周期天然划分阶段 | `okr +cycle-list` |
| 日程里程碑 | 日程中的关键节点事件 | `calendar +agenda` 跨时间段查询 |
| 任务清单分组 | 清单 section 划分 | `task tasklists.tasks` / `sections.list` |
| 知识库结构 | 节点层级反映阶段划分 | `wiki nodes.list` |

### 3.4 关联负责人提取

| 角色 | 来源 | 解析方式 |
|------|------|---------|
| 决策提出人 | 消息的 sender | `im +messages-mget` → sender open_id → `contact +get-user` |
| 决策审批人 | 文档评论者 | `drive file.comments list` → commenter → `contact +search-user` |
| 执行人 | 任务 assignee | `task +get-my-tasks` → assignee → `contact +get-user` |
| 参会人 | 日程/会议参与者 | `calendar event.attendees list` / `vc meeting.get --with_participants` |

---

## 4. lark-cli 提取命令大全

### 4.1 群聊信息

```bash
# 搜索群聊
lark-cli im +chat-search --query "<项目名/关键词>"

# 按时间范围列出群消息（核心提取命令）
lark-cli im +chat-messages-list \
  --chat-id oc_xxx \
  --start-time "$(date -d '2026-04-01' +%s)" \
  --end-time "$(date +%s)" \
  --page-all

# 关键词搜索消息（跨群搜索，user-only）
lark-cli im +messages-search \
  --query "<决策关键词>" \
  --chat-ids oc_xxx,oc_yyy \
  --sender open_id \
  --start-time "$(date -d '2026-04-01' +%s)" \
  --end-time "$(date +%s)" \
  --page-all

# 获取 Pin 消息
lark-cli im pins list --params '{"chat_id": "oc_xxx"}'

# 展开消息线程
lark-cli im +threads-messages-list \
  --thread-id omt_xxx \
  --page-all

# 批量获取消息详情（解析 sender name）
lark-cli im +messages-mget \
  --message-ids om_xxx,om_yyy

# 获取群成员列表
lark-cli im chat.members get \
  --params '{"chat_id": "oc_xxx", "page_size": 100}'

# 获取群信息
lark-cli im chats get \
  --params '{"chat_id": "oc_xxx"}'
```

### 4.2 日程信息

```bash
# 查看今日/指定日期日程
lark-cli calendar +agenda --date "2026-04-28"

# 按关键词和时间搜索日程
lark-cli calendar events search \
  --params '{"query": "<关键词>", "start_time": "1714320000", "end_time": "1716825599"}'

# 查询日程视图（含重复日程展开）
lark-cli calendar events instance_view \
  --params '{"calendar_id": "primary", "start_time": "1714320000", "end_time": "1716825599"}'

# 获取日程详情
lark-cli calendar events get \
  --params '{"event_id": "event_xxx"}'

# 获取日程参与人
lark-cli calendar event.attendees list \
  --params '{"event_id": "event_xxx", "page_size": 100}'

# 查询用户忙闲
lark-cli calendar +freebusy \
  --user-ids "open_id1,open_id2" \
  --start-time "1714320000" \
  --end-time "1716825599"
```

### 4.3 云文档信息

```bash
# 搜索云空间文档（资源发现入口）
lark-cli docs +search --query "<关键词>" --page-all

# 读取文档内容
lark-cli docs +fetch \
  --api-version v2 \
  --doc "<doc_token/URL>" \
  --doc-format xml

# 批量获取文档元数据（标题、URL）
lark-cli schema drive.metas.batch_query
lark-cli drive metas batch_query \
  --data '{"request_docs": [{"doc_type": "docx", "doc_token": "<token>"}], "with_url": true}'

# 获取未解决评论（可能包含决策讨论）
lark-cli drive file.comments list \
  --params '{"file_token": "<doc_token>", "file_type": "docx", "is_solved": false}'

# 获取评论中的回复
lark-cli drive file.comment.replys list \
  --params '{"file_token": "<doc_token>", "comment_id": "<comment_id>"}'

# 获取文件统计信息
lark-cli drive file.statistics get \
  --params '{"file_token": "<doc_token>"}'

# 获取访问者记录
lark-cli drive file.view_records list \
  --params '{"file_token": "<doc_token>"}'

# 获取文件夹下的文件清单
lark-cli drive files list \
  --params '{"folder_token": "fldcnxxx", "page_size": 50}'
```

### 4.4 知识库信息

```bash
# 获取知识空间列表
lark-cli wiki spaces list

# 获取知识空间信息
lark-cli wiki spaces get --params '{"space_id": "xxx"}'

# 解析 wiki 链接（获取真实文档 token 和类型）
lark-cli wiki spaces get_node --params '{"token": "<wiki_token>"}'

# 获取子节点列表
lark-cli wiki nodes list \
  --params '{"space_id": "xxx", "page_size": 50}'

# 获取空间成员
lark-cli wiki members list \
  --params '{"space_id": "xxx", "page_size": 100}'
```

### 4.5 会议与妙记信息

```bash
# 搜索已结束会议（核心提取命令）
lark-cli vc +search \
  --start-time "$(date -d '2026-04-01' +%s)" \
  --end-time "$(date +%s)" \
  --query "<关键词>" \
  --page-all

# 获取会议详情（含参会人）
lark-cli vc meeting get \
  --params '{"meeting_id": "<meeting_id>", "with_participants": true}'

# 获取会议纪要产物（总结、待办、章节、逐字稿）
lark-cli vc +notes \
  --meeting-ids "<meeting_id>"

# 通过妙记获取纪要
lark-cli vc +notes \
  --minute-tokens "<minute_token>"

# 获取妙记基础信息
lark-cli minutes minutes get \
  --params '{"minute_token": "<minute_token>"}'

# 搜索妙记
lark-cli minutes +search \
  --query "<关键词>" \
  --owner me \
  --start-time "1714320000" \
  --end-time "1716825599"

# 读取纪要文档（AI 总结内容）
lark-cli docs +fetch \
  --api-version v2 \
  --doc "<note_doc_token>" \
  --doc-format markdown
```

### 4.6 任务信息

```bash
# 获取分配给我的任务
lark-cli task +get-my-tasks --page-size 50

# 获取与我相关的任务
lark-cli task +get-related-tasks --page-size 50

# 按关键词搜索任务
lark-cli task +search --query "<关键词>" --page-size 50

# 获取任务详情
lark-cli task tasks get --params '{"guid": "<task_guid>"}'

# 获取任务清单
lark-cli task tasklists list --as user

# 获取清单中的任务
lark-cli task tasklists tasks \
  --params '{"tasklist_guid": "<guid>", "page_size": 50}'
```

### 4.7 OKR 信息

```bash
# 获取 OKR 周期列表
lark-cli okr +cycle-list --user-id "<open_id>"

# 获取周期内 OKR 详情（含目标和关键结果）
lark-cli okr +cycle-detail --cycle-id "<cycle_id>"
```

### 4.8 联系人信息

```bash
# 按姓名搜索用户
lark-cli contact +search-user --query "<姓名/邮箱>"

# 获取用户详细信息
lark-cli contact +get-user --user-id "<open_id>"
```

---

## 5. 事件监听：触发决策提取的时机

使用 `lark event +subscribe` 通过 WebSocket 长连接实时监听飞书事件。

### 5.1 群聊消息事件

```bash
# 监听群聊消息 + Pin 变化
lark-cli event +subscribe \
  --events im.message.receive_v1,im.message.pin,im.message.unpin \
  --filter '^oc_project_group' \
  --compact \
  --output ./events/im.log
```

| 事件 | 触发条件 | 提取动作 |
|------|---------|---------|
| `im.message.receive_v1` | 群聊收到新消息 | 检测决策关键词，匹配则提取完整消息上下文 |
| `im.message.pin` | 消息被 Pin | **强信号**：立即提取被 Pin 消息为潜在决策锚点 |
| `im.message.unpin` | 消息取消 Pin | 标记关联决策状态变化 |

### 5.2 文档变更事件

```bash
# 监听文档创建、更新和评论
lark-cli event +subscribe \
  --events drive.file.created_v1,drive.file.updated_v1,drive.file.comment_add_v1 \
  --compact \
  --output ./events/docs.log
```

| 事件 | 触发条件 | 提取动作 |
|------|---------|---------|
| `drive.file.created_v1` | 新文档创建 | 检测标题是否含项目名，提取文档信息 |
| `drive.file.updated_v1` | 文档内容更新 | 标记需重新提取决策内容 |
| `drive.file.comment_add_v1` | 文档新增评论 | 检测评论中的 approval/反对意见 |

### 5.3 日历事件

```bash
# 监听日程变化
lark-cli event +subscribe \
  --events calendar.event.created,calendar.event.updated,calendar.event.deleted \
  --compact \
  --output ./events/calendar.log
```

| 事件 | 触发条件 | 提取动作 |
|------|---------|---------|
| `calendar.event.created` | 创建新日程 | 检测是否为决策评审会议 |
| `calendar.event.updated` | 日程时间变更 | 时间调整可能影响决策 timeline |
| `calendar.event.deleted` | 删除日程 | 标记关联决策状态变更 |

### 5.4 任务事件

```bash
# 监听任务更新
lark-cli event +subscribe \
  --events task.task.updated \
  --compact \
  --output ./events/task.log
```

| 事件 | 触发条件 | 提取动作 |
|------|---------|---------|
| `task.task.updated` | 任务状态变更 | 更新决策执行进度，完成时标记决策阶段推进 |

### 5.5 会议结束事件

```bash
# 监听会议结束
lark-cli event +subscribe \
  --events vc.meeting.meeting_ended \
  --compact \
  --output ./events/vc.log
```

| 事件 | 触发条件 | 提取动作 |
|------|---------|---------|
| `vc.meeting.meeting_ended` | 会议结束 | **强信号**：立即拉取会议纪要、AI 总结、待办，提取决策结论 |

### 5.6 注意事项

- **`im.message.receive_v1` 在 bot 身份下仅能收到 bot 所在群的消息**，需要确保 bot 已加入目标群
- 用户加群/退群事件也会触发非核心事件，可通过 `--filter` 正则路由过滤
- 每个 `event +subscribe` 命令建立一条独立的 WebSocket 连接，建议为关键事件订阅单独运行

---

## 6. 轮询策略（定时补充）

事件驱动可能遗漏部分数据，设计定期轮询作为补充。

### 6.1 每日定时任务

```bash
# 09:00 — 获取当日日程，预识别决策会议
lark-cli calendar +agenda --date "$(date +%Y-%m-%d)"

# 18:00 — 获取今日群聊消息，提取决策议题
START_TS=$(date -j -f "%Y-%m-%d %H:%M:%S" "$(date +%Y-%m-%d) 00:00:00" +%s)
END_TS=$(date +%s)
lark-cli im +chat-messages-list --chat-id oc_xxx \
  --start-time "$START_TS" --end-time "$END_TS" --page-all

# 18:30 — 获取今日结束的会议
lark-cli vc +search \
  --start-time "$START_TS" --end-time "$END_TS" --page-all
```

### 6.2 每周回顾

```bash
# 周一 09:00 — 回顾上周新增的文档
lark-cli docs +search --query "<项目关键词>" --page-all

# 周一 09:30 — 回顾上周结束的会议
WEEK_AGO=$(date -j -v-7d +%s)
lark-cli vc +search --start-time "$WEEK_AGO" --end-time "$(date +%s)" --page-all

# 周一 10:00 — 回顾上周更新的任务
lark-cli task +get-related-tasks --page-size 100
```

---

## 7. 完整提取工作流

### 场景 A：群聊产生决策

```
① 事件触发: im.message.receive_v1 包含 "决定了，就用方案B"
    │
② 提取消息详情
    ├── im +messages-mget --message-ids om_xxx
    └── 解析 sender open_id
    │
③ 获取讨论上下文
    ├── im +threads-messages-list --thread-id omt_xxx
    └── 获取完整讨论链
    │
④ 识别关联文档（消息中可能含文档链接）
    ├── docs +search --query "方案B"
    └── docs +fetch --api-version v2 --doc "<doc_token>"
    │
⑤ 识别关联人员
    ├── contact +get-user --user-id "<open_id>"
    └── 获取姓名、部门
    │
⑥ 查询关联日程/任务
    ├── calendar events search --params '{"query": "方案B评审"}'
    └── task +search --query "方案B"
    │
⑦ 写入决策记录
    └── 整理为结构化决策记录
```

### 场景 B：会议结束触发提取

```
① 事件触发: vc.meeting.meeting_ended
    │
② 获取会议详情
    ├── vc meeting get --params '{"meeting_id":"xxx","with_participants":true}'
    └── 获取主题、时间、参会人
    │
③ 获取纪要产物
    ├── vc +notes --meeting-ids "<meeting_id>"
    │   ├── 读取 AI 总结（note_doc_token）
    │   ├── 读取待办列表
    │   └── 关联逐字稿（verbatim_doc_token）
    │
④ 解析决策结论
    ├── docs +fetch --api-version v2 --doc "<note_doc_token>" --doc-format markdown
    └── 从 AI 总结中提取: 决定事项、行动项、负责人
    │
⑤ 创建待办任务（从纪要中的待办提取）
    └── task +create --summary "<待办>" --assignee-id "<open_id>" --due-at "<deadline>"
    │
⑥ 关联已有决策
    ├── docs +search --query "<会议主题关键部分>"
    └── 查找是否已有相关决策记录
    │
⑦ 更新决策记录
    └── 写入/更新结构化决策记录
```

---

## 8. 决策记录存储结构

每条决策议题存储为独立记录，可使用 JSON 或 Markdown 格式。

### 8.1 决策记录 JSON 结构

```json
{
  "decision_id": "dec_20260428_001",
  "topic": "Q2 技术选型：确定采用方案B",
  "status": "已决策",
  "stage": "执行中",
  "confidence": "high",
  "detected_at": "2026-04-28T10:00:00+08:00",
  "decided_at": "2026-04-28T14:30:00+08:00",
  "sources": [
    {"type": "im_message", "id": "om_xxx", "chat_id": "oc_xxx"},
    {"type": "meeting", "id": "meeting_xxx"}
  ],
  "responsible": [
    {"open_id": "ou_xxx", "name": "张三", "role": "决策人"},
    {"open_id": "ou_yyy", "name": "李四", "role": "执行人"}
  ],
  "related_docs": [
    {"token": "doxcnxxx", "title": "Q2 技术选型方案", "type": "docx"}
  ],
  "related_chats": [
    {"chat_id": "oc_xxx", "name": "核心架构群", "message_ids": ["om_111", "om_222"]}
  ],
  "related_calendar": [
    {"event_id": "event_xxx", "title": "技术选型评审会", "start_time": "2026-04-28T14:00:00+08:00"}
  ],
  "related_meetings": [
    {
      "meeting_id": "meeting_xxx",
      "summary": "评审结论：采用方案B",
      "note_doc_token": "doxcn_yyy",
      "attendees": ["ou_xxx", "ou_zzz"]
    }
  ],
  "related_tasks": [
    {"guid": "task_xxx", "title": "方案B原型开发", "status": "completed", "assignee": "ou_yyy"}
  ],
  "progress": "原型已完成，进入集成测试阶段",
  "next_review": "2026-05-12",
  "tags": ["技术选型", "Q2", "架构"]
}
```

### 8.2 项目阶段记录

```json
{
  "project_id": "proj_Q2_core_platform",
  "name": "Q2 核心平台升级",
  "current_stage": "执行阶段",
  "stage_history": [
    {"name": "需求评审", "status": "done", "completed_at": "2026-04-01"},
    {"name": "技术选型", "status": "done", "completed_at": "2026-04-28"},
    {"name": "开发", "status": "in_progress", "target": "2026-05-30", "progress": "35%"},
    {"name": "测试", "status": "todo", "target": "2026-06-15"},
    {"name": "上线", "status": "todo", "target": "2026-06-30"}
  ],
  "decisions": ["dec_20260428_001", "dec_20260415_002"],
  "okr_alignment": {
    "cycle_id": "cycle_xxx",
    "objectives": ["提升系统稳定性至 99.9%"]
  }
}
```

---

## 9. 方案核心原则

1. **事件驱动 + 定时轮询结合**：实时监听（WebSocket）感知变化即时触发，定时轮询补充事件遗漏
2. **增量提取**：按时间戳/偏移量记录上次提取位置，每次只处理新数据
3. **关联推理**：根据消息中的链接（文档 URL/日程链接）、@提及的人员、共享关键词进行关联
4. **去重合并**：同一决策从多个来源（群聊 + 会议 + 文档）检测到时，按决策 ID 合并
5. **状态机驱动**：决策状态（待决策 → 决策中 → 已决策 → 执行中 → 已完成）决定后续提取策略的优先级
6. **身份选择**：群聊消息搜索（`+messages-search`）仅支持 `--as user`；群消息列表（`+chat-messages-list`）支持 `user` 和 `bot`。读取个人日程/任务/OKR 优先使用 `--as user`，读取公开会议纪要可用 `--as bot`
