# Feishu Memory Agent

飞书群聊/文档决策记忆系统。自动检测 IM 消息和文档变更中的决策信息，提取结构化记录，Git 持久化，飞书卡片推送。

## 数据模型

```go
type DecisionNode struct {
    SDRID       string        // 唯一标识 DEC-20260507-xxxx
    Title       string        // 决策标题
    Decision    string        // 决策内容
    Rationale   string        // 决策依据
    Project     string        // 项目
    Topic       string        // 议题（位置锚点）
    Status      DecisionStatus // pending / pending_confirmation / decided / superseded / rejected / deprecated
    ImpactLevel ImpactLevel   // advisory / minor / major / critical
    Relations   []Relation    // 关系图：DEPENDS_ON | SUPERSEDES | CONFLICTS_WITH
    Proposer    string        // 提出人
    Executor    string        // 执行人
    AccessStats AccessStats   // 访问统计 → 热点值
}
```

## 架构

```
外部源 → 检测器 → 工作池 → 信号引擎 → 状态机 → 管线引擎 → Git + Bitable + 内存图
                                                                        ↓
                                                                   MCP Server → OpenClaw Agent
```

### 核心流程

**1. 检测** — `internal/lark-adapter/lark_im.go` / `lark_doc.go`
- lark-im: 每 5s 轮询群聊消息 → 消息聚合 → 多因子评分（lexical/structural/pattern/anti，阈值 0.5）
- lark-doc: 每 10s 搜索文档变更 → 白名单 MD5 比对 → 评论扫描
- 噪声过滤: 评分 < 0.5 跳过，< 0.2 直接丢弃

**2. 提取** — `internal/signal/engine.go`
- 评分 ≥ 0.5 触发 LLM `ExtractDecisionWithContext`
- LLM 返回: 标题、决策、依据、影响级别、反对意见
- 置信度 ≥ 0.6 接受，[0.6, 0.8) 标记 `pending_confirmation`

**3. 去重** — `internal/signal/engine.go`
```
findSimilarDecision → Token重叠匹配(CJK分词 + 英文完整匹配)
  → evaluateDedupAction → LLM: skip / update / conflict
    → resolveConflict → LLM: merge / keep_both
```
Token 重叠: matchCount ≥ 3 且比例 ≥ 60%，准确率 100%（10 组测试 vs LLM 80%）

**4. 持久化** — `internal/core/pipeline.go` → `internal/storage/git/git_storage.go`
```
applyCreate     → Git WriteDecision + MemoryGraph UpsertDecision
applyUpdate     → Git WriteDecision(覆盖文件, 新commit)
applyStatusChange → Git WriteDecision + MemoryGraph UpsertDecision
applyConflictMerge → Git WriteDecision + Bitable 清理冲突
applyConflictKeepBoth → Git WriteDecision(新SDRID) + Bitable 双向冲突标记
applyDeprecate  → Git WriteDecision + MemoryGraph UpsertDecision
applyRevert     → Git ReadDecisionAtCommit + Git WriteDecision(新commit)
applyCreateObjection → Git WriteObjection + MemoryGraph Upsert
```

**5. 热点值** — `internal/recall/hot_score.go`
```
HotScore = ReferenceCount*20*0.4 + AccessCount*15*0.2 + Relations*25*0.15 + Base(25)
```
每次讨论引用即时重算（`RecordReference` + `RecalculateHotScore`），讨论越多热点值逐步放大。

## Git 存储结构

```
data/decisions/{project}/{topic}/{SDRID}.md
data/objections/{project}/{topic}/{OID}.md
```

文件格式: YAML frontmatter + Markdown 正文。每次写操作 = `git add + git commit`，全部历史可追溯。

## MCP 工具

通过 `cmd/mcp-server/main.go` stdio 模式暴露 30+ 工具给 OpenClaw Agent：

| 类别 | 工具 |
|------|------|
| 查询 | `search` / `decision` / `hot_decisions` / `recent_decisions` / `fulltext_search` / `topic` |
| 创建 | `create_decision` / `extract_and_create` / `confirm_decision` / `reject_decision` |
| 冲突 | `conflict_list` / `resolve_conflict` / `check_conflict` / `evaluate_dedup` |
| Git | `git_history` / `git_search` / `git_blame` / `decision_history` / `revert_decision` |

## 构建

```bash
go build -o bin/mem-service ./cmd/mem-service/main.go    # 服务模式(检测+推送)
go build -o bin/mcp-server ./cmd/mcp-server/main.go       # MCP模式(OpenClaw)
```

## 项目结构

```
cmd/
├── mem-service/        # 主服务(检测器循环 + 工作池 + WebSocket)
├── mcp-server/         # MCP stdio 服务器
├── openclaw-hooks/     # OpenClaw 钩子
└── send-messages/      # 消息发送工具

internal/
├── lark-adapter/       # 飞书检测器(im/doc/wiki/vc/calendar/task/contact)
├── signal/             # 信号引擎(提取 + 去重 + 冲突 + 状态机)
├── core/               # 管线引擎 + 内存图 + 冲突仲裁
├── decision/           # 数据模型
├── mcp/server/         # MCP 工具注册与实现
├── push/               # 飞书卡片推送
├── recall/             # 检索 + 热点值计算
├── card/               # 卡片渲染
├── llm/                # LLM 代理
├── storage/git/        # Git CRUD + 历史追溯
├── storage/bitable/    # Bitable 同步
├── config/             # 配置加载
└── ws/                 # WebSocket 通信
```
