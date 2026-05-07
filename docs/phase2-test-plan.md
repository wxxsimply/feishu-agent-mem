# Phase 2 测试计划

## 前置条件

### 环境准备
1. 进入 docker：`docker exec -it openclaw-zh bash`
2. 停止所有旧进程：`pkill -f "mem-service|detector-" 2>/dev/null`
3. 设置 LARK_DETECT_CHAT_IDS（lark-im 检测用）和 LARK_CHAT_IDS（推送用）
4. 缩短检测间隔：在 `config/openclaw.yaml` 中设置 `lark_im.interval: 5s`, `lark_doc.interval: 10s`
5. 编译：`go build -o bin/mem-service ./cmd/mem-service/main.go`

### 状态重置
```bash
cat > outputs/detect_state.json <<EOF
{
  "lark_im": {
    "last_check": "2026-05-01T17:00:00Z",
    "last_detected": "2026-05-01T17:00:00Z",
    "version": 150
  },
  "lark_doc": {
    "last_check": "2026-05-01T17:00:00+08:00",
    "last_detected": "2026-05-01T17:00:00+08:00",
    "version": 150
  }
}
EOF
```

### 启动服务
```bash
./bin/mem-service &
tail -f logs/app.log
```

---

## Test 1: 反对意见测试

### lark-im 方案

**目标**：验证 IM 消息中的反对意见被正确检测和持久化。

**测试数据**（发送到检测群聊 `oc_f382174f2ab17aa2ebefba72834df0b2`）：

**消息 1**（创建决策）：
> "我建议后端使用 Gin 框架，性能好、社区活跃。"

**消息 2**（反对意见）：
> "我不同意用 Gin，Gin 虽然快但生态不如 Echo 丰富，而且我们团队对 Echo 更熟悉。"

**执行步骤**：
1. 发送消息 1，等待检测器检测（约 5-10s）
2. 检查日志：确认提取到决策
3. 发送消息 2，等待检测器检测
4. 检查日志中出现 `[SignalEngine] Processing objection` 或类似信息
5. 通过 MCP 工具 `objection_list` 检查反对意见是否被持久化

**预期结果**：
- 决策被创建，内容为"后端使用 Gin 框架"
- 反对意见被提取，内容包含"不同意用 Gin"
- 反对意见的 `SourceType` 为 "im"
- 反对意见通过 `objection_list` 可查询

### lark-doc 方案

**目标**：验证文档评论中的反对意见被正确检测。

**测试数据**：

在飞书文档中操作：
1. 在白名单文档（如 `wiki/NiifwvCvtiPJmrkkegMckNDPnxf`）正文写入： "数据库方案：采用 MySQL 8.0"
2. 在对应文档添加评论： "我建议用 PostgreSQL，MySQL 8.0 的分区表性能问题太多"

**执行步骤**：
1. 在文档中写入内容，等待检测器检测
2. 在文档中添加评论，等待检测器扫描评论
3. 检查日志：确认检测到评论和反对意见
4. 通过 `objection_list` 验证

**预期结果**：
- 决策被创建："数据库采用 MySQL 8.0"
- 反对意见被提取："建议用 PostgreSQL"
- 反对意见的 `SourceType` 为 "comment"

---

## Test 2: 决策热点更新测试

### 测试步骤（lark-im 和 lark-doc 共用）

**准备**：
1. 确保存在至少一个决策（或通过 `create_decision` MCP 工具创建）

**执行**：
1. 查询决策初始状态：
   ```
   decision <sdr_id>
   ```
   记录 `AccessCount` 和 `HotScore`

2. 通过 MCP 工具 `search` 搜索该决策关键词：
   ```
   search --query "<关键词>"
   ```

3. 再次查询决策：
   ```
   decision <sdr_id>
   ```
   检查 `AccessCount` 是否 +1

4. 每查询一次，`AccessCount` 应递增，重新计算 `HotScore` 后值应变化

**验证**：
- `AccessCount` 在每次查询后 +1
- 查询后 `HotScore` 重新计算（通过 `decision_card` 工具查看）

---

## Test 3: 冲突决策测试

### lark-im 方案

**目标**：验证 IM 消息中的冲突决策被正确检测。

**测试数据**：

**消息 1**（创建决策）：
> "数据库选型：确定使用 MySQL 8.0，主从复制架构"

**消息 2**（冲突决策）：
> "数据库选型改为使用 PostgreSQL 15，支持更复杂的查询"

**执行步骤**：
1. 发送消息 1，等待检测
2. 确认决策被创建（记录 SDRID）
3. 发送消息 2，等待检测
4. 检查日志中冲突检测相关输出

