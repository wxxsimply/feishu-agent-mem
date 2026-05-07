# Lark-IM 演示计划

## 环境准备

1. 确保 `config/openclaw.yaml` 中 `lark_im.enabled: true`
2. `.env` 中 `LARK_DETECT_CHAT_IDS=oc_adbd88237ce258352e10e2baf580efad`
3. 清除状态：`rm -f outputs/detect_state.json && rm -rf data/decisions data/objections`
4. 编译：`go build -o bin/mem-service ./cmd/mem-service/main.go`
5. 启动：`./bin/mem-service &`
6. 观察日志：`tail -f logs/app.log`

## 演示流程

### 场景 1：决策创建与提取

**目的**：展示群聊中提出方案后被自动提取为决策

**发送者**：Zack（`fd9dc672-40ae-4bdb-8316-c7923a7c9f22`）

| 步骤 | 消息内容 | 预期结果 |
|------|---------|---------|
| 1 | "技术选型决定采用 Spring Boot 3 作为微服务基础框架，搭配 JDK 17" | 检测到高置信度决策，LLM 提取"采用 Spring Boot 3 作为微服务框架" |
| 2 | 等待 15-20 秒 | 决策被持久化到 Git，推送卡片到群聊 |

**验证点**：
- 日志中出现 `level=high` 评分
- LLM 提取 `HasDecision=true`
- 新决策 SDRID 被创建
- Git 中有对应 `decisions/feishu-mem/general/DEC-xxx.md` 文件

---

### 场景 2：重复决策去重

**目的**：展示相同语义的决策被自动去重

**发送者**：Zack

| 步骤 | 消息内容 | 预期结果 |
|------|---------|---------|
| 3 | "确定使用 Spring Boot 3 作为后端框架，技术栈选用 Java 17" | 检测到，LLM 提取，`findSimilarDecision` 匹配到步骤 1 |
| 4 | 等待 20 秒 | `evaluateDedupAction` 返回 `skip`，无新决策创建 |

**验证点**：
- 日志显示 `Found overlapping decision`（tokenMatch）
- `Minor change, skipping`
- 无新 `Type=create` 出现

---

### 场景 3：反对意见检测

**目的**：展示对已有决策的反对意见被提取

**发送者**：Floy（`c857523a-a9a9-40ed-bfef-5efc70f0242d`）

| 步骤 | 消息内容 | 预期结果 |
|------|---------|---------|
| 5 | "我不同意用 Spring Boot，Quarkus 的启动速度更快，云原生支持更好" | 检测到反对意见信号 |
| 6 | 等待 20 秒 | LLM 提取反对意见，创建 `OBJ-xxx` 持久化 |

**验证点**：
- 日志出现 `Objections extracted: 1`
- `Created objection mutation`
- 文件写入 `data/objections/feishu-mem/general/OBJ-xxx.md`

---

### 场景 4：冲突决策检测

**目的**：展示冲突决策被保留并推送解决卡片

**发送者**：Zack

| 步骤 | 消息内容 | 预期结果 |
|------|---------|---------|
| 7 | "数据库选型决定采用 PostgreSQL 15，支持复杂查询和 MVCC" | 高评分，LLM 提取 |
| 8 | 等待 20 秒 | 新决策创建："采用 PostgreSQL 15" |
| 9 | "数据库选型改为使用 MySQL 8.0，运维团队更熟悉" | 检测到，LLM 提取 |
| 10 | 等待 20 秒 | `findSimilarDecision` 匹配 → `evaluateDedupAction` 返回 `conflict` |
| 11 | 等待冲突处理 | pipeline 执行 `keep_both`，推送冲突卡片到群聊 |

**验证点**：
- `DedupConflict` → `resolveConflict` → `keep_both`
- `CONFLICTS_WITH` 关系建立
- `Sent conflict resolution card`

---

### 场景 5：热点值更新

**目的**：展示被讨论的决策热点值增长

**执行**：在步骤 1-3 后，通过 MCP 工具查询节点热点值

```bash
# 使用 mcp-test 或 OpenClaw agent 查询
# 每次讨论引用后，HotScore 增加约 8 点
```

---

## 发送脚本

```bash
# Zack - 方案提出
curl -s -X POST "https://open.feishu.cn/open-apis/bot/v2/hook/fd9dc672-40ae-4bdb-8316-c7923a7c9f22" \
  -H "Content-Type: application/json" \
  -d '{"msg_type":"text","content":{"text":"技术选型决定采用 Spring Boot 3 作为微服务基础框架，搭配 JDK 17"}}'

# 等待 20 秒

# Zack - 重复决策
curl -s -X POST "https://open.feishu.cn/open-apis/bot/v2/hook/fd9dc672-40ae-4bdb-8316-c7923a7c9f22" \
  -H "Content-Type: application/json" \
  -d '{"msg_type":"text","content":{"text":"确定使用 Spring Boot 3 作为后端框架，技术栈选用 Java 17"}}'

# 等待 20 秒

# Floy - 反对意见
curl -s -X POST "https://open.feishu.cn/open-apis/bot/v2/hook/c857523a-a9a9-40ed-bfef-5efc70f0242d" \
  -H "Content-Type: application/json" \
  -d '{"msg_type":"text","content":{"text":"我不同意用 Spring Boot，Quarkus 的启动速度更快，云原生支持更好"}}'

# 等待 25 秒

# Zack - 数据库选型
curl -s -X POST "https://open.feishu.cn/open-apis/bot/v2/hook/fd9dc672-40ae-4bdb-8316-c7923a7c9f22" \
  -H "Content-Type: application/json" \
  -d '{"msg_type":"text","content":{"text":"数据库选型决定采用 PostgreSQL 15，支持复杂查询和 MVCC"}}'

# 等待 20 秒

# Zack - 冲突决策
curl -s -X POST "https://open.feishu.cn/open-apis/bot/v2/hook/fd9dc672-40ae-4bdb-8316-c7923a7c9f22" \
  -H "Content-Type: application/json" \
  -d '{"msg_type":"text","content":{"text":"数据库选型改为使用 MySQL 8.0，运维团队更熟悉"}}'

# 等待 25 秒，观察冲突卡片推送
```

## 录屏要点

1. 打开日志窗口 (`tail -f logs/app.log`) 展示实时检测过程
2. 每个消息发送后等待检测日志出现再发下一条
3. 重点截图/录像：
   - 日志中 `level=high` + LLM 提取过程
   - `Found overlapping decision` 去重
   - `Objections extracted` 反对意见
   - `DedupConflict` + `Sent conflict resolution card`
4. 展示 Git 中生成的决策文件
5. 展示飞书群聊中收到的决策卡片和冲突卡片



技术选型决定采用 Spring Boot 3 作为微服务基础框架，搭配 JDK 17

确定使用 Spring Boot 3 作为后端框架，技术栈选用 Java 17

数据库选型决定采用 PostgreSQL 15，支持复杂查询和 MVCC

数据库选型改为使用 MySQL 8.0，运维团队更熟悉

