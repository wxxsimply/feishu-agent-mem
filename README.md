# Feishu Memory Agent

飞书群聊/文档决策记忆系统。自动检测 IM 消息和文档变更中的决策信息，提取结构化记录，Git 分支持久化，飞书卡片推送，OpenClaw MCP 查询。

## 快速开始

### 1. 环境配置

复制 `.env.example` 为 `.env`，填入飞书应用凭证：

```bash
# 飞书应用凭证（必填）
LARK_APP_ID=cli_xxxxxxxxxxxxxxxx
LARK_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxx

# LLM API（必填，二选一）
DASHSCOPE_API_KEY=sk-xxxxx          # 通义千问
DASHSCOPE_BASE_URL=https://dashscope.aliyuncs.com/compatible-mode/v1
# 或
DEEPSEEK_API_KEY=sk-xxxxx           # DeepSeek
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-chat

# 飞书群聊（检测与推送）
LARK_CHAT_IDS=oc_xxxxx              # 推送目标群
LARK_DETECT_CHAT_IDS=oc_xxxxx       # 检测源群（不填则用 LARK_CHAT_IDS）

# MCP Server 注册路径（供 OpenClaw 发现）
MCP_REGISTER_PATH=~/.openclaw/openclaw.json
```

### 2. 启动 mem-service（全功能模式）

```bash
# 编译
go build -o bin/mem-service ./cmd/mem-service/main.go

# 启动（后台运行）
./bin/mem-service &

# 查看日志
tail -f logs/app.log
```

mem-service 启动后会：
- 以 5-10s 间隔轮询飞书群聊消息
- 自动检测并提取决策 → Git 持久化 → 飞书卡片推送
- 同时启动 MCP Server（端口 37777）供 OpenClaw 连接
- 同时启动 WebSocket Server（端口 8765）供独立检测器连接

### 3. 启动 mcp-server（仅查询模式）

```bash
# 编译
go build -o bin/mcp-server ./cmd/mcp-server/main.go

# 启动（stdio 模式，供 OpenClaw 或 MCP 客户端使用）
./bin/mcp-server
```

mcp-server 启动后通过 stdin/stdout 暴露 30+ MCP 工具。OpenClaw 配置示例：

```json
{
  "mcpServers": {
    "feishu-mem": {
      "command": "/root/openclaw-workspace/feishu-agent-mem/bin/mcp-server"
    }
  }
}
```

### 4. Docker 内快速重启

```bash
# 一键编译 + 重启 mem-service
docker exec openclaw-zh bash -c 'cd /root/openclaw-workspace/feishu-agent-mem && \
  pkill -f mem-service 2>/dev/null; sleep 1; \
  go build -o bin/mem-service ./cmd/mem-service/main.go && \
  ./bin/mem-service & echo "✅ 已启动"'
```

## 环境变量参考

| 变量 | 必填 | 说明 |
|------|------|------|
| `LARK_APP_ID` | ✅ | 飞书应用 ID |
| `LARK_APP_SECRET` | ✅ | 飞书应用 Secret |
| `DASHSCOPE_API_KEY` | ✅* | 通义千问 API Key（与 DeepSeek 二选一） |
| `DEEPSEEK_API_KEY` | ✅* | DeepSeek API Key |
| `DEEPSEEK_MODEL` | | 模型名，默认 `deepseek-chat` |
| `LARK_CHAT_IDS` | ✅ | 推送目标群聊 ID（逗号分隔） |
| `LARK_DETECT_CHAT_IDS` | | 检测源群聊 ID（不填则用 LARK_CHAT_IDS） |
| `CONFIG_PATH` | | 配置文件路径，默认 `config/openclaw.yaml` |

## 数据模型

```go
type DecisionNode struct {
    SDRID       string        // 唯一标识 DEC-20260507-xxxx
    Title       string        // 决策标题
    Decision    string        // 决策内容
    Rationale   string        // 决策依据
    Project     string        // 项目
    Topic       string        // 议题（位置锚点）
    Branch      string        // Git 分支 decision/DEC-xxx
    Version     int           // 版本号（分支 commits 数）
    Status      DecisionStatus // pending | pending_confirmation | decided | superseded | rejected | deprecated
    ConflictStatus string     // "" | "active" | "resolved"
    ConflictWith   string     // 冲突对端 SDRID
    ImpactLevel ImpactLevel   // advisory | minor | major | critical
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
Token 重叠: matchCount ≥ 3 且比例 ≥ 60%

**4. 持久化** — `internal/core/pipeline.go` → `internal/storage/git/git_storage.go`
每个决策写入独立 Git 分支 `decision/{sdr_id}`，版本号由分支 commits 数推导。

**5. 热点值** — `internal/recall/hot_score.go`
```
HotScore = ReferenceCount*20*0.4 + AccessCount*15*0.2 + Relations*25*0.15 + Base(25)
```

## Git 决策树模型

每决策 = 每分支。所有决策以独立 Git 分支存储，通过分支操作管理生命周期。

```
data/
├── decisions/{project}/{topic}/{SDRID}.md   ← 各在分支 decision/DEC-xxx 上
├── objections/{project}/{topic}/{OID}.md
├── L0_RULES.md                              ← 首个提交
└── DEC-000.md                               ← Dummy 根决策（main 分支，v0）
```

查看决策树拓扑：
```bash
cd data
git log --graph --oneline --all --decorate
```

### 6 种 Git 操作

| 操作 | 说明 |
|------|------|
| **创建** | `git checkout -b decision/DEC-001 main` → commit, version=1 |
| **更新** | checkout 自身分支 → 修改 → commit, version++ |
| **冲突** | 双方分支各自标记 `conflict_status=active` + CONFLICTS_WITH 关系 |
| **解决** | 胜者 `resolved`，败者 `superseded` + SUPERSEDES 关系 |
| **回退** | 切回历史分支 → `version = 当前版本 + 1` → commit |
| **废弃** | 自身分支 `status=deprecated` → commit |

## MCP 工具

通过 `cmd/mcp-server/main.go` stdio 模式暴露 30+ 工具给 OpenClaw Agent：

| 类别 | 工具 |
|------|------|
| 查询 | `search` / `decision` / `hot_decisions` / `recent_decisions` / `fulltext_search` / `topic` |
| 创建 | `create_decision` / `extract_and_create` / `confirm_decision` / `reject_decision` |
| 冲突 | `conflict_list` / `resolve_conflict` / `check_conflict` / `evaluate_dedup` |
| Git | `git_history` / `git_search` / `git_blame` / `decision_history` / `revert_decision` |
| 卡片 | `decision_card` / `refresh` / `stats` / `llm_stats` |

## 测试

```bash
# 核心模型测试
go test ./internal/decision/ -v

# 管线引擎测试
go test ./internal/core/ -v

# 信号引擎测试
go test ./internal/signal/ -v

# Git 决策树全量测试（18 个用例）
go test ./test/git-tree/ -v

# MCP 集成测试（启动 MCP server + create + resolve + 验证 DAG）
go test ./test/git-tree/ -v -run "TestMCP"

# 卡片渲染测试（状态 emoji/颜色指示）
go test ./test/git-tree/ -v -run "TestCard"
```

## 项目结构

```
cmd/
├── mem-service/        # 主服务(检测器循环 + 工作池 + WebSocket)
├── mcp-server/         # MCP stdio 服务器


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
├── storage/git/        # Git CRUD + 分支管理 + 历史追溯
├── storage/bitable/    # Bitable 同步
├── config/             # 配置加载
└── ws/                 # WebSocket 通信

docs/
├── git-tree.md         # Git 决策树设计文档

```
