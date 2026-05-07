# Git 决策树设计

## 1. 概述

将 Git 分支模型映射为决策树，每个决策对应一个独立 Git 分支，通过分支操作（创建/切换/提交）实现决策生命周期管理。

### 核心原则

- **每决策 = 每分支**：每个独立决策拥有自己的 Git 分支 `decision/{sdr_id}`
- **版本 = 分支 commits 数**：`Version` 就是该分支上的提交次数
- **冲突 = 分支间关系**：冲突决策通过分支间交叉引用管理
- **回退 = 在被回退的分支上提交新版本**

---

## 2. Git 分支模型

### 2.1 分支规范

| 分支 | 用途 | 规则 |
|------|------|------|
| `main` | 仅存放 dummy 决策 | 永久不变，version=0 |
| `decision/{sdr_id}` | 单个决策的所有版本 | 每个决策独享 |

### 2.2 目录结构

```
data/
  decisions/
    feishu-mem/
      general/
        DEC-001.md    ← 位于分支 decision/DEC-001
        DEC-002.md    ← 位于分支 decision/DEC-002
  objections/
    feishu-mem/
      general/
        OBJ-xxx.md    ← 反对意见在各自的 decision/ 分支上
  L0_RULES.md          ← 首个提交（main）
  DEC-000.md           ← Dummy 决策（main）
```

### 2.3 Dummy 决策

位于 `main` 分支，所有真实决策的根：

```yaml
sdr_id: "DEC-000"
title: "Root Dummy"
version: 0
branch: "main"
status: "completed"       # 不参与活跃查询
```

- 创建于 `L0_RULES.md` 之后的第一次提交
- `version` 永远为 0
- 所有真实决策从 `main` 创建分支，互不干扰

---

## 3. DecisionNode 新增字段

```go
// === Git 分支管理 ===
Branch         string `json:"branch" yaml:"branch"`                     // "decision/DEC-001"
Version        int    `json:"version" yaml:"version"`                   // 版本号（分支 commits 数）

// === 冲突状态 ===
ConflictStatus string `json:"conflict_status,omitempty" yaml:"conflict_status,omitempty"` // "" | "active" | "resolved"
ConflictWith    string `json:"conflict_with,omitempty" yaml:"conflict_with,omitempty"`     // 冲突对端的 SDRID
```

**复用现有字段**：
- `Relations[].Type = CONFLICTS_WITH` — 已有的关系类型标识冲突对端
- `Status = superseded` — 冲突解决后败方最终状态
- `PreviousCommitHash` — 版本链回溯

**新常量**：

```go
const (
    ConflictActive           = "active"
    ConflictResolved         = "resolved"
    BranchPrefixDecision     = "decision/"
    DummySDRID               = "DEC-000"
)
```

---

## 4. 各操作 Git 流程

### 4.1 创建全新决策

**触发**：信号引擎提取到新决策 → `MutationCreate`

```
1. git checkout main
2. git checkout -b decision/DEC-001       # 从 main 创建决策分支
3. 写入 decisions/.../DEC-001.md
4. git add + git commit -m "create(DEC-001): 采用 Spring Boot 3"
5. version = 1, branch = "decision/DEC-001"
```

决策文件内容：

```yaml
sdr_id: "DEC-001"
branch: "decision/DEC-001"
version: 1
conflict_status: ""
conflict_with: ""
```

### 4.2 决策更新

**触发**：`MutationUpdate`（MCP `update_decision`）

```
1. git checkout decision/DEC-001          # 切到该决策分支
2. 修改 decisions/.../DEC-001.md
3. git add + git commit -m "update(DEC-001): title=新标题"
4. version = 2
```

### 4.3 决策冲突检测

**场景**：决策 A（先提交，当前 version 最新，无冲突）与决策 B（后提交）语义冲突。

**创建决策 B 时**：

```
1. git checkout main
2. git checkout -b decision/DEC-B         # 从 main 创建 B 的分支
3. 写入 B 的决策文件
4. git add + git commit -m "create(DEC-B): ..."
5. version_B = 1
```

