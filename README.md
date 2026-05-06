# Feishu Agent Memory (飞书智能记忆系统)

基于飞书平台的智能决策记忆提取与管理系统，通过监听飞书各功能模块的变更，使用 LLM 智能提取决策信息并持久化存储。

## 功能特性

- 🤖 **多源信号监听**: 实时监听飞书 IM 消息、文档、日历、会议、任务、Wiki、通讯录等变更
- 🧠 **智能决策提取**: 通过 LLM 自动从信号中提取 Decision（决策）
- 💾 **双重存储**: 同时存储到 Git 仓库（用于版本追踪）和飞书多维表格（用于可视化）
- 🔌 **MCP 服务**: 提供 MCP (Model Context Protocol) 接口，供 AI Agent 查询记忆
- 🚀 **进程分离架构**: 主服务与检测器进程分离，支持独立部署和扩展
- ⚡ **长连接模式**: 支持 WebSocket 长连接实时接收飞书事件

## 目录结构

```
feishu-agent-mem/
├── cmd/                          # 可执行程序入口
│   ├── mem-service/             # 主服务进程（核心）
│   ├── mcp-server/              # MCP 服务器
│   ├── detector-lark-im/        # IM 消息检测器
│   ├── detector-lark-doc/       # 文档检测器
│   ├── detector-lark-wiki/      # Wiki 检测器
│   ├── detector-lark-calendar/  # 日历检测器
│   ├── detector-lark-task/      # 任务检测器
│   ├── detector-lark-vc/        # 会议检测器
│   └── detector-lark-contact/   # 联系人检测器
│
├── internal/                     # 核心业务逻辑
│   ├── core/                    # 内存图、变更管道
│   ├── decision/                # 决策数据模型
│   ├── signal/                  # 信号激活引擎、状态机
│   ├── lark-adapter/            # 飞书 API 适配器
│   ├── llm/                     # LLM 集成模块
│   ├── storage/                 # Git / Bitable 存储
│   ├── ws/                      # WebSocket 通信
│   ├── detector/                # 检测器基类
│   ├── mcp/                     # MCP 协议实现
│   ├── config/                  # 配置管理
│   ├── push/                    # 推送模块
│   ├── recall/                  # 召回模块
│   └── sync/                    # 同步模块
│
├── config/                      # 配置文件
│   └── openclaw.yaml           # 主配置文件
│
├── data/                        # 数据目录
│   ├── decisions/              # 决策文件存储（Git）
│   └── git/                    # Git 仓库
│
├── outputs/                     # 输出目录
├── logs/                        # 日志目录
└── scripts/                     # 辅助脚本
```

## 核心架构

### 核心数据模型

**DecisionNode** (`internal/decision/node.go`):

```go
type DecisionNode struct {
    SDRID          string              // 决策唯一标识
    GitCommitHash  string              // Git 提交哈希
    Title          string              // 决策标题
    Decision       string              // 决策内容
    Rationale      string              // 决策理由
    Project        string              // 所属项目
    Topic          string              // 所属议题
    Phase          string              // 所属阶段
    ImpactLevel    ImpactLevel         // 影响级别 (advisory/minor/major/critical)
    Status         DecisionStatus      // 状态 (pending/in_discussion/decided/executing/completed/...)
    ParentDecision string              // 父决策 ID
    ChildrenCount  int                 // 子决策数量
    Relations      []Relation          // 关联关系
    Proposer       string              // 提议人
    Executor       string              // 执行人
    Stakeholders   []string            // 利益相关者
    FeishuLinks    FeishuLinks         // 飞书关联（消息/文档/日程/会议/任务/妙记）
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

### 系统架构图

```
┌─────────────────────────────────────────────────────────────┐
│                        飞书平台 (Feishu)                       │
│  ┌──────┐ ┌─────┐ ┌──────┐ ┌────────┐ ┌──────┐ ┌──────┐  │
│  │  IM  │ │Doc  │ │ Wiki │ │Calendar│ │ Task │ │  VC  │  │
│  └──┬───┘ └──┬──┘ └──┬───┘ └───┬────┘ └──┬───┘ └──┬───┘  │
└─────┼─────────┼────────┼──────────┼─────────┼─────────┼──────┘
      │         │        │          │         │         │
      │ Webhook / 轮询 / WebSocket 长连接
      │         │        │          │         │         │
      ▼         ▼        ▼          ▼         ▼         ▼
┌─────────────────────────────────────────────────────────────┐
│                    检测器进程 (detector-*)                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │
│  │ lark-im  │ │ lark-doc │ │ lark-vc  │ │   ...    │     │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘     │
└───────┼────────────┼────────────┼────────────┼───────────┘
        │            │            │            │
        │      WebSocket (检测变更)
        │            │            │            │
        ▼            ▼            ▼            ▼
┌─────────────────────────────────────────────────────────────┐
│                   mem-service (主服务)                        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  signal/engine.go: 信号激活引擎                       │  │
│  │  ├── 多因子加权检测 (词义/结构/动态/模式)             │  │
│  │  ├── LLM 决策提取                                    │  │
│  │  └── 状态机流转 (pending → in_discussion → ...)      │  │
│  ├──────────────────────────────────────────────────────┤  │
│  │  core/pipeline.go: 变更处理管道                       │  │
│  │  ├── Create / Update / StatusChange / Conflict       │  │
│  │  ├── Git 存储                                        │  │
│  │  ├── Bitable 同步                                    │  │
│  │  └── 内存图更新                                      │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
              │
              ▼
    ┌─────────────────┐
    │ MCP Server      │ ← AI Agent 查询
    └─────────────────┘
