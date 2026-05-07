# Lark-Doc 演示计划

## 环境准备

1. 确保 `config/openclaw.yaml` 中 `lark_doc.enabled: true`
2. `doc_tokens` 包含 `UxJZwA218iXVPjkL0nbcXFDWnSd`
3. 清除状态：`rm -f outputs/detect_state.json && rm -rf data/decisions data/objections`
4. 编译：`go build -o bin/mem-service ./cmd/mem-service/main.go`
5. 启动：`./bin/mem-service &`
6. 观察日志：`tail -f logs/app.log`

## 文档说明

**文档 Token**: `UxJZwA218iXVPjkL0nbcXFDWnSd`

在飞书文档中依次写入/追加以下内容。使用 lark-cli 追加文本到文档：

```bash
# 追加内容到文档
lark-cli docs +append --doc UxJZwA218iXVPjkL0nbcXFDWnSd --content "要追加的内容"
```

每次修改后等待 lark_doc 检测周期（约 10-15 秒），观察日志输出。

---

## 演示流程

### 阶段 1：项目文档初始化（非决策内容）

**目的**：展示文档初始状态，验证噪声过滤

| 步骤 | 追加内容 | 预期结果 |
|------|---------|---------|
| 1 | `# 电商平台技术方案 v2.0\n\n## 项目背景\n\n基于微服务架构重构现有单体电商平台，提升系统可扩展性和团队开发效率。` | 检测到文档变更，但 LLM 判断为非决策内容 |
| 2 | 等待 15 秒 | `HasDecision=false`，跳过 |

---

### 阶段 2：技术选型决策

**目的**：展示文档修改中提取决策

| 步骤 | 追加内容 | 预期结果 |
|------|---------|---------|
| 3 | `## 技术栈选型\n\n### 后端框架\n\nSpring Boot 3.x + JDK 17` | 检测到变更，LLM 提取决策 |
| 4 | 等待 15 秒 | `HasDecision=true`，提取"采用 Spring Boot 3.x + JDK 17 作为后端框架" |
| 5 | `### 数据库\n\nMySQL 8.0，采用读写分离架构` | 检测到变更，提取决策 |
| 6 | 等待 15 秒 | 提取"数据库采用 MySQL 8.0 读写分离架构" |
| 7 | `### 缓存\n\nRedis 7.x 集群，缓存热点数据` | 检测到变更，提取决策 |
| 8 | 等待 15 秒 | 提取"缓存方案采用 Redis 7.x 集群" |

**验证点**：
- 每次追加后日志出现 LLM 提取过程
- 新决策被创建并持久化到 Git
- Bitable 同步成功

---

### 阶段 3：架构设计决策

| 步骤 | 追加内容 | 预期结果 |
|------|---------|---------|
| 9 | `## 系统架构\n\n### 服务划分\n\n拆分为 6 个核心微服务：用户服务、商品服务、订单服务、支付服务、库存服务、消息服务` | 检测到变更，LLM 提取"拆分为 6 个核心微服务" |
| 10 | 等待 15 秒 | 新决策创建 |
| 11 | `### 通信方式\n\n服务间采用 gRPC 通信，异步消息使用 RocketMQ` | 检测到变更，提取决策 |
| 12 | 等待 15 秒 | 提取"服务间通信采用 gRPC，异步消息使用 RocketMQ" |

---

### 阶段 4：决策变更与冲突

**目的**：展示文档修改导致的决策冲突

| 步骤 | 追加内容 | 预期结果 |
|------|---------|---------|
| 13 | `### 数据库变更\n\n由于分片需求，从 MySQL 切换为 TiDB，兼容 MySQL 协议且支持分布式事务` | 检测到变更，LLM 提取"数据库从 MySQL 切换为 TiDB" |
| 14 | 等待 20 秒 | `findSimilarDecision` 匹配阶段 2 的 MySQL 决策 |
| 15 | 等待冲突处理 | `evaluateDedupAction` 返回 `conflict`，`resolveConflict` → `keep_both` |
| 16 | 观察 | 两个决策都保留，带 `CONFLICTS_WITH` 关系，推送冲突卡片 |