**检测到冲突后，标记双方**：

```
6. git checkout decision/DEC-A
7. A 文件加字段: conflict_status=active, conflict_with=DEC-B
8. A.Relations += [{type: CONFLICTS_WITH, target: DEC-B}]
9. git add + git commit -m "conflict(DEC-A): vs DEC-B"
10. version_A = 2

11. git checkout decision/DEC-B
12. B 文件加字段: conflict_status=active, conflict_with=DEC-A
13. B.Relations += [{type: CONFLICTS_WITH, target: DEC-A}]
14. git add + git commit -m "conflict(DEC-B): vs DEC-A"
15. version_B = 2
```

**最终状态**：

```yaml
# DEC-A（先提交）
version: 2
conflict_status: "active"
conflict_with: "DEC-B"
relations:
  - type: CONFLICTS_WITH
    target_sdr_id: DEC-B

# DEC-B（后提交）
version: 2
conflict_status: "active"
conflict_with: "DEC-A"
relations:
  - type: CONFLICTS_WITH
    target_sdr_id: DEC-A
```

### 4.4 冲突解决

#### 场景 4a：选定决策 A（胜），决策 B（败）

```
1. git checkout decision/DEC-A
2. A.conflict_status=resolved, A.conflict_with=""
3. git add + git commit -m "resolve(DEC-A): won vs DEC-B"
4. version_A = 3

5. git checkout decision/DEC-B
6. B.status=superseded, B.conflict_status=resolved, B.conflict_with=""
7. B.Relations += [{type: SUPERSEDES, target: DEC-A}]
8. git add + git commit -m "resolve(DEC-B): superseded by DEC-A"
9. version_B = 3
```

#### 场景 4b：新决策 C 替代 A 和 B

```
1. git checkout main
2. git checkout -b decision/DEC-C
3. 写入合并后的决策内容
4. git add + git commit -m "create(DEC-C): merged from A,B"
5. C.version = max(A.version, B.version) + 1 = 4
6. C.Relations += [{SUPERSEDES, A}, {SUPERSEDES, B}]
7. git commit --amend（或额外 commit）

8. git checkout decision/DEC-A
9. A.status=superseded, A.conflict_status=resolved
10. git commit -m "resolve(DEC-A): superseded by DEC-C"

11. git checkout decision/DEC-B
12. B.status=superseded, B.conflict_status=resolved
13. git commit -m "resolve(DEC-B): superseded by DEC-C"
```

#### 场景 4c：Git merge 合并

利用 Git 原生的 merge commit 记录冲突解决历史：

```
1. git checkout decision/DEC-A
2. git merge --no-ff decision/DEC-B -m "merge(DEC-A): resolve conflict with DEC-B"
3. 解决文件冲突（手动选择合并后内容）
4. git add + git commit
5. version_A 进入合并后新版本
```

`git log --graph` 可以看到两个分支的交叉历史。

### 4.5 决策回退（Revert）

**场景**：当前使用决策 B（version=5），决定回退到历史决策 A（version=3）。

```
1. git checkout decision/DEC-A            # 切回 A 的分支
2. 读取 A 当前内容（无需改动决策正文）
3. A.version = B.version + 1 = 6          # 版本跳到最新
4. A.status = decided
5. A.Relations += [{SUPERSEDES, target: DEC-B}]
6. git add + git commit -m "revert(DEC-A): restored, supersedes DEC-B(v5)"
7. version_A = 6

8. git checkout decision/DEC-B
9. B.status = superseded
10. git commit -m "revert(DEC-B): superseded by DEC-A(v6)"
11. version_B = 6
```

**版本跳跃**：`A.version` 从 3 跳到 6（= B.version + 1），表示 A 是当前最新有效决策。

### 4.6 决策废弃

```
1. git checkout decision/DEC-A
2. A.status = deprecated
3. git add + git commit -m "deprecate(DEC-A): 方案不再适用"
4. version_A++
```

---

## 5. 查询与读取