```

## 核心流程

### 检测周期

1. **检测器进程** 通过轮询或 WebSocket 监听飞书平台变更
2. 检测到变更后，通过 WebSocket 发送到 **mem-service 主服务**
3. **信号激活引擎** (signal/engine.go) 处理信号：
   - 多因子加权评分检测
   - LLM 提取决策
   - 状态机管理决策状态
4. **变更管道** (core/pipeline.go) 应用变更：
   - 写入 Git 存储
   - 同步 Bitable
   - 更新内存图索引

### 决策状态机

```
pending → in_discussion → decided → executing → completed
                                     ↘→ rejected
                                     ↘→ shelved
                                     ↘→ superseded
                                     ↘→ deprecated
```

## 配置说明

### 主配置 (config/openclaw.yaml)

```yaml
service:
  name: "feishu-memory-service"
  version: "2.0.0"
  ws_port: 8765
  data_dir: "./data"
  output_dir: "./outputs"
  log_dir: "./logs"

detectors:
  lark_im:
    enabled: true
    interval: 5s
    burst_interval: 5s
    burst_timeout: 1m
  lark_doc:
    enabled: true
    interval: 10s
  lark_wiki:
    enabled: false
  lark_calendar:
    enabled: false
  lark_task:
    enabled: false
  lark_vc:
    enabled: false
  lark_contact:
    enabled: false

storage:
  git_path: "./data/git"
  bitable_enabled: false

llm:
  enabled: true
  extract_prompt: "extraction"
  doc_extract_prompt: "extraction_doc"
```

### 环境变量 (.env)

```bash
FEISHU_APP_ID=cli_xxxxxxxxxx
FEISHU_APP_SECRET=xxxxxxxxxxxxx
LARK_CHAT_IDS=oc_xxxxxxxxxx
LARK_DETECT_CHAT_IDS=oc_xxxxxxxxxx
BITABLE_BASE_TOKEN=basexxxxxx
BITABLE_TABLE_DECISION=tblxxxxxxx
MCP_PORT=37777
GIT_REMOTE=git@github.com:your-org/mem-repo.git
GIT_AUTO_PUSH=false
LOG_LEVEL=info
```

## 快速开始

### 前置条件

- Go 1.25+
- Docker (可选)
- 飞书应用凭证 (App ID / App Secret)

### 本地运行

#### 方式一：单体模式（不推荐）

```bash
# 编译
go build -o bin/mem-service ./cmd/mem-service/main.go

# 运行
./bin/mem-service
```

#### 方式二：进程分离模式（推荐）

```bash
# 1. 编译所有二进制
go build -o bin/mem-service ./cmd/mem-service/main.go
go build -o bin/detector-lark-im ./cmd/detector-lark-im/main.go
go build -o bin/detector-lark-doc ./cmd/detector-lark-doc/main.go
go build -o bin/detector-lark-wiki ./cmd/detector-lark-wiki/main.go
go build -o bin/detector-lark-calendar ./cmd/detector-lark-calendar/main.go
go build -o bin/detector-lark-task ./cmd/detector-lark-task/main.go
go build -o bin/detector-lark-vc ./cmd/detector-lark-vc/main.go
go build -o bin/detector-lark-contact ./cmd/detector-lark-contact/main.go

# 2. 启动主服务
./bin/mem-service &
sleep 2

# 3. 启动所有检测器
./bin/detector-lark-im &
./bin/detector-lark-doc &
./bin/detector-lark-wiki &
./bin/detector-lark-calendar &
./bin/detector-lark-task &
./bin/detector-lark-vc &
./bin/detector-lark-contact &

# 4. 查看进程
ps aux | grep -E "mem-service|detector-"

# 5. 查看日志
tail -f logs/app.log
```

### Docker 部署

```bash
# 构建镜像
docker build -t feishu-agent-mem .

# 运行容器
docker run -d \
  --name feishu-agent-mem \
  -p 37777:37777 \
  -v $(PWD)/data:/opt/feishu-agent-mem/data \
  -v $(PWD)/config:/opt/feishu-agent-mem/config \
  --env-file .env \
  feishu-agent-mem
```

### 开发环境重启（Docker 容器内）

```bash
# 进入 docker
docker exec -it openclaw-zh bash
cd /root/openclaw-workspace/feishu-agent-mem

# 停止旧进程
pkill -f "mem-service|detector-"
sleep 1

# 重置状态文件
cat > outputs/detect_state.json <<EOF
{
  "lark_im": {
    "last_check": "2026-05-01T17:00:00Z",
    "last_detected": "2026-05-01T17:00:00Z",
    "version": 150
  }
}
EOF

# 重新编译并启动
go build -o bin/mem-service ./cmd/mem-service/main.go
./bin/mem-service &

# 查看日志
tail -f logs/app.log
```

## 主要依赖

| 依赖 | 用途 |
|-----|------|
| gorilla/websocket | WebSocket 通信 |
| modelcontextprotocol/go-sdk | MCP 协议 |
| sashabaranov/go-openai | OpenAI 客户端 |
| volcengine/go-sdk | 火山引擎 SDK |
| joho/godotenv | 环境变量加载 |

## 开发说明

- 开发语言：Golang
- 编译命令：在本地执行 `go build`
- 重置状态：清除 `mem-service` 进程后删除 `outputs` 文件夹
- 确保 `openclaw.yaml` 中的所有变量都被正确加载

更多开发指南请参考 [CLAUDE.md](./CLAUDE.md)

## License

MIT