**预期结果**：
- 检测到相似决策，`findSimilarDecision` 返回匹配
- `evaluateDedupAction` 返回 `DedupConflict`（因为 "MySQL" vs "PostgreSQL" 为冲突选择）
- `resolveConflict` 被调用，LLM 决定 `keep_both`
- 两个决策均被持久化，通过 `conflict_list <sdr_id_1>` 可查询到 `CONFLICTS_WITH` 关系

**冲突解决验证**：
1. 通过 `conflict_list` 查看所有冲突
2. 使用 `resolve_conflict` 工具选择保留哪个：
   ```
   resolve_conflict --winner_sdr_id <sdr_id_1> --loser_sdr_id <sdr_id_2> --reason "MySQL更适合当前场景"
   ```
3. 验证 loser 状态变为 `superseded`，冲突关系被清除

### lark-doc 方案

**目标**：验证文档内容变更触发的冲突检测。

**测试数据**：

1. 在文档中写入："缓存方案：采用 Redis 6.x，集群模式"
2. 修改文档内容为："缓存方案改为采用 Memcached，一致性哈希分布"

**执行步骤**：
1. 写入原始内容，等待检测
2. 修改为冲突内容，等待检测
3. 检查日志确认冲突检测

**预期结果**：
- 第一次写入创建决策："缓存采用 Redis 6.x 集群模式"
- 第二次写入检测到内容变更，LLM 评估为冲突
- 两个决策均保留，带有 `CONFLICTS_WITH` 关系

---

## Test 4: Git 搜索测试

### 测试步骤（lark-im 和 lark-doc 共用）

**准备**：
1. 通过 `create_decision` 或检测器创建至少 2-3 个决策
2. 对其中一个决策使用 `update_decision` 进行更新

**执行**：

**4a. Git 历史查询**
```
git_history --path "decisions/feishu-mem/general"
```
预期：显示所有 Git 提交记录

**4b. Git 内容搜索**
```
git_search --query "MySQL" --project "feishu-mem"
```
预期：返回包含 "MySQL" 的文件和行号

**4c. 决策历史**
```
decision_history --sdr_id <sdr_id>
```
预期：显示该决策的所有 Git 提交版本

**4d. Git Blame（如果决策有多个版本）**
```
git_blame --sdr_id <sdr_id>
```
预期：显示每行内容的最后修改 commit 和作者

**4e. 决策回溯（revert）**
1. 记录当前决策内容
2. 使用 `update_decision` 更新决策内容
3. 使用 `decision_history` 获取历史 commit hash
4. 使用 `revert_decision` 回退到原始版本：
   ```
   revert_decision --sdr_id <sdr_id> --target_commit <hash> --reason "测试回溯"
   ```
5. 验证决策内容恢复为原始版本
6. `decision_history` 中应显示包括 revert 在内的所有历史记录

---

## 新增 MCP 工具验证

### confirm_decision / reject_decision
1. 创建低置信度决策（通过 `extract_and_create` 或检测器）
2. 确认状态为 `pending_confirmation`
3. 使用 `confirm_decision --sdr_id <id>` 确认
4. 确认状态变为 `decided`
5. 使用 `reject_decision --sdr_id <id>` 拒绝另一个
6. 确认状态变为 `rejected`

### extract_and_create
1. 调用 `extract_and_create --content "确定使用 Go 1.22 作为开发语言"`
2. 确认决策被持久化到 Git
3. 再次调用相似内容，确认去重检测提示

### resolve_conflict
1. 通过冲突测试先创建冲突决策对
2. 使用 `conflict_list` 查看冲突
3. 使用 `resolve_conflict` 解决
4. 确认败者状态变更为 `superseded`

---

## 测试记录表

| 测试编号 | 场景 | lark-im | lark-doc | 结果 | 备注 |
|---------|------|---------|----------|------|------|
| T1 | 反对意见检测 | □ | □ | □ | |
| T2 | 热点值更新 | □ | □ | □ | |
| T3 | 冲突检测与解决 | □ | □ | □ | |
| T4a | Git 历史查询 | □ | □ | □ | |
| T4b | Git 内容搜索 | □ | □ | □ | |
| T4c | 决策历史 | □ | □ | □ | |
| T4d | Git Blame | □ | □ | □ | |
| T4e | 决策回溯 | □ | □ | □ | |
| T5a | confirm_decision | - | - | □ | 通用工具 |
| T5b | reject_decision | - | - | □ | 通用工具 |
| T5c | extract_and_create | - | - | □ | 通用工具 |
| T5d | resolve_conflict | - | - | □ | 通用工具 |