### 5.1 MemoryGraph 加载

启动时遍历所有 `decision/` 分支，读取每个分支的最新 HEAD 内容：

```
git branch -a | grep "decision/"  → 列出所有决策分支
  → git show decision/DEC-001:decisions/feishu-mem/general/DEC-001.md
  → 解析 → 写入 MemoryGraph
```

`main` 上的 Dummy 决策也加载，但 `isActive()` 过滤掉（status=completed）。

### 5.2 MCP 查询

所有 MCP 查询基于 MemoryGraph，不受分支影响：
- 按热点值查询：`isActive()` 过滤活跃决策，按 `HotScore` 排序
- 按议题查询：`QueryByTopic()` → 活跃决策
- 关键词搜索：`SearchByKeywords()` → 活跃决策

### 5.3 Git 原生查询

```bash
# 查看决策 A 的所有版本
git log decision/DEC-A -- decisions/feishu-mem/general/DEC-A.md

# 查看全部决策拓扑
git log --graph --oneline --all

# 列出所有决策分支
git branch -a | grep "decision/"

# 查看某个分支的最新内容
git show decision/DEC-A:decisions/feishu-mem/general/DEC-A.md
```

---

## 6. 修改清单

### 6.1 internal/decision/node.go

- 新增 `Branch string`、`Version int`、`ConflictStatus string`、`ConflictWith string` 字段
- 新增 `ConflictActive`、`ConflictResolved` 常量
- 新增 `BranchPrefixDecision = "decision/"`、`DummySDRID = "DEC-000"` 常量
- 新增方法：
  - `GetDecisionBranch() string` — 返回 `BranchPrefixDecision + sdr_id`
  - `BumpVersion()` — `Version++`
  - `SetConflict(otherSDRID string)` — 设置冲突状态 + CONFLICTS_WITH 关系
  - `ResolveConflict()` — 清除冲突状态
- `NewDecisionNode()` 初始化 `version=1`、`branch=BranchPrefixDecision + sdrID`

### 6.2 internal/storage/git/git_storage.go

- `initRepo()` 增加：创建 Dummy 决策文件 `DEC-000.md` 并提交
- `WriteDecision()` 改为：
  1. 保存当前分支名
  2. `EnsureDecisionBranch(node.Branch)` — 如果分支不存在则从 main 创建
  3. `git checkout node.Branch`
  4. 写入文件 + commit
  5. `git checkout` 回之前的分支
- 新增 `EnsureDecisionBranch(branchName string) error` — 从 main 创建分支
- 新增 `ListDecisionBranches() ([]string, error)` — 列出 `decision/` 前缀分支
- `ListDecisions()` 改为遍历 `decision/` 分支（而非文件系统）
- 移除 `CreateBranch` / `SwitchBranch` / `MergeBranch` 通用方法（改为内部使用）

### 6.3 internal/core/pipeline.go

- `applyCreate()`：调用 `WriteDecision`（自动写入决策分支）
- `applyUpdate()`：切换决策分支 → 修改 → 提交 → version++
- `applyConflictKeepBoth()`：先用 `applyCreate` 写 B → 然后标记双方冲突
- 新增 `markConflict(sdrID, otherSDRID string)`：切分支 → 标记冲突 → 提交
- 新增 `resolveConflictWinner(winnerID, loserID string)`：切胜者分支 → resolved → 切败者分支 → superseded
- `applyRevert()`：切回决策分支 → version 跳跃 → 提交

### 6.4 internal/core/memory_graph.go

- `LoadFromGit()` 改为：遍历 `decision/` 分支读取 HEAD

### 6.5 internal/mcp/server/server.go

- `handleCreateDecision` / `handleUpdateDecision` / `handleResolveConflict` / `handleRevertDecision`：调用新 pipeline 方法
- 新增 `handleDecisionTree` — 返回 `git log --graph --oneline --all` 文本

### 6.6 初始化流程（cmd/mem-service/main.go）