---

### 阶段 5：决策回溯（Revert）

**目的**：还原文档中的决策变更

| 步骤 | 操作 | 预期结果 |
|------|------|---------|
| 17 | 在文档中添加：`### 数据库方案确认\n\n最终决定采用 TiDB 作为数据库方案` | 提取决策"确定采用 TiDB 数据库方案" |
| 18 | 等待 15 秒 | 新决策创建 |
| 19 | 追加：`### 回滚说明\n\n⚠️ 由于 TiDB 运维成本过高，回退到 MySQL 8.0 读写分离架构，后续再评估分布式方案` | LLM 可能检测到回滚（RevertDetector） |

---

### 阶段 6：决策确认状态

**目的**：展示低置信度决策自动标记待确认

| 步骤 | 追加内容 | 预期结果 |
|------|---------|---------|
| 20 | `### 监控方案\n\n可以考虑 Prometheus + Grafana，或者 SkyWalking` | 置信度较低（不确定措辞），标记为 `pending_confirmation` |
| 21 | 等待 15 秒 | 状态为 `pending_confirmation`，可通过 `confirm_decision` 确认 |

---

电商平台技术方案 v2.0
项目背景
基于微服务架构重构现有单体电商平台，提升系统可扩展性和团队开发效率。

技术栈选型
后端框架：SprintBoot 3.x + JDK 17
数据库：MySQL 8.0，采用读写分离架构
缓存：Redis 7.x集群，缓存热点数据

系统架构
服务划分为6个核心微服务：
- 用户服务
- 商品服务
- 订单服务
- 支付服务
- 库存服务
- 消息服务

## 完整发送脚本

```bash
#!/bin/bash
DOC="UxJZwA218iXVPjkL0nbcXFDWnSd"
BASE="lark-cli docs +append --doc $DOC --content"

# 阶段 1：初始化
$BASE "# 电商平台技术方案 v2.0

## 项目背景

基于微服务架构重构现有单体电商平台，提升系统可扩展性和团队开发效率。"

sleep 15

# 阶段 2：技术选型
$BASE "## 技术栈选型

### 后端框架

Spring Boot 3.x + JDK 17"

sleep 15

$BASE "### 数据库

MySQL 8.0，采用读写分离架构"

sleep 15

$BASE "### 缓存

Redis 7.x 集群，缓存热点数据"

sleep 15

# 阶段 3：架构设计
$BASE "## 系统架构

### 服务划分

拆分为 6 个核心微服务：用户服务、商品服务、订单服务、支付服务、库存服务、消息服务"

sleep 15

$BASE "### 通信方式

服务间采用 gRPC 通信，异步消息使用 RocketMQ"

sleep 15

# 阶段 4：冲突决策
$BASE "### 数据库变更

由于分片需求，从 MySQL 切换为 TiDB，兼容 MySQL 协议且支持分布式事务"

sleep 25

# 阶段 5：回溯
$BASE "### 数据库方案确认

最终决定采用 TiDB 作为数据库方案"

sleep 15

$BASE "### 回滚说明

⚠️ 由于 TiDB 运维成本过高，回退到 MySQL 8.0 读写分离架构，后续再评估分布式方案"

sleep 15

# 阶段 6：待确认
$BASE "### 监控方案

可以考虑 Prometheus + Grafana，或者 SkyWalking"

echo "全部完成"
```

## 录屏要点

1. **分屏展示**：左侧飞书文档编辑窗口，右侧终端日志
2. **关键时间点**：
   - 追加文档内容后 → 日志显示检测到变更
   - LLM 提取出决策信息
   - 新决策 SDRID 被创建
   - 冲突检测触发 → 冲突卡片推送
3. **展示内容**：
   - 飞书文档在每次追加后内容变化
   - `tail -f logs/app.log` 实时检测过程
   - Git 目录中生成的 `.md` 文件
   - 飞书群聊中收到的决策卡片
4. **时间安排**：全程约 3-4 分钟