启动时确保 `main` 分支存在 Dummy 决策：
1. 初始化 Git 仓库
2. 如果 `DEC-000.md` 不存在：创建并提交
3. `MemoryGraph.LoadFromGit()` 遍历所有 `decision/` 分支

### 6.7 文件格式变更

**旧格式（移除）**：
```yaml
version_range:
  from: "1"
  to: "2"
```

**新格式**：
```yaml
sdr_id: "DEC-001"
version: 3
branch: "decision/DEC-001"
conflict_status: "resolved"
conflict_with: "DEC-002"
relations:
  - type: CONFLICTS_WITH
    target_sdr_id: DEC-002
```

---

## 7. Git DAG 完整示例

一个完整的生命周期：创建 A → 创建 B → 冲突 → A 胜出 → 回退到 B：

```
* (decision/DEC-B) revert: restored, supersedes DEC-A(v4)     [v5]
| * (decision/DEC-A) resolve: won vs DEC-B                    [v4]
| * (decision/DEC-A) conflict: vs DEC-B                        [v3]
| * (decision/DEC-A) update: 标题修改                          [v2]
| * (decision/DEC-A) create: 采用 Spring Boot 3                [v1]
| | * (decision/DEC-B) resolve: superseded by DEC-A            [v3]
| | * (decision/DEC-B) conflict: vs DEC-A                      [v2]
| | * (decision/DEC-B) create: 采用 Quarkus                    [v1]
| |/
|/|
* | (main) create: DEC-000 Root Dummy                          [v0]
|/
* L0_RULES.md
```

---

## 8. 验证方案

### 8.1 启动验证

```bash
# 检查 Dummy 决策
git log --oneline main     # 应有 L0_RULES.md + DEC-000 两个提交
cat data/DEC-000.md        # 展示 Dummy 决策内容

# 检查所有分支
git branch -a              # 仅 main，无 decision/ 分支
```

### 8.2 创建决策验证

```bash
# 发送消息 → 自动创建决策
curl ... "采用 Spring Boot 3"

# 验证
git branch -a | grep "decision/"    # 应有新分支
git log decision/DEC-001 --oneline   # 应有 1 个 commit
git show decision/DEC-001:data/decisions/feishu-mem/general/DEC-001.md | grep "version: 1"
```

### 8.3 更新验证

```bash
openclaw mcp call update_decision --sdr_id "DEC-001" --field title --value "新标题"
git log decision/DEC-001 --oneline   # 应有 2 个 commit
git show decision/DEC-001:.../DEC-001.md | grep "version: 2"
```

### 8.4 冲突验证

```bash
# 创建冲突决策 B
curl ... "采用 Quarkus"

# 验证两个分支冲突标记
git show decision/DEC-001:.../DEC-001.md | grep conflict_status
git show decision/DEC-002:.../DEC-002.md | grep conflict_status
```

### 8.5 冲突解决验证

```bash
openclaw mcp call resolve_conflict --winner "DEC-001" --loser "DEC-002"

# 验证胜者
git show decision/DEC-001:.../DEC-001.md | grep "conflict_status: resolved"

# 验证败者
git show decision/DEC-002:.../DEC-002.md | grep "status: superseded"

# 查看拓扑
git log --graph --oneline --all
```

### 8.6 回退验证

```bash
openclaw mcp call revert_decision --sdr_id "DEC-001" --target_version 1

# 验证版本跳跃
git show decision/DEC-001:.../DEC-001.md | grep "version: 4"
```

### 8.7 全量验证脚本

```bash
#!/bin/bash
echo "=== 决策树 Git 状态 ==="
echo ""
echo "--- 全部分支 ---"
git branch -a
echo ""
echo "--- 冲突拓扑 ---"
git log --graph --oneline --all
echo ""
echo "--- 各决策最新版本 ---"
for branch in $(git branch | grep "decision/"); do
  file=$(git show $branch: | head -1 | grep "decisions/")
  if [ -n "$file" ]; then
    echo "$branch: $(git show $branch:$file | grep 'version:' | head -1)"
  fi
done
```
